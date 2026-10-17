package theme

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedBuiltinNames is the golden sorted list of all 34 built-in theme names.
var expectedBuiltinNames = []string{
	"ayu-dark",
	"ayu-light",
	"ayu-mirage",
	"catppuccin-frappe",
	"catppuccin-latte",
	"catppuccin-macchiato",
	"catppuccin-mocha",
	"dracula",
	"everforest-dark",
	"everforest-light",
	"github-dark",
	"github-light",
	"gruvbox-dark",
	"gruvbox-light",
	"kanagawa-dragon",
	"kanagawa-lotus",
	"kanagawa-wave",
	"material-darker",
	"material-oceanic",
	"material-palenight",
	"monokai-classic",
	"nord",
	"one-dark",
	"panix",
	"rose-pine",
	"rose-pine-dawn",
	"rose-pine-moon",
	"solarized-dark",
	"solarized-light",
	"synthwave-84",
	"tokyonight-day",
	"tokyonight-moon",
	"tokyonight-night",
	"tokyonight-storm",
}

// sortedBuiltinNamesPlus returns the golden built-in names plus extra user theme
// names, sorted.
func sortedBuiltinNamesPlus(extra ...string) []string {
	names := append(slices.Clone(expectedBuiltinNames), extra...)
	slices.Sort(names)

	return names
}

func TestBuiltinThemeNames(t *testing.T) {
	t.Parallel()

	names := NewRegistry().Names()

	assert.Equal(t, expectedBuiltinNames, names)

	for idx := 1; idx < len(names); idx++ {
		assert.Less(t, names[idx-1], names[idx], "names must be unique and sorted: %q vs %q", names[idx-1], names[idx])
	}
}

func TestRegistryUserThemeShadowsBuiltin(t *testing.T) {
	t.Parallel()

	path := writeThemesFile(t, `
themes:
  panix:
    status:
      ok:
        fg: "#123456"
  custom:
    chars:
      phase_arrow: "=>"
`)

	reg := NewRegistry()
	require.NoError(t, reg.LoadThemesFiles(path, ""))

	assert.Equal(t, sortedBuiltinNamesPlus("custom"), reg.Names(), "names must be sorted and deduped")

	scheme, ok := reg.Build("panix")
	require.True(t, ok)
	assert.Equal(t, style.Color("#123456"), scheme.Status.OK.Style.GetForeground(), "user theme must shadow the built-in")
	assert.Equal(t, "builtin", reg.Source("rose-pine"))
	assert.Equal(t, path, reg.Source("panix"), "shadowing theme provenance is the user file")
	assert.Equal(t, path, reg.Source("custom"))
}

func TestRegistryBuildUnknownName(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	scheme, ok := reg.Build("does-not-exist")

	assert.False(t, ok)
	assert.Nil(t, scheme)
	assert.Empty(t, reg.Source("does-not-exist"))
}

func TestRegistryBuildPanixMatchesDefault(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	scheme, ok := reg.Build("panix")
	require.True(t, ok)

	assert.Equal(t, DefaultTheme(), scheme)
}

func TestRegistryBuildIsAlwaysFresh(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	first, ok := reg.Build("panix")
	require.True(t, ok)

	second, ok := reg.Build("panix")
	require.True(t, ok)

	assert.NotSame(t, first, second)

	// Deep-mutate the first instance: the spinner frame slice and pointer fields
	// must be independent copies.
	first.Spinner.Frames[0] = "X"

	require.NotNil(t, first.PhaseStatus.Pill.Bold)
	*first.PhaseStatus.Pill.Bold = false

	third, ok := reg.Build("panix")
	require.True(t, ok)

	assert.Equal(t, "⣾", third.Spinner.Frames[0])
	assert.Equal(t, "⣾", second.Spinner.Frames[0], "earlier instances must be independent")
	assert.True(t, derefBool(third.PhaseStatus.Pill.Bold))
}

func TestRegistryBuildPartialMergesOntoDefault(t *testing.T) {
	t.Parallel()

	path := writeThemesFile(t, `
themes:
  custom:
    status:
      ok:
        fg: "#31748f"
`)

	reg := NewRegistry()
	require.NoError(t, reg.LoadThemesFiles(path, ""))

	scheme, ok := reg.Build("custom")
	require.True(t, ok)

	assert.Equal(t, style.Color("#31748f"), scheme.Status.OK.Style.GetForeground(), "set color must win")
	assert.Equal(t, style.Color("#FFB86C"), scheme.Status.Warning.Style.GetForeground(), "unset colors must inherit the default theme")
	assert.Equal(t, "✅", scheme.Status.Icons.OK, "glyphs must inherit the default theme")
	assert.Equal(t, DefaultTheme().Spinner, scheme.Spinner, "spinner must inherit the default theme")
}

func TestRegistryLoadThemesFileInvalidFails(t *testing.T) {
	t.Parallel()

	path := writeThemesFile(t, "themes: [this is: not a map\n")

	reg := NewRegistry()
	err := reg.LoadThemesFiles(path, "")

	require.Error(t, err, "an invalid themes file is a hard error")
	assert.Contains(t, err.Error(), strconv.Quote(path))
	assert.Equal(t, NewRegistry().Names(), reg.Names(), "a failed load must not extend the registry")
}

func TestRegistryLoadThemesFileMissingFails(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "nope.yml")

	reg := NewRegistry()
	err := reg.LoadThemesFiles(missing, "")

	require.Error(t, err, "a missing themes file is a hard error")
	assert.Contains(t, err.Error(), strconv.Quote(missing))
	assert.Equal(t, NewRegistry().Names(), reg.Names(), "a failed load must not extend the registry")
}

func writeThemesFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), ThemesFileName)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}
