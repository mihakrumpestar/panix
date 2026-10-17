package flow

import (
	"testing"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGradientPairFromColors pins the exact colorful values the flow gradient
// renderer receives, including palette-index resolution through style.Color.
func TestGradientPairFromColors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		from     string
		to       string
		wantFrom string
		wantTo   string
	}{
		{name: "valid hex pair", from: "#01536e", to: "#007da7", wantFrom: "#01536e", wantTo: "#007da7"},
		{
			// Palette indices resolve through the 256-color cube in
			// style.Color: 139 -> #996699, 200 -> #ff00cc.
			name:     "palette index sides resolve through the palette",
			from:     "139",
			to:       "200",
			wantFrom: "#996699",
			wantTo:   "#ff00cc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			from, err := style.ParseColor(tt.from)
			require.NoError(t, err)

			to, err := style.ParseColor(tt.to)
			require.NoError(t, err)

			assert.Equal(t,
				GradientPair{Dark: mustHexColorful(t, tt.wantFrom), Light: mustHexColorful(t, tt.wantTo)},
				GradientPairFromColors(from, to))
		})
	}
}

func mustHexColorful(t *testing.T, hex string) colorful.Color {
	t.Helper()

	color, err := colorful.Hex(hex)
	require.NoError(t, err)

	return color
}
