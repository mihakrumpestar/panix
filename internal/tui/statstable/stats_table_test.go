package statstable

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/stringbyte"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

// testFleet builds a fleet with machineCount machines and no machine infos:
// MachineCount only walks the tree.
func testFleet(machineCount int) *fleet.Fleet {
	flakesMap := atomicorderedmap.New[string, *flake.Flake]()

	flakeObj := &flake.Flake{
		Installables: atomicorderedmap.New[string, *atomicorderedmap.AtomicOrderedMap[string, *installable.Installable]](),
	}

	out := &installable.Installable{Machines: atomicorderedmap.New[string, *machine.Machine]()}
	for i := range machineCount {
		out.Machines.Set(fmt.Sprintf("m%d", i), &machine.Machine{})
	}

	attrMap := atomicorderedmap.New[string, *installable.Installable]()
	attrMap.Set("cfg0", out)
	flakeObj.Installables.Set("nixosConfigurations", attrMap)
	flakesMap.Set("flake0", flakeObj)

	return &fleet.Fleet{Flakes: flakesMap}
}

func setMachineInfos(f *fleet.Fleet) {
	f.CacheMachineInfos = []fleet.MachineInfo{
		{
			State:       machine.State{Status: stats.Running},
			MachineName: stringbyte.StringByte("m0"),
			FlakeName:   stringbyte.StringByte("flake0"),
			OutputType:  stringbyte.StringByte("nixosConfigurations"),
			OutputName:  stringbyte.StringByte("cfg0"),
		},
		{
			State:       machine.State{Status: stats.Failed},
			MachineName: stringbyte.StringByte("m1"),
			FlakeName:   stringbyte.StringByte("flake0"),
			OutputType:  stringbyte.StringByte("nixosConfigurations"),
			OutputName:  stringbyte.StringByte("cfg0"),
		},
	}
}

func themeWithIcons(ok, failed, running string) *theme.Theme {
	scheme := theme.DefaultTheme().Merge(theme.Theme{
		Status: theme.ThemeStatus{
			Icons: theme.ThemeStatusIcons{OK: ok, Failed: failed, Running: running},
		},
	})

	return &scheme
}

// TestTableConfig_StatusColumnWidth proves the status column is sized from the
// widest status icon: 1 cell for a 1-cell-only theme, 2 for a mixed 1/2-cell set.
func TestTableConfig_StatusColumnWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ok      string
		failed  string
		running string
		want    int
	}{
		{name: "one cell only", ok: "✓", failed: "✗", running: "↻", want: 1},
		{name: "mixed one and two cell", ok: "✓", failed: "✗", running: "🔄", want: 2},
		{name: "two cell only", ok: "✅", failed: "❌", running: "🔄", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scheme := themeWithIcons(tt.ok, tt.failed, tt.running)
			st := New(testFleet(2), scheme)

			cfg := st.tableConfig(scheme)

			require.Len(t, cfg.ColumnStyles, 12)
			assert.Equal(t, tt.want, cfg.ColumnStyles[1].GetWidth(),
				"status column must fit the widest status icon")
		})
	}
}

// TestTableConfig_StatusColumnWidthUsesMeasuredWidth pins that applying the theme
// after the width probe uses the measured width, not the static table.
//
//nolint:paralleltest // mutates the process-global measured width overrides
func TestTableConfig_StatusColumnWidthUsesMeasuredWidth(t *testing.T) {
	t.Cleanup(style.ResetMeasuredGlyphWidths)

	const glyph = "\U0001FA9C" // font gap glyph: static table says two cells

	require.Equal(t, 2, style.CellWidth([]byte(glyph)), "static table baseline")

	scheme := themeWithIcons("✓", "✗", glyph)
	st := New(testFleet(2), scheme)

	style.InstallMeasuredGlyphWidths(map[string]int{glyph: 1})
	st.ApplyTheme(scheme)

	cfg := st.tableConfig(scheme)

	require.Len(t, cfg.ColumnStyles, 12)
	assert.Equal(t, 1, cfg.ColumnStyles[1].GetWidth(),
		"status column must use the measured icon width")
}

// TestRender_NoRowExceedsWidth proves no row overflows the table width for both a
// 1-cell-only and a mixed 1/2-cell icon set: the status column grows to the widest
// icon instead of truncating it or shifting the row separator.
func TestRender_NoRowExceedsWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ok      string
		failed  string
		running string
	}{
		{name: "one cell only", ok: "✓", failed: "✗", running: "↻"},
		{name: "mixed one and two cell", ok: "✓", failed: "✗", running: "🔄"},
	}

	const width = 200

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scheme := themeWithIcons(tt.ok, tt.failed, tt.running)
			f := testFleet(2)
			setMachineInfos(f)

			st := New(f, scheme)
			out := st.Render(width)
			require.NotZero(t, out.Len(), "rendered table must not be empty")

			for i := range out.Len() {
				assert.LessOrEqual(t, style.CellWidth(out.Line(i)), width,
					"row %d must not exceed the table width", i)
			}
		})
	}
}
