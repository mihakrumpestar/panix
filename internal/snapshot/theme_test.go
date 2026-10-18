package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

const customThemeYAML = `
themes:
  custom:
    status:
      ok:
        fg: "#31748f"
    chars:
      phase_arrow: "=>"
    spinner:
      frames: ["a", "b"]
      interval: 10ms
`

// newSnapshotConfig builds a snapshot-ready config carrying only the theme
// context a snapshot needs: flags.tui.theme and flags.tui.themes_file.
func newSnapshotConfig(themeName, themesFile string) *config.Config {
	conf := &config.Config{}
	conf.Flags.Tui.Theme = themeName
	conf.Flags.Tui.ThemesFile = themesFile
	conf.Snapshot.StartTime = time.Unix(1700000000, 0).UTC()
	conf.Snapshot.SnapshotTime = time.Unix(1700000100, 0).UTC()
	conf.Snapshot.Reason = config.SnapshotReasonManual

	return conf
}

// writeSnapshot writes conf and returns the produced snapshot path.
func writeSnapshot(t *testing.T, dir string, conf *config.Config) string {
	t.Helper()

	require.NoError(t, Write(dir, conf))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	return filepath.Join(dir, entries[0].Name())
}

// isolateReadEnvironment moves the working directory and the user config
// directory to fresh temp locations. Not parallel-safe.
func isolateReadEnvironment(t *testing.T) (string, string) {
	t.Helper()

	workDir := t.TempDir()
	configHome := t.TempDir()
	t.Chdir(workDir)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", t.TempDir()) // os.UserConfigDir falls back to HOME on darwin

	return workDir, configHome
}

// requireBuiltinDefault asserts the config resolves the built-in panix theme.
func requireBuiltinDefault(t *testing.T, conf *config.Config) {
	t.Helper()

	assert.Equal(t, "panix", conf.Theme.Name)

	resolved, ok := conf.ResolveTheme()
	require.True(t, ok)
	assert.Equal(t, theme.DefaultTheme(), resolved)
}

// TestReadLegacySnapshotWithEmptyThemeName pins that snapshot flags written
// before flags.tui.theme existed still open and render the built-in theme. Not
// parallel: t.Chdir/t.Setenv.
//
//nolint:paralleltest // t.Chdir/t.Setenv are not parallel-safe
func TestReadLegacySnapshotWithEmptyThemeName(t *testing.T) {
	path := writeSnapshot(t, t.TempDir(), newSnapshotConfig("", ""))

	isolateReadEnvironment(t)

	conf, err := Read(path)
	require.NoError(t, err, "an empty theme name must not fail snapshot loading")
	requireBuiltinDefault(t, conf)
}

// TestReadSnapshotWithUnknownThemeFallsBackToDefault pins that an unknown theme
// name never fails snapshot loading. Not parallel: t.Chdir/t.Setenv.
//
//nolint:paralleltest // t.Chdir/t.Setenv are not parallel-safe
func TestReadSnapshotWithUnknownThemeFallsBackToDefault(t *testing.T) {
	path := writeSnapshot(t, t.TempDir(), newSnapshotConfig("does-not-exist", ""))

	isolateReadEnvironment(t)

	conf, err := Read(path)
	require.NoError(t, err, "an unknown theme name must not fail snapshot loading")
	requireBuiltinDefault(t, conf)
}

