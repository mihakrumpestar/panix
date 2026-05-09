package table

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	lipglosstable "charm.land/lipgloss/v2/table"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

var defaultColumnStyles = []style.Style{{}, {}, {}}

var largeColumnStyles = []style.Style{
	{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
}

var largeHeaders = []string{"#", "Icon", "Flake", "Config", "Machine", "Arch", "Status", "Gen", "Date", "NixOS", "Kernel"}

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

// --- Small (3 cols, 10 rows) ---

func Benchmark__Table_Small(b *testing.B) {
	tbl := New(Config{
		Width:        60,
		Border:       style.NormalBorder(),
		Headers:      []string{"Name", "Status", "Time"},
		ColumnStyles: defaultColumnStyles[:3],
	})

	rows := make([][]string, 10)
	for i := range 10 {
		rows[i] = []string{fmt.Sprintf("item-%d", i), "done", "1.23s"}
	}

	tbl.SetRows(rows)

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}

func Benchmark_Lipgloss__Table_Small(b *testing.B) {
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

func Benchmark__Table_Large(b *testing.B) {
	rows := buildLargeRows()

	tbl := New(Config{
		Width:        120,
		Border:       style.NormalBorder(),
		Headers:      largeHeaders,
		ColumnStyles: largeColumnStyles[:11],
	})

	tbl.SetRows(rows)
	_ = tbl.String()

	b.ResetTimer()

	for b.Loop() {
		tbl.SetRows(rows)
		_ = tbl.String()
	}
}

// Table_Large_FreshRows measures SetRows with different data each iteration
// (simulates rows being updated from external source). The rows are pre-built
// so the benchmark only measures table rendering, not data construction.
func Benchmark__Table_Large_FreshRows(b *testing.B) {
	rowsA := buildLargeRows()

	rowsB := make([][]string, len(rowsA))
	for i, row := range rowsA {
		rowsB[i] = make([]string, len(row))
		copy(rowsB[i], row)
	}

	rowsB[0][6] = "running"

	tbl := New(Config{
		Width:        120,
		Border:       style.NormalBorder(),
		Headers:      largeHeaders,
		ColumnStyles: largeColumnStyles[:11],
	})

	tbl.SetRows(rowsA)
	_ = tbl.String()

	b.ResetTimer()

	for b.Loop() {
		tbl.SetRows(rowsA)
		_ = tbl.String()
		tbl.SetRows(rowsB)
		_ = tbl.String()
	}
}

func Benchmark_Lipgloss__Table_Large_FreshRows(b *testing.B) {
	rowsA := buildLargeRows()

	rowsB := make([][]string, len(rowsA))
	for i, row := range rowsA {
		rowsB[i] = make([]string, len(row))
		copy(rowsB[i], row)
	}

	rowsB[0][6] = "running"

	tbl := lipglosstable.New().
		Width(120).
		Border(lipgloss.NormalBorder()).
		Headers(largeHeaders...).
		StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })

	for _, row := range rowsA {
		tbl.Row(row...)
	}

	_ = tbl.String()

	b.ResetTimer()

	for b.Loop() {
		tbl = lipglosstable.New().
			Width(120).
			Border(lipgloss.NormalBorder()).
			Headers(largeHeaders...).
			StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })
		for _, row := range rowsA {
			tbl.Row(row...)
		}

		_ = tbl.String()

		tbl = lipglosstable.New().
			Width(120).
			Border(lipgloss.NormalBorder()).
			Headers(largeHeaders...).
			StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })
		for _, row := range rowsB {
			tbl.Row(row...)
		}

		_ = tbl.String()
	}
}

func Benchmark_Lipgloss__Table_Large(b *testing.B) {
	tbl := lipglosstable.New().
		Width(120).
		Border(lipgloss.NormalBorder()).
		Headers(largeHeaders...).
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

func Benchmark__Table_LongContent(b *testing.B) {
	longCell := strings.Repeat("x", 200)

	tbl := New(Config{
		Width:   60,
		Border:  style.NormalBorder(),
		Headers: []string{"A", "B"},
	})
	tbl.SetRows([][]string{{longCell, longCell}})

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}

func Benchmark_Lipgloss__Table_LongContent(b *testing.B) {
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

func Benchmark__Table_WithColumnStyles(b *testing.B) {
	styledCols := []style.Style{
		style.NewStyle().Foreground(style.Color("#8BE9FD")),
		style.NewStyle().Foreground(style.Color("#FF5555")),
		style.NewStyle().Foreground(style.Color("#50FA7B")),
	}

	tbl := New(Config{
		Width:        120,
		Border:       style.NormalBorder(),
		Headers:      []string{"Name", "Status", "Time"},
		ColumnStyles: styledCols,
	})

	rows := make([][]string, 50)
	for i := range 50 {
		rows[i] = []string{fmt.Sprintf("item-%d", i), "running", "1.23s"}
	}

	tbl.SetRows(rows)

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.String()
	}
}

func Benchmark_Lipgloss__Table_WithColumnStyles(b *testing.B) {
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

func Benchmark__Table_NoChange(b *testing.B) {
	rows := buildLargeRows()

	tbl := New(Config{
		Width:        120,
		Border:       style.NormalBorder(),
		Headers:      largeHeaders,
		ColumnStyles: largeColumnStyles[:11],
	})

	tbl.SetRows(rows)
	_ = tbl.String()

	b.ResetTimer()

	for b.Loop() {
		tbl.SetRows(rows)
		_ = tbl.String()
	}
}

func Benchmark_Lipgloss__Table_NoChange(b *testing.B) {
	tbl := lipglosstable.New().
		Width(120).
		Border(lipgloss.NormalBorder()).
		Headers(largeHeaders...).
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

func Benchmark__Table_SelectionChange(b *testing.B) {
	tbl := New(Config{
		Width:               120,
		Border:              style.NormalBorder(),
		Headers:             largeHeaders,
		ColumnStyles:        largeColumnStyles[:11],
		SelectionBackground: style.Color("#333333"),
	})

	tbl.SetRows(buildLargeRows())
	tbl.Select(0)
	_ = tbl.String()

	b.ResetTimer()

	for b.Loop() {
		tbl.Select((tbl.SelectedIndex() + 1) % 50)
		_ = tbl.String()
	}
}

func Benchmark_Lipgloss__Table_SelectionChange(b *testing.B) {
	tbl := lipglosstable.New().
		Width(120).
		Border(lipgloss.NormalBorder()).
		Headers(largeHeaders...).
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
