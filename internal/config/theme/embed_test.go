package theme

import (
	"bytes"
	"maps"
	"slices"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/yamlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuiltinYAMLInvariants pins the embedded builtin/*.yml as the source of
// truth: every file parses and validates, the set names exactly the 34 expected
// themes with panix present, and every theme survives a YAML round-trip (which
// pins explicit "bold: false" / "pad_y: 0" fidelity).
func TestBuiltinYAMLInvariants(t *testing.T) {
	t.Parallel()

	themes, err := loadEmbeddedThemes()
	require.NoError(t, err)
	assert.Equal(t, expectedBuiltinNames, slices.Sorted(maps.Keys(themes)))
	assert.Contains(t, themes, "panix")

	embedded := builtinThemes()
	require.Len(t, embedded, len(expectedBuiltinNames))

	for name, theme := range embedded {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			require.NoError(t, yamlx.Encode(theme, &buf))

			var decoded Theme
			require.NoError(t, yamlx.Decode(buf.Bytes(), &decoded))
			decoded = decoded.derive()
			assert.Equal(t, theme, decoded, "YAML round-trip equality")
		})
	}
}

// TestBuiltinThemesAreDeepCopies proves every accessor hands out fresh backing
// memory: mutating a returned theme (including pointers and spinner frames) never
// corrupts the embedded cache.
func TestBuiltinThemesAreDeepCopies(t *testing.T) {
	t.Parallel()

	first := builtinThemes()
	first["panix"].Spinner.Frames[0] = "x"
	*first["panix"].Header.Title.Bold = false

	second := builtinThemes()
	assert.Equal(t, "⣾", second["panix"].Spinner.Frames[0], "frames must be cloned")
	assert.Equal(t, new(true), second["panix"].Header.Title.Bold, "pointers must be cloned")

	single, ok := builtinTheme("panix")
	require.True(t, ok)

	single.Spinner.Frames[0] = "y"

	assert.Equal(t, "⣾", defaultTheme().Spinner.Frames[0], "defaultTheme must be a deep copy")

	_, ok = builtinTheme("does-not-exist")
	assert.False(t, ok)
}
