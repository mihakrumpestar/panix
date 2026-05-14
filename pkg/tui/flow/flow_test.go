package flow

import (
	"testing"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

func makeTestStyles(hexFn func(string) colorful.Color) Styles {
	white := style.NewStyle().Foreground(style.Color("#FFFFFF")).Bold(true).Padding(0, 1)

	return Styles{
		GradientRunning: GradientPair{Dark: hexFn("#01536e"), Light: hexFn("#007da7")},
		GradientFailed:  GradientPair{Dark: hexFn("#5f1414"), Light: hexFn("#DC2626")},
		GradientDone:    GradientPair{Dark: hexFn("#14532D"), Light: hexFn("#11883d")},
		GradientDefault: GradientPair{Dark: hexFn("#535862"), Light: hexFn("#6B7280")},
		Pill:            white,
		Status: StatusStyles{
			Running: style.NewStyle().Foreground(style.Color("#00BFFF")),
			Failed:  style.NewStyle().Foreground(style.Color("#FF5555")),
			Done:    style.NewStyle().Foreground(style.Color("#50FA7B")),
		},
		StatusSeparator: style.NewStyle().Foreground(style.Color("#6272A4")),
		Arrow:           style.NewStyle().Foreground(style.Color("#6272A4")),
		PhaseArrow:      []byte(string(rune(0x1FB74))),
		SelectionBg:     style.Color("#3B3258"),
	}
}

func TestDetermineState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		data PhaseData
		want PhaseState
		name string
	}{
		{PhaseData{}, Idle, "empty"},
		{PhaseData{Running: 1}, StateRunning, "running"},
		{PhaseData{Failed: 1}, StateFailed, "failed"},
		{PhaseData{Done: 1}, StateDone, "done"},
		{PhaseData{Running: 1, Failed: 1}, StateRunning, "running_takes_priority"},
		{PhaseData{Failed: 1, Done: 1}, StateFailed, "failed_over_done"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := determineState(tt.data); got != tt.want {
				t.Errorf("determineState(%+v) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}
