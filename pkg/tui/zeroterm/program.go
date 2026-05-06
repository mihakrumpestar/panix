// Derived from charm.land/bubbletea/v2. See pkg/tui/LICENSE.charmbracelet.

package zeroterm

import (
	"os"
	"time"

	"github.com/pkg/errors"
)

type Program struct {
	model     Model
	terminal  *Terminal
	prevLines [][]byte
	outBuf    []byte
	msgCh     chan Msg
	stopCh    chan struct{}
	width     int
	height    int
	raw       bool
	frames    [2]RenderBuffer
	curFrame  int
}

type ProgramOption func(*Program)

// Raw disables frame diffing; every render writes all lines directly.
func Raw() ProgramOption {
	return func(p *Program) { p.raw = true }
}

func NewProgram(model Model, opts ...ProgramOption) *Program {
	prog := &Program{
		model:  model,
		msgCh:  make(chan Msg, 1024), //nolint:mnd
		stopCh: make(chan struct{}),
		outBuf: make([]byte, 0, 8192), //nolint:mnd
	}

	for _, opt := range opts {
		opt(prog)
	}

	return prog
}

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

	p.processInitCmds()

	sigCh := term.WatchResize()
	defer term.StopWatchResize()

	inputDone := make(chan struct{})
	go p.readInput(inputDone)

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

					cmds := p.model.Update(msg)
					p.processCmds(cmds)
				}
			} else {
				cmds := p.model.Update(msg)
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
				p.prevLines = p.prevLines[:0]
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
	cur := &p.frames[p.curFrame]
	cur.Reset()
	p.model.Render(cur)

	if len(cur.Lines()) == 0 {
		return
	}

	lines := cur.Lines()

	SetCurrentLines(lines)

	if p.raw {
		p.prevLines = p.prevLines[:0]
	}

	prevCount := len(p.prevLines)
	diffs := DiffLines(lines, p.prevLines)

	if !p.raw && len(diffs) == 0 && len(lines) >= len(p.prevLines) {
		p.prevLines = lines
		p.curFrame = 1 - p.curFrame

		return
	}

	p.outBuf = p.outBuf[:0]
	p.outBuf = RenderLines(p.outBuf, diffs, lines, prevCount, p.height)

	if len(p.outBuf) > 0 {
		_, err := p.terminal.out.Write(p.outBuf)
		if err != nil {
			_, _ = os.Stderr.WriteString("render error: write error\n")
		}
	}

	p.prevLines = lines
	p.curFrame = 1 - p.curFrame
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
func (p *Program) readInput(done chan<- struct{}) {
	defer close(done)

	buf := make([]byte, 1024) //nolint:mnd

	var leftover []byte

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
				leftover = nil
			}

			// If we filled the buffer, more data may be available;
			// partial sequences should be deferred.
			canHaveMoreData := readRes.n == len(buf)

			msgs, consumed := parseInput(data, canHaveMoreData)
			for _, msg := range msgs {
				p.msgCh <- msg
			}

			if consumed < len(data) && canHaveMoreData {
				// Incomplete sequence at end of buffer — save for next read.
				leftover = make([]byte, 0, len(data[consumed:])+len(buf))
				leftover = append(leftover, data[consumed:]...)
			}
		}
	}
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
