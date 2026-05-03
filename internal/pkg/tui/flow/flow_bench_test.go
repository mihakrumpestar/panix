package flow

import (
	"fmt"
	"testing"

	"charm.land/lipgloss/v2"
	lipglosstable "charm.land/lipgloss/v2/table"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
)

func benchStyles() Styles {
	return Styles{
		GradientRunning: GradientPair{
			Dark:  colorfulMustParseHex("#01536e"),
			Light: colorfulMustParseHex("#007da7"),
		},
		GradientFailed: GradientPair{
			Dark:  colorfulMustParseHex("#5f1414"),
			Light: colorfulMustParseHex("#DC2626"),
		},
		GradientDone: GradientPair{
			Dark:  colorfulMustParseHex("#14532D"),
			Light: colorfulMustParseHex("#11883d"),
		},
		GradientDefault: GradientPair{
			Dark:  colorfulMustParseHex("#535862"),
			Light: colorfulMustParseHex("#6B7280"),
		},
		Pill:            style.NewStyle().Foreground(style.Color("#FFFFFF")).Bold(true).Padding(0, 1),
		StatusRunning:   style.NewStyle().Foreground(style.Color("#00BFFF")),
		StatusFailed:    style.NewStyle().Foreground(style.Color("#FF5555")),
		StatusDone:      style.NewStyle().Foreground(style.Color("#50FA7B")),
		StatusSeparator: style.NewStyle().Foreground(style.Color("#6272A4")),
		Arrow:           style.NewStyle().Foreground(style.Color("#6272A4")),
		SelectionBg:     style.Color("#3B3258"),
	}
}

func benchPhases() []string {
	return []string{"INSPECT", "BUILD", "TRANSFER", "ACTIVATE", "DONE"}
}

func benchData() []PhaseData {
	return []PhaseData{
		{Running: 1, Failed: 0, Done: 0},
		{Running: 1, Failed: 0, Done: 0},
		{Running: 0, Failed: 0, Done: 1},
		{Running: 0, Failed: 1, Done: 0},
		{Running: 0, Failed: 0, Done: 3},
	}
}

func colorfulMustParseHex(hex string) colorful.Color {
	c, err := colorful.Hex(hex)
	if err != nil {
		panic(err)
	}
	return c
}

// --- Small (5 phases, width 80) ---

func BenchmarkFlow_Small(b *testing.B) {
	pf := New().Width(80).Phases(benchPhases()...).Styles(benchStyles())
	pf.SetData(benchData())
	_ = pf.String()

	b.ResetTimer()

	for b.Loop() {
		_ = pf.String()
	}
}

func BenchmarkLipglossTable_Small(b *testing.B) {
	headers := benchPhases()
	data := benchData()

	row := make([]string, len(headers))
	for i, d := range data {
		switch {
		case d.Running > 0:
			row[i] = fmt.Sprintf("%d", d.Running)
		case d.Failed > 0:
			row[i] = fmt.Sprintf("%d", d.Failed)
		case d.Done > 0:
			row[i] = fmt.Sprintf("%d", d.Done)
		default:
			row[i] = ""
		}
	}

	tbl := lipglosstable.New().
		Width(80).
		Border(lipgloss.NormalBorder()).
		Headers(headers...).
		Row(row...).
		StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}

// --- NoChange (same data — cache hit) ---

func BenchmarkFlow_NoChange(b *testing.B) {
	pf := New().Width(80).Phases(benchPhases()...).Styles(benchStyles())
	data := benchData()
	pf.SetData(data)
	_ = pf.String()

	b.ResetTimer()

	for b.Loop() {
		pf.SetData(data)
		_ = pf.String()
	}
}

func BenchmarkLipglossTable_NoChange(b *testing.B) {
	headers := benchPhases()
	data := benchData()

	row := make([]string, len(headers))
	for i, d := range data {
		switch {
		case d.Running > 0:
			row[i] = fmt.Sprintf("%d", d.Running)
		case d.Failed > 0:
			row[i] = fmt.Sprintf("%d", d.Failed)
		case d.Done > 0:
			row[i] = fmt.Sprintf("%d", d.Done)
		default:
			row[i] = ""
		}
	}

	tbl := lipglosstable.New().
		Width(80).
		Border(lipgloss.NormalBorder()).
		Headers(headers...).
		Row(row...).
		StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })

	_ = tbl.String()

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}

// --- WithGradient (gradient animation active) ---

func BenchmarkFlow_WithGradient(b *testing.B) {
	pf := New().Width(120).Phases(benchPhases()...).Styles(benchStyles())
	pf.SetData(benchData())
	_ = pf.String()

	b.ResetTimer()

	for b.Loop() {
		_ = pf.String()
	}
}

func BenchmarkLipglossTable_WithGradient(b *testing.B) {
	headers := benchPhases()
	data := benchData()

	row := make([]string, len(headers))
	for i, d := range data {
		switch {
		case d.Running > 0:
			row[i] = fmt.Sprintf("%d", d.Running)
		case d.Failed > 0:
			row[i] = fmt.Sprintf("%d", d.Failed)
		case d.Done > 0:
			row[i] = fmt.Sprintf("%d", d.Done)
		default:
			row[i] = ""
		}
	}

	bgStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#01536e")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Align(lipgloss.Center)

	tbl := lipglosstable.New().
		Width(120).
		Border(lipgloss.NormalBorder()).
		Headers(headers...).
		Row(row...).
		StyleFunc(func(_, _ int) lipgloss.Style { return bgStyle })

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}

// --- SelectionChange (selection index changes between renders) ---

func BenchmarkFlow_SelectionChange(b *testing.B) {
	pf := New().Width(80).Phases(benchPhases()...).Styles(benchStyles())
	pf.SetData(benchData())
	pf.HandleNavigation("right", false)
	_ = pf.String()

	b.ResetTimer()

	for b.Loop() {
		pf.HandleNavigation("right", false)
		_ = pf.String()
	}
}

func BenchmarkLipglossTable_SelectionChange(b *testing.B) {
	headers := benchPhases()
	data := benchData()

	row := make([]string, len(headers))
	for i, d := range data {
		switch {
		case d.Running > 0:
			row[i] = fmt.Sprintf("%d", d.Running)
		case d.Failed > 0:
			row[i] = fmt.Sprintf("%d", d.Failed)
		case d.Done > 0:
			row[i] = fmt.Sprintf("%d", d.Done)
		default:
			row[i] = ""
		}
	}

	selStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#3B3258")).
		Align(lipgloss.Center)

	tbl := lipglosstable.New().
		Width(80).
		Border(lipgloss.NormalBorder()).
		Headers(headers...).
		Row(row...).
		StyleFunc(func(_, _ int) lipgloss.Style { return selStyle })

	_ = tbl.String()

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}
