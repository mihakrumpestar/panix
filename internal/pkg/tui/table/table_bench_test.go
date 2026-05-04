package table

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	lipglosstable "charm.land/lipgloss/v2/table"

	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
)

var defaultColumnStyles = []style.Style{{}, {}, {}}

var largeColumnStyles = []style.Style{
	{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
}

func buildLargeRows() [][]string {
	rows := make([][]string, 50)
	for i := range 50 {
		rows[i] = []string{
			strconv.Itoa(i + 1), "✅",
			fmt.Sprintf("flake-%d", i%5), fmt.Sprintf("config-%d", i%10),
			fmt.Sprintf("machine-%d", i), "x86_64", "done", "42",
			"2024-01-01", "24.05", "6.1.0",
		}
	}

	return rows
}

func buildLargeLipglossRows() []string {
	return []string{
		strconv.Itoa(1), "✅", "flake-0", "config-0",
		"machine-0", "x86_64", "done", "42",
		"2024-01-01", "24.05", "6.1.0",
	}
}

// --- Small (3 cols, 10 rows) ---

func BenchmarkTable_Small(b *testing.B) {
	tbl := New().Width(60).Border(style.NormalBorder()).
		Headers("Name", "Status", "Time").
		ColumnStyles(defaultColumnStyles[:3])

	for i := range 10 {
		tbl.Row(fmt.Sprintf("item-%d", i), "done", "1.23s")
	}

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}

func BenchmarkLipglossTable_Small(b *testing.B) {
	tbl := lipglosstable.New().
		Width(60).
		Border(lipgloss.NormalBorder()).
		Headers("Name", "Status", "Time").
		StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })

	for i := range 10 {
		tbl.Row(fmt.Sprintf("item-%d", i), "done", "1.23s")
	}

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}

// --- Large (11 cols, 50 rows) ---

func BenchmarkTable_Large(b *testing.B) {
	tbl := New().Width(120).Border(style.NormalBorder()).
		Headers("#", "Icon", "Flake", "Config", "Machine", "Arch", "Status", "Gen", "Date", "NixOS", "Kernel").
		ColumnStyles(largeColumnStyles[:11])

	tbl.SetRows(buildLargeRows())
	_ = tbl.String()

	b.ResetTimer()

	for b.Loop() {
		tbl.SetRows(buildLargeRows())
		_ = tbl.String()
	}
}

func BenchmarkLipglossTable_Large(b *testing.B) {
	tbl := lipglosstable.New().
		Width(120).
		Border(lipgloss.NormalBorder()).
		Headers("#", "Icon", "Flake", "Config", "Machine", "Arch", "Status", "Gen", "Date", "NixOS", "Kernel").
		StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })

	for i := range 50 {
		tbl.Row(strconv.Itoa(i+1), "✅",
			fmt.Sprintf("flake-%d", i%5), fmt.Sprintf("config-%d", i%10),
			fmt.Sprintf("machine-%d", i), "x86_64", "done", "42",
			"2024-01-01", "24.05", "6.1.0")
	}

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}

// --- LongContent (wide cells that need truncation) ---

func BenchmarkTable_LongContent(b *testing.B) {
	longCell := strings.Repeat("x", 200)

	tbl := New().Width(60).Border(style.NormalBorder()).
		Headers("A", "B").
		SetRows([][]string{{longCell, longCell}})

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}

func BenchmarkLipglossTable_LongContent(b *testing.B) {
	longCell := strings.Repeat("x", 200)

	tbl := lipglosstable.New().
		Width(60).
		Border(lipgloss.NormalBorder()).
		Headers("A", "B").
		Row(longCell, longCell).
		StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}

// --- WithColumnStyles (styled columns) ---

func BenchmarkTable_WithColumnStyles(b *testing.B) {
	styledCols := []style.Style{
		style.NewStyle().Foreground(style.Color("#8BE9FD")),
		style.NewStyle().Foreground(style.Color("#FF5555")),
		style.NewStyle().Foreground(style.Color("#50FA7B")),
	}

	tbl := New().Width(120).Border(style.NormalBorder()).
		Headers("Name", "Status", "Time").
		ColumnStyles(styledCols)

	for i := range 50 {
		tbl.Row(fmt.Sprintf("item-%d", i), "running", "1.23s")
	}

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}

func BenchmarkLipglossTable_WithColumnStyles(b *testing.B) {
	tbl := lipglosstable.New().
		Width(120).
		Border(lipgloss.NormalBorder()).
		Headers("Name", "Status", "Time").
		StyleFunc(func(_, col int) lipgloss.Style {
			switch col {
			case 0:
				return lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
			case 1:
				return lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
			default:
				return lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))
			}
		})

	for i := range 50 {
		tbl.Row(fmt.Sprintf("item-%d", i), "running", "1.23s")
	}

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}

// --- NoChange (SetRows with identical data — cache hit) ---

func BenchmarkTable_NoChange(b *testing.B) {
	rows := buildLargeRows()

	tbl := New().Width(120).Border(style.NormalBorder()).
		Headers("#", "Icon", "Flake", "Config", "Machine", "Arch", "Status", "Gen", "Date", "NixOS", "Kernel").
		ColumnStyles(largeColumnStyles[:11])

	tbl.SetRows(rows)
	_ = tbl.String()

	b.ResetTimer()

	for b.Loop() {
		tbl.SetRows(rows)
		_ = tbl.String()
	}
}

func BenchmarkLipglossTable_NoChange(b *testing.B) {
	tbl := lipglosstable.New().
		Width(120).
		Border(lipgloss.NormalBorder()).
		Headers("#", "Icon", "Flake", "Config", "Machine", "Arch", "Status", "Gen", "Date", "NixOS", "Kernel").
		StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })

	for i := range 50 {
		tbl.Row(strconv.Itoa(i+1), "✅",
			fmt.Sprintf("flake-%d", i%5), fmt.Sprintf("config-%d", i%10),
			fmt.Sprintf("machine-%d", i), "x86_64", "done", "42",
			"2024-01-01", "24.05", "6.1.0")
	}

	_ = tbl.String()

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}

// --- SelectionChange (only selection index changes between renders) ---

func BenchmarkTable_SelectionChange(b *testing.B) {
	tbl := New().Width(120).Border(style.NormalBorder()).
		Headers("#", "Icon", "Flake", "Config", "Machine", "Arch", "Status", "Gen", "Date", "NixOS", "Kernel").
		SelectionBackground(style.Color("#333333")).
		ColumnStyles(largeColumnStyles[:11])

	tbl.SetRows(buildLargeRows())
	tbl.Select(0)
	_ = tbl.String()

	b.ResetTimer()

	for b.Loop() {
		tbl.Select((tbl.SelectedIndex() + 1) % 50)
		_ = tbl.String()
	}
}

func BenchmarkLipglossTable_SelectionChange(b *testing.B) {
	tbl := lipglosstable.New().
		Width(120).
		Border(lipgloss.NormalBorder()).
		Headers("#", "Icon", "Flake", "Config", "Machine", "Arch", "Status", "Gen", "Date", "NixOS", "Kernel").
		StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })

	for i := range 50 {
		tbl.Row(strconv.Itoa(i+1), "✅",
			fmt.Sprintf("flake-%d", i%5), fmt.Sprintf("config-%d", i%10),
			fmt.Sprintf("machine-%d", i), "x86_64", "done", "42",
			"2024-01-01", "24.05", "6.1.0")
	}

	_ = tbl.String()

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}