// TestReadSnapshotSwallowsBrokenThemesFiles pins that a broken, unreadable or
// missing themes file never fails snapshot loading. Not parallel:
// t.Chdir/t.Setenv.
//
//nolint:funlen,paralleltest // table keeps every theme-failure source in one place; t.Chdir/t.Setenv are not parallel-safe
func TestReadSnapshotSwallowsBrokenThemesFiles(t *testing.T) {
	tests := []struct {
		name          string
		themesFile    func(t *testing.T, workDir string) string
		installGlobal bool
	}{
		{
			name: "invalid local themes file",
			themesFile: func(t *testing.T, workDir string) string {
				t.Helper()

				path := filepath.Join(workDir, theme.ThemesFileName)
				require.NoError(t, os.WriteFile(path, []byte("themes: [broken\n"), 0o600))

				return ""
			},
		},
		{
			name: "invalid global themes file",
			themesFile: func(t *testing.T, workDir string) string {
				t.Helper()

				return ""
			},
			installGlobal: true,
		},
		{
			name: "missing explicit themes file",
			themesFile: func(t *testing.T, workDir string) string {
				t.Helper()

				return filepath.Join(workDir, "missing-themes.yml")
			},
		},
		{
			name: "absent themes files",
			themesFile: func(t *testing.T, workDir string) string {
				t.Helper()

				return ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshotDir := t.TempDir()

			workDir, configHome := isolateReadEnvironment(t)

			themesFile := tt.themesFile(t, workDir)

			if tt.installGlobal {
				globalPath := filepath.Join(configHome, "panix", theme.ThemesFileName)
				require.NoError(t, os.MkdirAll(filepath.Dir(globalPath), 0o750))
				require.NoError(t, os.WriteFile(globalPath, []byte("themes: [broken\n"), 0o600))
			}

			path := writeSnapshot(t, snapshotDir, newSnapshotConfig("custom", themesFile))

			conf, err := Read(path)
			require.NoError(t, err, "a themes file problem must not fail snapshot loading")
			requireBuiltinDefault(t, conf)
		})
	}
}

// TestReadSnapshotResolvesCustomThemeFromThemesFile pins that a snapshot naming a
// custom theme still resolves it when the themes file is available. Not parallel:
// t.Chdir/t.Setenv.
//
//nolint:paralleltest // t.Chdir/t.Setenv are not parallel-safe
func TestReadSnapshotResolvesCustomThemeFromThemesFile(t *testing.T) {
	workDir, _ := isolateReadEnvironment(t)

	require.NoError(t, os.WriteFile(filepath.Join(workDir, theme.ThemesFileName), []byte(customThemeYAML), 0o600))

	path := writeSnapshot(t, t.TempDir(), newSnapshotConfig("custom", ""))

	conf, err := Read(path)
	require.NoError(t, err)
	assert.Equal(t, "custom", conf.Theme.Name)

	resolved, ok := conf.ResolveTheme()
	require.True(t, ok)
	assert.Equal(t, style.Color("#31748f"), resolved.Status.OK.Style.GetForeground())
	assert.Equal(t, []byte("=>"), resolved.Chars.PhaseArrowBytes)
}

// TestWriteDoesNotPersistTheme pins the snapshot shape: the theme name lives only
// in flags.tui.theme and no theme object is serialized under snapshot.
func TestWriteDoesNotPersistTheme(t *testing.T) {
	t.Parallel()

	path := writeSnapshot(t, t.TempDir(), newSnapshotConfig("custom", ""))

	data, err := os.ReadFile(path) //nolint:gosec // path is the test-produced snapshot
	require.NoError(t, err)

	var raw struct {
		Snapshot map[string]json.RawMessage `json:"snapshot"`
		Flags    struct {
			Tui struct {
				Theme string `json:"theme"`
			} `json:"tui"`
		} `json:"flags"`
	}
	require.NoError(t, json.Unmarshal(data, &raw))

	require.NotContains(t, raw.Snapshot, "theme", "snapshots must not carry a theme object")
	assert.Equal(t, "custom", raw.Flags.Tui.Theme, "the theme name must remain in flags.tui.theme")
}

// TestReadTrackedExampleSnapshot pins that the tracked example snapshot keeps
// opening and renders the built-in default.
func TestReadTrackedExampleSnapshot(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "examples", "panix-snapshot-1786805167-1786805230-manual.json")

	conf, err := Read(path)
	require.NoError(t, err, "the tracked example snapshot must keep opening")
	requireBuiltinDefault(t, conf)
}
