package zeroterm

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestTickCmd(t *testing.T) {
	t.Parallel()

	cmd := TickCmd(1*time.Millisecond, func(t time.Time) Msg {
		return KeyPressMsg{Key: "tick"}
	})

	result := cmd()
	if kpm, ok := result.(KeyPressMsg); !ok || kpm.Key != "tick" {
		t.Errorf("TickCmd returned %v, want KeyPressMsg{Key: tick}", result)
	}
}

func TestBatchCmd(t *testing.T) {
	t.Parallel()

	cmd := BatchCmd(
		func() Msg { return KeyPressMsg{Key: "a"} },
		func() Msg { return KeyPressMsg{Key: "b"} },
	)

	result := cmd()

	batch, ok := result.(batchMsg)
	if !ok {
		t.Fatalf("BatchCmd should return batchMsg, got %T", result)
	}

	if len(batch.msgs) != 2 {
		t.Fatalf("batch should have 2 msgs, got %d", len(batch.msgs))
	}
}

func TestBatchCmdWithNil(t *testing.T) {
	t.Parallel()

	cmd := BatchCmd(
		func() Msg { return KeyPressMsg{Key: "a"} },
		nil,
		func() Msg { return KeyPressMsg{Key: "c"} },
	)

	result := cmd()

	batch, ok := result.(batchMsg)
	if !ok {
		t.Fatalf("BatchCmd should return batchMsg, got %T", result)
	}

	if len(batch.msgs) != 2 {
		t.Errorf("batch should have 2 msgs (nil skipped), got %d", len(batch.msgs))
	}
}

func TestBatchCmdEmpty(t *testing.T) {
	t.Parallel()

	cmd := BatchCmd()
	result := cmd()

	batch, ok := result.(batchMsg)
	if !ok {
		t.Fatalf("BatchCmd should return batchMsg, got %T", result)
	}

	if len(batch.msgs) != 0 {
		t.Errorf("empty batch should have 0 msgs, got %d", len(batch.msgs))
	}
}

func TestProgramNew(t *testing.T) {
	t.Parallel()

	m := &dummyModel{}

	p := NewProgram(m)
	if p == nil {
		t.Error("NewProgram returned nil")
	}

	if p.model == nil {
		t.Error("Program.model should be set")
	}
}

type dummyModel struct{}

func (m *dummyModel) Init() []Cmd          { return nil }
func (m *dummyModel) Update(msg Msg) []Cmd { return nil }
func (m *dummyModel) Render() []string     { return nil }

func TestProcessCmdsWithNil(t *testing.T) {
	t.Parallel()

	m := &dummyModel{}
	p := NewProgram(m)

	// Should not panic with nil cmds
	p.processCmds(nil)
	p.processCmds([]Cmd{nil, nil})
}

func TestRenderFrameWithEmptyModel(t *testing.T) {
	t.Parallel()

	m := &dummyModel{}
	p := NewProgram(m)
	p.terminal = &Terminal{out: os.Stderr}
	p.width = 80
	p.height = 24

	p.renderFrame()
}

func TestRenderFrameDetectsChanges(t *testing.T) {
	t.Parallel()

	renderModel := &renderTestModel{content: "Hello World"}
	p := NewProgram(renderModel)
	p.terminal = &Terminal{out: os.Stderr}
	p.width = 80
	p.height = 24

	p.renderFrame()

	if len(p.prevLines) == 0 || p.prevLines[0] != "Hello World" {
		t.Error("prevLines should have content after renderFrame")
	}
}

type renderTestModel struct {
	content string
}

func (m *renderTestModel) Init() []Cmd          { return nil }
func (m *renderTestModel) Update(msg Msg) []Cmd { return nil }
func (m *renderTestModel) Render() []string {
	return []string{m.content}
}

func TestNewProgramWithOptions(t *testing.T) {
	t.Parallel()

	var applied bool

	opt := func(p *Program) {
		applied = true
	}
	m := &dummyModel{}
	p := NewProgram(m, opt)

	if !applied {
		t.Error("ProgramOption should have been applied")
	}

	_ = p
}

func TestProcessCmdsAsync(t *testing.T) {
	t.Parallel()

	m := &dummyModel{}
	p := NewProgram(m)
	p.msgCh = make(chan Msg, 10)

	var called bool

	cmd := func() Msg {
		called = true

		return KeyPressMsg{Key: "async"}
	}

	p.processCmds([]Cmd{cmd})

	msg := <-p.msgCh

	kp, ok := msg.(KeyPressMsg)
	if !ok || kp.Key != "async" {
		t.Errorf("processCmds should send cmd result to msgCh, got %v", msg)
	}

	if !called {
		t.Error("cmd should have been called")
	}
}

func TestRenderFrameFlushError(t *testing.T) {
	t.Parallel()

	m := &renderTestModel{content: "Test"}
	p := NewProgram(m)
	p.terminal = &Terminal{out: os.Stderr}
	p.width = 80
	p.height = 24

	p.renderFrame()
}

func TestInitialWindowSizeMsgSent(t *testing.T) {
	t.Parallel()

	var receivedSize WindowSizeMsg

	m := &sizeTrackingModel{}
	p := NewProgram(m)
	p.terminal = &Terminal{out: os.Stderr}
	p.width = 80
	p.height = 24

	sizeCmds := p.model.Update(WindowSizeMsg{Width: 80, Height: 24})
	p.processCmds(sizeCmds)
	p.renderFrame()

	receivedSize = m.lastSize
	if receivedSize.Width != 80 || receivedSize.Height != 24 {
		t.Errorf("model should have received initial WindowSizeMsg: got %dx%d", receivedSize.Width, receivedSize.Height)
	}
}

type sizeTrackingModel struct {
	lastSize WindowSizeMsg
}

func (m *sizeTrackingModel) Init() []Cmd { return nil }
func (m *sizeTrackingModel) Update(msg Msg) []Cmd {
	if ws, ok := msg.(WindowSizeMsg); ok {
		m.lastSize = ws
	}

	return nil
}
func (m *sizeTrackingModel) Render() []string {
	return []string{fmt.Sprintf("%dx%d", m.lastSize.Width, m.lastSize.Height)}
}

func TestStopChClosesOnExit(t *testing.T) {
	t.Parallel()

	m := &dummyModel{}
	p := NewProgram(m)

	select {
	case <-p.stopCh:
		t.Error("stopCh should not be closed before Run returns")
	default:
	}

	close(p.stopCh)

	select {
	case <-p.stopCh:
	default:
		t.Error("stopCh should be closable")
	}
}
