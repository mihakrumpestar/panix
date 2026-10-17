// Derived from charm.land/bubbletea/v2. See pkg/tui/LICENSE.charmbracelet.

package zeroterm

import (
	"os"
	"time"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type Program struct {
	model           Model
	terminal        *Terminal
	outBuf          []byte
	msgCh           chan Msg
	stopCh          chan struct{}
	width           int
	height          int
	raw             bool
	forceFullRender bool
	frames          [2]*buffer.LinesBufDiff
	curFrame        int
	renderCounter   uint64

	widthProbeGlyphs  []string
	widthProbeInstall func(measured map[string]int, elapsed time.Duration)
}

type ProgramOption func(*Program)

// NewProgram creates a new terminal program with the given model and options.
func NewProgram(model Model, opts ...ProgramOption) *Program {
	prog := &Program{
		model:  model,
		msgCh:  make(chan Msg, 1024), //nolint:mnd
		stopCh: make(chan struct{}),
		outBuf: make([]byte, 0, 8192), //nolint:mnd
		frames: [2]*buffer.LinesBufDiff{
			buffer.NewLinesBufDiff(),
			buffer.NewLinesBufDiff(),
		},
	}

	for _, opt := range opts {
		opt(prog)
	}

	return prog
}

// Raw disables frame diffing; every render writes all lines directly.
func Raw() ProgramOption {
	return func(p *Program) { p.raw = true }
}

// WithWidthProbe enables the glyph width probe: before the first frame, Run
// measures each glyph and calls install with the widths and duration. glyphs
// should cover every glyph the UI can draw. install runs at most once, and only
// on success; otherwise the static width table is kept.
func WithWidthProbe(glyphs []string, install func(measured map[string]int, elapsed time.Duration)) ProgramOption {
	return func(p *Program) {
		p.widthProbeGlyphs = glyphs
		p.widthProbeInstall = install
	}
}

// RenderCounter returns the current render counter.
func (p *Program) RenderCounter() uint64 { return p.renderCounter }

func (p *Program) Run() error {
	term, err := NewTerminal(os.Stdin, os.Stdout)
	if err != nil {
		return errors.Wrap(err, "failed to initialize terminal")
	}

	p.terminal = term

	err = term.EnterRawMode()
	if err != nil {
		return errors.Wrap(err, "failed to enter raw mode")
	}
	defer term.ExitRawMode()

	term.EnterAltScreen()
	defer term.ExitAltScreen()

	term.HideCursor()
	defer term.ShowCursor()

	term.EnableMouse()
	defer term.DisableMouse()

	p.width, p.height = term.Size()

	// Clear screen
	p.outBuf = append(p.outBuf[:0], "\x1b[2J\x1b[H"...)

	_, err = os.Stdout.Write(p.outBuf)
	if err != nil {
		return errors.Wrap(err, "failed to clear screen")
	}

	// Probe before the first frame: raw mode and the alt screen are active, and
	// the input goroutine does not exist yet, so the probe cannot race it. Bytes
	// that are not probe replies are user keystrokes and go to the input loop.
	probeLeftover := p.probeWidths()

	p.processInitCmds()

	sigCh := term.WatchResize()
	defer term.StopWatchResize()

	inputDone := make(chan struct{})
	go p.readInput(inputDone, probeLeftover)

	defer func() { <-inputDone }()
	defer close(p.stopCh)

	err = p.eventLoop(sigCh)
	if err != nil {
		return err
	}

	// Clear the alt screen buffer and move cursor home before exiting.
	p.outBuf = append(p.outBuf[:0], "\x1b[2J\x1b[H"...)
	_, _ = os.Stdout.Write(p.outBuf)

	return nil
}

// probeWidths runs the configured probe and installs the widths, returning
// leftover input bytes for the input loop. A failure keeps the static width table
// and is logged at debug level.
func (p *Program) probeWidths() []byte {
	if len(p.widthProbeGlyphs) == 0 || p.widthProbeInstall == nil {
		return nil
	}

	start := time.Now()

	measured, leftover, err := p.terminal.ProbeGlyphWidths(p.widthProbeGlyphs)
	if err != nil {
		log.Debug().Err(err).Msg("terminal glyph width probe failed; keeping static widths")

		return leftover
	}

	if len(measured) > 0 {
		p.widthProbeInstall(measured, time.Since(start))
	}

	return leftover
}

// processInitCmds runs the model's Init and initial WindowSize update, then
// renders the first frame.
func (p *Program) processInitCmds() {
	initCmds := p.model.Init()
	for _, cmd := range initCmds {
		p.processCmds(cmd)
	}

	sizeCmds := p.model.Update(WindowSizeMsg{Width: p.width, Height: p.height})
	p.processCmds(sizeCmds)
	p.renderFrame()

	postCmds := p.model.Update(PostRenderMsg{})
	p.processCmds(postCmds)
}

//nolint:cyclop,unparam
func (p *Program) eventLoop(sigCh <-chan os.Signal) error {
	for {
		select {
		case msg, ok := <-p.msgCh:
			if !ok {
				return nil
			}

			_, ok = msg.(QuitMsg)
			if ok {
				return nil
			}

			batch, ok := msg.(batchMsg)
			if ok {
				for _, msg := range batch.msgs {
					_, ok = msg.(QuitMsg)
					if ok {
						return nil
					}

					cmds := p.model.Update(p.enrichMouseMsg(msg))
					p.processCmds(cmds)
				}
			} else {
				cmds := p.model.Update(p.enrichMouseMsg(msg))
				p.processCmds(cmds)
			}

			p.renderFrame()

			postCmds := p.model.Update(PostRenderMsg{})
			p.processCmds(postCmds)
		case <-sigCh:
			newWidth, newHeight := p.terminal.UpdateSize()
			if newWidth != p.width || newHeight != p.height {
				p.width = newWidth
				p.height = newHeight
				p.forceFullRender = true
				cmds := p.model.Update(WindowSizeMsg{Width: newWidth, Height: newHeight})
				p.processCmds(cmds)
				p.renderFrame()

				postCmds := p.model.Update(PostRenderMsg{})
				p.processCmds(postCmds)
			}
		}
	}
}

func (p *Program) renderFrame() {
	cur := p.frames[p.curFrame]
	cur.Reset()

	p.renderCounter++
	p.model.Render(cur, p.renderCounter)

	if cur.Len() == 0 {
		return
	}

	prevBuf := p.frames[1-p.curFrame]

	if p.raw || p.forceFullRender {
		prevBuf.Reset()

		p.forceFullRender = false
	}

	prevCount := prevBuf.Len()
	if cur.Len() < prevCount {
		prevBuf.Reset()

		prevCount = 0
	}

	diffs := cur.Diff(prevBuf)

	if len(diffs) == 0 && cur.Len() >= prevCount {
		p.curFrame = 1 - p.curFrame

		return
	}

	p.outBuf = p.outBuf[:0]
	p.outBuf = RenderLines(p.outBuf, diffs, cur, prevCount, p.height)

	if len(p.outBuf) > 0 {
		_, err := p.terminal.out.Write(p.outBuf)
		if err != nil {
			_, _ = os.Stderr.WriteString("render error: write error\n")
		}
	}

	p.curFrame = 1 - p.curFrame
}

// enrichMouseMsg injects the current frame into MouseClickMsg so
// click handlers can do zone hit-testing without a global.
func (p *Program) enrichMouseMsg(msg Msg) Msg {
	if click, ok := msg.(MouseClickMsg); ok {
		click.Lines = p.frames[p.curFrame]

		return click
	}

	return msg
}

func (p *Program) processCmds(cmd Cmd) {
	if cmd == nil {
		return
	}

	go func(c Cmd) {
		msg := c()
		p.msgCh <- msg
	}(cmd)
}

//nolint:cyclop
func (p *Program) readInput(done chan<- struct{}, initialLeftover []byte) {
	defer close(done)

	buf := make([]byte, 1024) //nolint:mnd

	// Dispatch bytes typed while the probe ran before starting the read loop:
	// they already arrived, so waiting for the next read would strand them.
	leftover := p.dispatchInput(initialLeftover, true)

	readCh := make(chan readResult, 1)

	for {
		go func() {
			n, err := p.terminal.in.Read(buf)
			readCh <- readResult{n: n, err: err}
		}()

		select {
		case <-p.stopCh:
			return
		case readRes := <-readCh:
			if readRes.err != nil {
				select {
				case p.msgCh <- QuitMsg{}:
				default:
				}

				return
			}

			if readRes.n == 0 {
				continue
			}

			// Prepend any leftover bytes from a previous partial read.
			data := buf[:readRes.n]
			if len(leftover) > 0 {
				data = append(leftover, data...)
			}

			// If we filled the buffer, more data may be available;
			// partial sequences should be deferred.
			canHaveMoreData := readRes.n == len(buf)

			leftover = p.dispatchInput(data, canHaveMoreData)
		}
	}
}

// dispatchInput parses data into messages and returns the unconsumed tail when
// more data may follow. Incomplete sequences stay in the tail for the next read.
func (p *Program) dispatchInput(data []byte, canHaveMoreData bool) []byte {
	msgs, consumed := parseInput(data, canHaveMoreData)
	for _, msg := range msgs {
		p.msgCh <- msg
	}

	if consumed < len(data) && canHaveMoreData {
		return append([]byte(nil), data[consumed:]...)
	}

	return nil
}

type readResult struct {
	n   int
	err error
}

func TickCmd(d time.Duration, f func(time.Time) Msg) Cmd {
	return func() Msg {
		time.Sleep(d)

		return f(time.Now())
	}
}

func BatchCmd(cmds ...Cmd) Cmd {
	var nonNil []Cmd

	for _, c := range cmds {
		if c != nil {
			nonNil = append(nonNil, c)
		}
	}

	if len(nonNil) == 0 {
		return nil
	}

	if len(nonNil) == 1 {
		return nonNil[0]
	}

	return func() Msg {
		var batch batchMsg

		for _, c := range nonNil {
			batch.msgs = append(batch.msgs, c())
		}

		return batch
	}
}

type batchMsg struct {
	msgs []Msg
}
