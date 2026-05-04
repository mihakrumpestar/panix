package zeroterm

import (
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type Program struct {
	model     Model
	terminal  *Terminal
	prevLines []string
	outBuf    []byte
	msgCh     chan Msg
	stopCh    chan struct{}
	width     int
	height    int
	raw       bool
	mu        sync.Mutex
}

type ProgramOption func(*Program)

// WithRaw disables frame diffing; every render writes all lines directly.
func WithRaw() ProgramOption {
	return func(p *Program) { p.raw = true }
}

func NewProgram(model Model, opts ...ProgramOption) *Program {
	p := &Program{
		model:  model,
		msgCh:  make(chan Msg, 1024),
		stopCh: make(chan struct{}),
		outBuf: make([]byte, 0, 8192),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *Program) Run() error {
	in := os.Stdin
	out := os.Stdout

	term, err := NewTerminal(in, out)
	if err != nil {
		return errors.Wrap(err, "failed to initialize terminal")
	}

	p.terminal = term

	if err := term.EnterRawMode(); err != nil {
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
	if _, err := out.Write(p.outBuf); err != nil {
		return errors.Wrap(err, "failed to clear screen")
	}

	initCmds := p.model.Init()
	sizeCmds := p.model.Update(WindowSizeMsg{Width: p.width, Height: p.height})
	allCmds := append(initCmds, sizeCmds...)
	p.processCmds(allCmds)
	p.renderFrame()

	// Post-render update lets the model emit ticks (e.g. spinners) after
	// the first render has populated view state.
	postCmds := p.model.Update(PostRenderMsg{})
	p.processCmds(postCmds)

	sigCh := term.WatchResize()
	defer term.StopWatchResize()

	inputDone := make(chan struct{})
	go p.readInput(inputDone)

	defer func() { <-inputDone }()
	defer close(p.stopCh)

	if err := p.eventLoop(sigCh); err != nil {
		return err
	}

	// Clear the alt screen buffer and move cursor home before exiting.
	p.outBuf = append(p.outBuf[:0], "\x1b[2J\x1b[H"...)
	out.Write(p.outBuf)

	return nil
}

//nolint:cyclop
func (p *Program) eventLoop(sigCh <-chan os.Signal) error {
	for {
		select {
		case msg, ok := <-p.msgCh:
			if !ok {
				return nil
			}

			if _, ok := msg.(QuitMsg); ok {
				return nil
			}

			if batch, ok := msg.(batchMsg); ok {
				for _, m := range batch.msgs {
					if _, ok := m.(QuitMsg); ok {
						return nil
					}

					cmds := p.model.Update(m)
					p.processCmds(cmds)
				}
			} else {
				cmds := p.model.Update(msg)
				p.processCmds(cmds)
			}

			p.renderFrame()

			// Post-render update lets the model emit cmds based on what
			// was rendered (e.g. spinners that were viewed need ticks).
			postCmds := p.model.Update(PostRenderMsg{})
			p.processCmds(postCmds)
		case <-sigCh:
			w, h := p.terminal.UpdateSize()
			if w != p.width || h != p.height {
				p.width = w
				p.height = h
				// After resize, force full redraw by clearing prevLines
				// so all lines are detected as changed.
				p.prevLines = p.prevLines[:0]
				cmds := p.model.Update(WindowSizeMsg{Width: w, Height: h})
				p.processCmds(cmds)
				p.renderFrame()

				postCmds := p.model.Update(PostRenderMsg{})
				p.processCmds(postCmds)
			}
		}
	}
}

func (p *Program) renderFrame() {
	lines := p.model.Render()

	// nil means nothing changed (model-level cache hit)
	if lines == nil {
		return
	}

	// Store lines for zone resolution on mouse clicks
	SetCurrentLines(lines)

	if p.raw {
		p.prevLines = p.prevLines[:0]
	}

	prevCount := len(p.prevLines)

	// Diff against previous frame
	diffs := DiffLines(lines, p.prevLines)

	if !p.raw && len(diffs) == 0 && len(lines) >= len(p.prevLines) {
		// Nothing changed — update prevLines and return
		p.updatePrevLines(lines)

		return
	}

	// Render changed lines
	p.outBuf = p.outBuf[:0]
	p.outBuf = RenderLines(p.outBuf, diffs, lines, prevCount, p.height)

	if len(p.outBuf) > 0 {
		if _, err := p.terminal.out.Write(p.outBuf); err != nil {
			_, _ = os.Stderr.WriteString("render error: write error\n")
		}
	}

	p.updatePrevLines(lines)
}

// updatePrevLines stores the current lines for next frame's diff.
// Reuses the backing array when possible to minimize allocations.
func (p *Program) updatePrevLines(lines []string) {
	if cap(p.prevLines) >= len(lines) {
		p.prevLines = p.prevLines[:len(lines)]
	} else {
		p.prevLines = make([]string, len(lines))
	}

	copy(p.prevLines, lines)
}

func (p *Program) processCmds(cmds []Cmd) {
	for _, cmd := range cmds {
		if cmd == nil {
			continue
		}

		go func(c Cmd) {
			msg := c()
			p.msgCh <- msg
		}(cmd)
	}
}

func (p *Program) readInput(done chan<- struct{}) {
	defer close(done)

	buf := make([]byte, 1024)

	var leftover []byte

	for {
		readCh := make(chan readResult, 1)

		go func() {
			n, err := p.terminal.in.Read(buf)
			readCh <- readResult{n: n, err: err}
		}()

		select {
		case <-p.stopCh:
			return
		case r := <-readCh:
			if r.err != nil {
				select {
				case p.msgCh <- QuitMsg{}:
				default:
				}

				return
			}

			if r.n == 0 {
				continue
			}

			// Prepend any leftover bytes from a previous partial read.
			data := buf[:r.n]
			if len(leftover) > 0 {
				data = append(leftover, data...)
				leftover = nil
			}

			// If we filled the buffer, more data may be available;
			// partial sequences should be deferred.
			canHaveMoreData := r.n == len(buf)

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
	return func() Msg {
		var batch batchMsg

		for _, c := range cmds {
			if c != nil {
				batch.msgs = append(batch.msgs, c())
			}
		}

		return batch
	}
}

type batchMsg struct {
	msgs []Msg
}
