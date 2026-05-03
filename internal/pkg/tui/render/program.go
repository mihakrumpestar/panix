package render

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type Program struct {
	model    Model
	terminal *Terminal
	writer   *Writer
	buf      *CellBuf
	prevBuf  *CellBuf
	msgCh    chan Msg
	stopCh   chan struct{}
	width    int
	height   int
	mu       sync.Mutex
}

type ProgramOption func(*Program)

func NewProgram(model Model, opts ...ProgramOption) *Program {
	p := &Program{
		model:  model,
		msgCh:  make(chan Msg, 1024),
		stopCh: make(chan struct{}),
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
	p.buf = NewCellBuf(p.width, p.height)
	p.prevBuf = NewCellBuf(p.width, p.height)
	p.writer = NewWriter(out)

	p.writer.WriteClearScreen()

	if err := p.writer.Flush(); err != nil {
		return errors.Wrap(err, "failed to clear screen")
	}

	initCmds := p.model.Init()
	// Send initial window size so the model knows the terminal dimensions.
	// Without this, models that check dimensions in Render() will always
	// return early and produce no output.
	sizeCmds := p.model.Update(WindowSizeMsg{Width: p.width, Height: p.height})
	allCmds := append(initCmds, sizeCmds...)
	p.processCmds(allCmds)
	p.renderFrame()

	sigCh := term.WatchResize()
	defer term.StopWatchResize()

	inputDone := make(chan struct{})
	go p.readInput(inputDone)

	// Defer order matters (LIFO): stopCh must be closed before waiting on
	// inputDone so that readInput can unblock. The leaked Read() sub-goroutine
	// will exit when ExitRawMode restores the terminal (enabling ICANON
	// unblocks the read on the next keypress) or when the process exits.
	defer func() { <-inputDone }()
	defer close(p.stopCh)

	if err := p.eventLoop(sigCh); err != nil {
		return err
	}

	// Clear the alt screen buffer and move cursor home before exiting.
	// This ensures a clean handoff when ExitAltScreen switches back
	// to the main screen buffer.
	p.writer.WriteClearScreen()
	p.writer.setStyle(DefaultColor, DefaultColor, 0)
	p.writer.Flush()

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
		case <-sigCh:
			w, h := p.terminal.UpdateSize()
			if w != p.width || h != p.height {
				p.width = w
				p.height = h
				p.buf.Resize(w, h)
				p.prevBuf.Resize(w, h)
				// After resize, terminal content may not match prevBuf.
				// Force full redraw by clearing prevBuf so Diff detects
				// all lines as changed.
				p.prevBuf.Clear()
				cmds := p.model.Update(WindowSizeMsg{Width: w, Height: h})
				p.processCmds(cmds)
				p.renderFrame()
			}
		}
	}
}

func (p *Program) renderFrame() {
	prevVersion := p.buf.Version()
	SetCurrentBuf(p.buf)
	p.model.Render(p.buf)

	// If no cells changed during Render, skip the entire terminal write.
	// This makes idle frames (no spinner tick, no content update)
	// essentially free — the model still runs but detects no cell diffs.
	if p.buf.Version() == prevVersion {
		return
	}

	// Incremental Diff: only write lines that actually changed.
	// For a 1/4-screen change (~12 lines), this writes ~75% fewer
	// lines than fullRedraw. For no-change frames (where version-skip
	// didn't fire because ClearLinesBelow bumped the version), Diff's
	// lineChanged() check catches the false positive and returns empty diffs.
	diffs := Diff(p.buf, p.prevBuf)
	if len(diffs) == 0 {
		return
	}

	p.writer.Reset()
	p.writer.WriteDiff(diffs, p.buf)

	err := p.writer.Flush()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "render error: %v\n", err)
	}

	// Selective copy: only update prevBuf for changed lines instead of
	// copying the entire buffer. O(changed_lines × width) instead of
	// O(height × width). For 1/4 change, this is ~75% less memmove work.
	for _, d := range diffs {
		y := d.Y
		if y >= 0 && y < p.prevBuf.height && y < p.buf.height && p.prevBuf.width == p.buf.width {
			off := y * p.buf.width
			copy(p.prevBuf.cells[off:off+p.buf.width], p.buf.cells[off:off+p.buf.width])
			p.prevBuf.lineVersions[y] = p.buf.lineVersions[y]
		}
	}
	p.prevBuf.version = p.buf.version
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
	for {
		// Read in a sub-goroutine so the select below can also watch stopCh.
		// When stopCh is closed, readInput returns immediately, and the
		// sub-goroutine is leaked (it will be cleaned up on process exit
		// when the fd is closed by ExitRawMode).
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

			msgs := parseInput(buf[:r.n])
			for _, msg := range msgs {
				p.msgCh <- msg
			}
		}
	}
}

type readResult struct {
	n   int
	err error
}

func (p *Program) Buf() *CellBuf { return p.buf }

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
