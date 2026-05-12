package table

import (
	"fmt"
	"strconv"
	"testing"

	"charm.land/lipgloss/v2"
	lipglosstable "charm.land/lipgloss/v2/table"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

var defaultColumnStyles = []style.Style{{}, {}, {}}

var largeColumnStyles = []style.Style{
	{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
}

var largeHeaders = [][]byte{
	[]byte("#"), []byte("Icon"), []byte("Flake"), []byte("Config"), []byte("Machine"),
	[]byte("Arch"), []byte("Status"), []byte("Gen"), []byte("Date"), []byte("NixOS"), []byte("Kernel"),
}

func buildLargeRows() [][][]byte {
	rows := make([][][]byte, 50)
	for i := range 50 {
		rows[i] = [][]byte{
			[]byte(strconv.Itoa(i + 1)), []byte("✅"),
			fmt.Appendf(nil, "flake-%d", i%5), fmt.Appendf(nil, "config-%d", i%10),
			fmt.Appendf(nil, "machine-%d", i), []byte("x86_64"), []byte("done"), []byte("42"),
			[]byte("2024-01-01"), []byte("24.05"), []byte("6.1.0"),
		}
	}

	return rows
}

// --- Small (3 cols, 10 rows) ---

func Benchmark__Table_Small(b *testing.B) {
	tbl := New(Config{
		Width:        60,
		Border:       style.NormalBorder(),
		Headers:      [][]byte{[]byte("Name"), []byte("Status"), []byte("Time")},
		ColumnStyles: defaultColumnStyles[:3],
	})

	rows := make([][][]byte, 10)
	for i := range 10 {
		rows[i] = [][]byte{fmt.Appendf(nil, "item-%d", i), []byte("done"), []byte("1.23s")}
	}

	tbl.SetRows(rows)

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.Render()
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
	_ = tbl.Render()

	b.ResetTimer()

	for b.Loop() {
		tbl.SetRows(rows)
		_ = tbl.Render()
	}
}

func Benchmark__Table_Large_FreshRows(b *testing.B) {
	rowsA := buildLargeRows()

	rowsB := make([][][]byte, len(rowsA))
	for i, row := range rowsA {
		rowsB[i] = make([][]byte, len(row))
		for j, cell := range row {
			rowsB[i][j] = make([]byte, len(cell))
			copy(rowsB[i][j], cell)
		}
	}

	rowsB[0][6] = []byte("running")

	tbl := New(Config{
		Width:        120,
		Border:       style.NormalBorder(),
		Headers:      largeHeaders,
		ColumnStyles: largeColumnStyles[:11],
	})

	tbl.SetRows(rowsA)
	_ = tbl.Render()

	b.ResetTimer()

	for b.Loop() {
		tbl.SetRows(rowsA)
		_ = tbl.Render()
		tbl.SetRows(rowsB)
		_ = tbl.Render()
	}
}

func Benchmark_Lipgloss__Table_Large_FreshRows(b *testing.B) {
	rowsA := buildLargeRows()

	rowsB := make([][][]byte, len(rowsA))
	for i, row := range rowsA {
		rowsB[i] = make([][]byte, len(row))
		for j, cell := range row {
			rowsB[i][j] = make([]byte, len(cell))
			copy(rowsB[i][j], cell)
		}
	}

	rowsB[0][6] = []byte("running")

	tbl := lipglosstable.New().
		Width(120).
		Border(lipgloss.NormalBorder()).
		Headers("#", "Icon", "Flake", "Config", "Machine", "Arch", "Status", "Gen", "Date", "NixOS", "Kernel").
		StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })

	for _, row := range rowsA {
		tbl.Row(string(row[0]), string(row[1]), string(row[2]), string(row[3]), string(row[4]),
			string(row[5]), string(row[6]), string(row[7]), string(row[8]), string(row[9]), string(row[10]))
	}

	_ = tbl.String()

	b.ResetTimer()

	for b.Loop() {
		tbl = lipglosstable.New().
			Width(120).
			Border(lipgloss.NormalBorder()).
			Headers("#", "Icon", "Flake", "Config", "Machine", "Arch", "Status", "Gen", "Date", "NixOS", "Kernel").
			StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })
		for _, row := range rowsA {
			tbl.Row(string(row[0]), string(row[1]), string(row[2]), string(row[3]), string(row[4]),
				string(row[5]), string(row[6]), string(row[7]), string(row[8]), string(row[9]), string(row[10]))
		}

		_ = tbl.String()

		tbl = lipglosstable.New().
			Width(120).
			Border(lipgloss.NormalBorder()).
			Headers("#", "Icon", "Flake", "Config", "Machine", "Arch", "Status", "Gen", "Date", "NixOS", "Kernel").
			StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })
		for _, row := range rowsB {
			tbl.Row(string(row[0]), string(row[1]), string(row[2]), string(row[3]), string(row[4]),
				string(row[5]), string(row[6]), string(row[7]), string(row[8]), string(row[9]), string(row[10]))
		}

		_ = tbl.String()
	}
}

func Benchmark_Lipgloss__Table_Large(b *testing.B) {
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

func Benchmark__Table_LongContent(b *testing.B) {
	longCell := []byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")

	tbl := New(Config{
		Width:   60,
		Border:  style.NormalBorder(),
		Headers: [][]byte{[]byte("A"), []byte("B")},
	})
	tbl.SetRows([][][]byte{{longCell, longCell}})

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.Render()
	}
}

func Benchmark_Lipgloss__Table_LongContent(b *testing.B) {
	longCell := "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

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
		Headers:      [][]byte{[]byte("Name"), []byte("Status"), []byte("Time")},
		ColumnStyles: styledCols,
	})

	rows := make([][][]byte, 50)
	for i := range 50 {
		rows[i] = [][]byte{fmt.Appendf(nil, "item-%d", i), []byte("running"), []byte("1.23s")}
	}

	tbl.SetRows(rows)

	b.ResetTimer()

	for b.Loop() {
		_ = tbl.Render()
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
	_ = tbl.Render()

	b.ResetTimer()

	for b.Loop() {
		tbl.SetRows(rows)
		_ = tbl.Render()
	}
}

func Benchmark_Lipgloss__Table_NoChange(b *testing.B) {
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
	_ = tbl.Render()

	b.ResetTimer()

	for b.Loop() {
		tbl.Select((tbl.SelectedIndex() + 1) % 50)
		_ = tbl.Render()
	}
}

func Benchmark_Lipgloss__Table_SelectionChange(b *testing.B) {
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
