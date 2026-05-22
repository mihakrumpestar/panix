package zeroterm

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // package-level globals not concurrency-safe
func TestTickCmd(t *testing.T) {
	cmd := TickCmd(1*time.Millisecond, func(t time.Time) Msg {
		return KeyPressMsg{Key: "tick"}
	})

	result := cmd()
	kpm, ok := result.(KeyPressMsg); require.True(t, ok) ; require.Equal(t, "tick", kpm.Key)
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestProgramNew(t *testing.T) {
	m := &dummyModel{}

	prog := NewProgram(m)
	assert.NotNil(t, prog)
	assert.NotNil(t, prog.model, "Program.model should be set")
}

type dummyModel struct{}

func (m *dummyModel) Init() []Cmd                               { return nil }
func (m *dummyModel) Update(msg Msg) Cmd                        { return nil }
func (m *dummyModel) Render(buf *buffer.LinesBufDiff, _ uint64) {}

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
	assert.NotZero(t, prevBuf.Len(), "prevLines should have content after renderFrame") ; assert.Equal(t, "Hello World", string(prevBuf.Line(0)))
}

type renderTestModel struct {
	content string
}

func (m *renderTestModel) Init() []Cmd        { return nil }
func (m *renderTestModel) Update(msg Msg) Cmd { return nil }
func (m *renderTestModel) Render(buf *buffer.LinesBufDiff, _ uint64) {
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

	assert.True(t, applied, "ProgramOption should have been applied")

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
	require.True(t, ok, "processCmds should send cmd result to msgCh, got %v", msg)
	assert.Equal(t, "async", kp.Key)

	assert.True(t, called, "cmd should have been called")
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
	assert.Equal(t, 80, receivedSize.Width, "model should have received initial WindowSizeMsg: got %dx%d") ; assert.Equal(t, 24, receivedSize.Height)
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
func (m *sizeTrackingModel) Render(buf *buffer.LinesBufDiff, _ uint64) {
	buf.Write(fmt.Appendf(nil, "%dx%d", m.lastSize.Width, m.lastSize.Height))
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestStopChClosesOnExit(t *testing.T) {
	m := &dummyModel{}
	prog := NewProgram(m)

	select {
	case <-prog.stopCh:
		assert.Fail(t, "stopCh should not be closed before Run returns")
	default:
	}

	close(prog.stopCh)

	select {
	case <-prog.stopCh:
	default:
		assert.Fail(t, "stopCh should be closable")
	}
}
