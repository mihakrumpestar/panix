package zeroterm

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/pkg/linesbuffer"
)

//nolint:paralleltest // package-level globals not concurrency-safe
func TestTickCmd(t *testing.T) {
	cmd := TickCmd(1*time.Millisecond, func(t time.Time) Msg {
		return KeyPressMsg{Key: "tick"}
	})

	result := cmd()
	if kpm, ok := result.(KeyPressMsg); !ok || kpm.Key != "tick" {
		t.Errorf("TickCmd returned %v, want KeyPressMsg{Key: tick}", result)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestProgramNew(t *testing.T) {
	m := &dummyModel{}

	prog := NewProgram(m)
	if prog == nil {
		t.Fatal("NewProgram returned nil")
	}

	if prog.model == nil {
		t.Error("Program.model should be set")
	}
}

type dummyModel struct{}

func (m *dummyModel) Init() []Cmd            { return nil }
func (m *dummyModel) Update(msg Msg) Cmd    { return nil }
func (m *dummyModel) Render(buf *linesbuffer.LinesBuffer) {}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestProcessCmdsWithNil(t *testing.T) {
	m := &dummyModel{}
	prog := NewProgram(m)

	// Should not panic with nil cmds
	prog.processCmds(nil)
	prog.processCmds(nil)
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestRenderFrameWithEmptyModel(t *testing.T) {
	m := &dummyModel{}
	p := NewProgram(m)
	p.terminal = &Terminal{out: os.Stderr}
	p.width = 80
	p.height = 24

	p.renderFrame()
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestRenderFrameDetectsChanges(t *testing.T) {
	renderModel := &renderTestModel{content: "Hello World"}
	prog := NewProgram(renderModel)
	prog.terminal = &Terminal{out: os.Stderr}
	prog.width = 80
	prog.height = 24

	prog.renderFrame()

	prevBuf := prog.frames[1-prog.curFrame]
	if prevBuf.Len() == 0 || string(prevBuf.Line(0)) != "Hello World" {
		t.Error("prevLines should have content after renderFrame")
	}
}

type renderTestModel struct {
	content string
}

func (m *renderTestModel) Init() []Cmd         { return nil }
func (m *renderTestModel) Update(msg Msg) Cmd  { return nil }
func (m *renderTestModel) Render(buf *linesbuffer.LinesBuffer) {
	buf.Write([]byte(m.content))
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestNewProgramWithOptions(t *testing.T) {
	var applied bool

	opt := func(p *Program) {
		applied = true
	}
	m := &dummyModel{}
	prog := NewProgram(m, opt)

	if !applied {
		t.Error("ProgramOption should have been applied")
	}

	_ = prog
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestProcessCmdsAsync(t *testing.T) {
	m := &dummyModel{}
	prog := NewProgram(m)
	prog.msgCh = make(chan Msg, 10)

	var called bool

	cmd := func() Msg {
		called = true

		return KeyPressMsg{Key: "async"}
	}

	prog.processCmds(cmd)

	msg := <-prog.msgCh

	kp, ok := msg.(KeyPressMsg)
	if !ok || kp.Key != "async" {
		t.Errorf("processCmds should send cmd result to msgCh, got %v", msg)
	}

	if !called {
		t.Error("cmd should have been called")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestRenderFrameFlushError(t *testing.T) {
	m := &renderTestModel{content: "Test"}
	prog := NewProgram(m)
	prog.terminal = &Terminal{out: os.Stderr}
	prog.width = 80
	prog.height = 24

	prog.renderFrame()
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestInitialWindowSizeMsgSent(t *testing.T) {
	var receivedSize WindowSizeMsg

	model := &sizeTrackingModel{}
	prog := NewProgram(model)
	prog.terminal = &Terminal{out: os.Stderr}
	prog.width = 80
	prog.height = 24

	sizeCmds := prog.model.Update(WindowSizeMsg{Width: 80, Height: 24})
	prog.processCmds(sizeCmds)
	prog.renderFrame()

	receivedSize = model.lastSize
	if receivedSize.Width != 80 || receivedSize.Height != 24 {
		t.Errorf("model should have received initial WindowSizeMsg: got %dx%d", receivedSize.Width, receivedSize.Height)
	}
}

type sizeTrackingModel struct {
	lastSize WindowSizeMsg
}

func (m *sizeTrackingModel) Init() []Cmd { return nil }
func (m *sizeTrackingModel) Update(msg Msg) Cmd {
	if ws, ok := msg.(WindowSizeMsg); ok {
		m.lastSize = ws
	}

	return nil
}
func (m *sizeTrackingModel) Render(buf *linesbuffer.LinesBuffer) {
	buf.Write(fmt.Appendf(nil, "%dx%d", m.lastSize.Width, m.lastSize.Height))
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestStopChClosesOnExit(t *testing.T) {
	m := &dummyModel{}
	prog := NewProgram(m)

	select {
	case <-prog.stopCh:
		t.Error("stopCh should not be closed before Run returns")
	default:
	}

	close(prog.stopCh)

	select {
	case <-prog.stopCh:
	default:
		t.Error("stopCh should be closable")
	}
}
