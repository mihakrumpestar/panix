package config

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

// isolateThemesFile points the theme lookup (working directory and
// os.UserConfigDir) at fresh temp dirs and optionally installs a user themes
// file, returning its path ("" when none was created). Not parallel-safe.
func isolateThemesFile(t *testing.T, content string, unreadable bool) string {
	t.Helper()

	workDir := t.TempDir()
	t.Chdir(workDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir()) // os.UserConfigDir falls back to HOME on darwin

	if content == "" && !unreadable {
		return ""
	}

	path := filepath.Join(workDir, theme.ThemesFileName)

	mode := os.FileMode(0o600)
	if unreadable {
		mode = 0
	}

	require.NoError(t, os.WriteFile(path, []byte(content), mode))

	return path
}

// TestResolveTheme is the resolution table for resolveTheme: theme problems
// (unreadable or invalid themes files, invalid values, unknown names) are hard
// config errors that fail at eval. Not parallel: it drives the lookup through the
// working directory and os.UserConfigDir.
//
//nolint:funlen,paralleltest // t.Chdir/t.Setenv are not parallel-safe
func TestResolveTheme(t *testing.T) {
	tests := []struct {
		name           string
		requested      string
		themesFile     string
		unreadable     bool
		wantName       string
		wantUserSource bool
		wantDefault    bool
		wantErrSubstrs []string
		wantFileErr    bool
	}{
		{
			name:           "empty name is an error",
			wantErrSubstrs: []string{"theme name must not be empty"},
		},
		{
			name:      "valid built-in name resolves",
			requested: "rose-pine",
			wantName:  "rose-pine",
		},
		{
			name:           "valid partial theme passes",
			requested:      "custom",
			themesFile:     "themes:\n  custom:\n    chars:\n      phase_arrow: \"=>\"\n",
			wantName:       "custom",
			wantUserSource: true,
		},
		{
			name:           "user theme becomes active",
			requested:      "custom",
			themesFile:     "themes:\n  custom:\n    status:\n      ok:\n        fg: \"#31748f\"\n",
			wantName:       "custom",
			wantUserSource: true,
		},
		{
			name:           "user theme shadows built-in without error",
			requested:      "panix",
			themesFile:     "themes:\n  panix:\n    status:\n      ok:\n        fg: \"#31748f\"\n",
			wantName:       "panix",
			wantUserSource: true,
		},
		{
			name:           "unknown name fails naming the theme and the available names",
			requested:      "does-not-exist",
			wantErrSubstrs: []string{`unknown theme "does-not-exist"`, "rose-pine", "panix"},
		},
		{
			name:           "unknown name error lists user themes too",
			requested:      "does-not-exist",
			themesFile:     "themes:\n  custom: {}\n",
			wantErrSubstrs: []string{`unknown theme "does-not-exist"`, "custom"},
		},
		{
			name:           "invalid YAML in themes file fails",
			themesFile:     "themes: [broken\n",
			wantErrSubstrs: []string{"themes file"},
			wantFileErr:    true,
		},
		{
			name:           "unreadable themes file fails",
			unreadable:     true,
			wantErrSubstrs: []string{"themes file"},
			wantFileErr:    true,
		},
		{
			name:           "unknown YAML key fails like the config file does",
			themesFile:     "themes:\n  custom:\n    nope: 1\n",
			wantErrSubstrs: []string{"themes file", "nope"},
			wantFileErr:    true,
		},
		{
			name:           "bad color fails",
			themesFile:     "themes:\n  custom:\n    status:\n      ok:\n        fg: \"#zzzzzz\"\n",
			wantErrSubstrs: []string{`theme "custom"`, "status.ok.fg", `"#zzzzzz"`},
			wantFileErr:    true,
		},
		{
			name:           "bad gradient pair side fails",
			themesFile:     "themes:\n  custom:\n    phase_status:\n      running: [\"#zzzzzz\", \"#007da7\"]\n",
			wantErrSubstrs: []string{`theme "custom"`, "phase_status.running[0]", `"#zzzzzz"`},
			wantFileErr:    true,
		},
		{
			name:           "bad duration fails",
			themesFile:     "themes:\n  custom:\n    spinner:\n      interval: banana\n",
			wantErrSubstrs: []string{"banana"},
			wantFileErr:    true,
		},
		{
			name:           "unknown border preset fails",
			themesFile:     "themes:\n  custom:\n    border:\n      table: squiggly\n",
			wantErrSubstrs: []string{`theme "custom"`, "border.table", "squiggly"},
			wantFileErr:    true,
		},
		{
			name:           "empty spinner frame fails",
			themesFile:     "themes:\n  custom:\n    spinner:\n      frames: [\"|\", \"\"]\n",
			wantErrSubstrs: []string{`theme "custom"`, "spinner.frames[1]", "must not be empty"},
			wantFileErr:    true,
		},
		{
			name:           "mixed-width spinner frames fail",
			themesFile:     "themes:\n  custom:\n    spinner:\n      frames: [\"--\", \"|\"]\n",
			wantErrSubstrs: []string{`theme "custom"`, "spinner.frames[1]", "width"},
			wantFileErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unreadable && os.Geteuid() == 0 {
				t.Skip("root can read mode-000 files")
			}

			themesPath := isolateThemesFile(t, tt.themesFile, tt.unreadable)

			conf := &Config{}
			conf.Flags.Tui.Theme = tt.requested

			err := conf.resolveTheme()

			assertion := assert.New(t)

			if len(tt.wantErrSubstrs) > 0 {
				require.Error(t, err)

				for _, substr := range tt.wantErrSubstrs {
					assertion.Contains(err.Error(), substr)
				}

				if tt.wantFileErr {
					assertion.Contains(err.Error(), strconv.Quote(themesPath), "the error must name the themes file path")
				}

				_, ok := conf.ResolveTheme()
				assertion.False(ok, "a failed resolution must not fill the runtime state")

				return
			}

			require.NoError(t, err)
			assertion.Equal(tt.wantName, conf.Theme.Name, "resolved theme name")
			assertion.NotNil(conf.Theme.Registry, "theme registry must be exposed")

			wantSource := theme.SourceBuiltin
			if tt.wantUserSource {
				wantSource = themesPath
			}

			assertion.Equal(wantSource, conf.Theme.Registry.Source(tt.wantName), "theme provenance")

			resolved, ok := conf.ResolveTheme()
			require.True(t, ok)

			if tt.wantDefault {
				assertion.Equal(theme.DefaultTheme(), resolved,
					"default inputs must reproduce the default scheme")
			} else {
				assertion.NotEqual(theme.DefaultTheme(), resolved,
					"requested theme must be active")
			}

			expected, ok := conf.Theme.Registry.Build(tt.wantName)
			require.True(t, ok)
			assertion.Equal(expected, resolved, "config wiring must delegate to the registry unchanged")
		})
	}
}

// TestThemeResolutionIsUnified pins the DRY contract: applyConfigDefaults
// (initial load) and PostUnmarshalInit (snapshot rehydration) must produce the
// same scheme and name for the same merged flags. Not parallel:
// applyConfigDefaults initializes the global logger.
//
//nolint:paralleltest // applyConfigDefaults initializes the global logger and isolateThemesFile is not parallel-safe
func TestThemeResolutionIsUnified(t *testing.T) {
	tests := []struct {
		name        string
		configTheme string
		cliTheme    string
		wantName    string
	}{
		{"explicit cli theme beats config theme", "rose-pine-dawn", "rose-pine", "rose-pine"},
		{"explicit cli theme fills empty config", "", "rose-pine", "rose-pine"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateThemesFile(t, "", false)

			conf := &Config{}
			conf.Flags.Tui.Theme = tt.configTheme

			parsed := flags.Flags{}
			parsed.Tui.Theme = tt.cliTheme
			parsed.Output = flags.OutputModeTui // keep logging side effects minimal

			explicit := map[string]bool{}
			if tt.cliTheme != "" {
				explicit["tui.theme"] = true
			}

			require.NoError(t, applyConfigDefaults(conf, parsed, explicit))
			assert.Equal(t, tt.wantName, conf.Theme.Name)

			reh := &Config{Flags: conf.Flags}
			require.NoError(t, reh.PostUnmarshalInit())

			confTheme, ok := conf.ResolveTheme()
			require.True(t, ok)

			rehTheme, ok := reh.ResolveTheme()
			require.True(t, ok)

			assertion := assert.New(t)
			assertion.Equal(confTheme, rehTheme, "both entry points must derive the same scheme")
			assertion.Equal(conf.Theme.Name, reh.Theme.Name, "both entry points must resolve the same name")
		})
	}
}

// TestThemeResolutionErrorsAreUnified proves both entry points fail with the same
// error for the same broken themes file: theme problems are hard config errors
// everywhere. Not parallel: t.Chdir/t.Setenv are not parallel-safe.
//
//nolint:paralleltest // t.Chdir/t.Setenv are not parallel-safe
func TestThemeResolutionErrorsAreUnified(t *testing.T) {
	isolateThemesFile(t, "themes:\n  custom:\n    status:\n      ok:\n        fg: \"#zzzzzz\"\n", false)

	conf := &Config{}
	conf.Flags.Tui.Theme = "custom"

	parsed := flags.Flags{}
	parsed.Output = flags.OutputModeTui

	loadErr := applyConfigDefaults(conf, parsed, map[string]bool{})
	require.Error(t, loadErr, "initial load must fail at eval")
	assert.Contains(t, loadErr.Error(), "status.ok.fg")

	reh := &Config{Flags: conf.Flags}
	rehydrateErr := reh.PostUnmarshalInit()
	require.Error(t, rehydrateErr, "snapshot rehydration must fail at eval")
	assert.Equal(t, loadErr.Error(), rehydrateErr.Error(), "both entry points must report the same error")
}

// TestResolveThemeMergesGlobalAndLocal proves the layered themes file behavior
// through the config wiring: both files load and merge, local wins field by field,
// unset local fields inherit from global and merged provenance is the local file.
// Not parallel: t.Chdir/t.Setenv are not parallel-safe.
//
//nolint:paralleltest // t.Chdir/t.Setenv are not parallel-safe
func TestResolveThemeMergesGlobalAndLocal(t *testing.T) {
	localPath := isolateThemesFile(t, `
themes:
  custom:
    status:
      ok:
        fg: "#31748f"
`, false)

	configDir, err := os.UserConfigDir()
	require.NoError(t, err)

	globalPath := filepath.Join(configDir, "panix", theme.ThemesFileName)
	require.NoError(t, os.MkdirAll(filepath.Dir(globalPath), 0o750))
	require.NoError(t, os.WriteFile(globalPath, []byte(`
themes:
  custom:
    status:
      warning:
        fg: "#c4a7e7"
    chars:
      phase_arrow: "=>"
`), 0o600))

	conf := &Config{}
	conf.Flags.Tui.Theme = "custom"

	require.NoError(t, conf.resolveTheme())

	assert.Equal(t, "custom", conf.Theme.Name)

	theme, ok := conf.ResolveTheme()
	require.True(t, ok)

	assert.Equal(t, style.Color("#31748f"), theme.Status.OK.Style.GetForeground(), "local-set field must win")
	assert.Equal(t, style.Color("#c4a7e7"), theme.Status.Warning.Style.GetForeground(), "local-unset field must inherit from the global theme")
	assert.Equal(t, "=>", theme.Chars.PhaseArrow, "local-unset field must inherit from the global theme")
	assert.Equal(t, localPath, conf.Theme.Registry.Source("custom"), "merged theme provenance is the highest precedence contributor")
}

// TestResolveThemeBrokenGlobalEvenWhenOverriddenFails proves garbage anywhere is
// an error even when the local file overrides the field: every parsed theme is
// validated before the merge. Not parallel: t.Chdir/t.Setenv are not parallel-safe.
//
//nolint:paralleltest // t.Chdir/t.Setenv are not parallel-safe
func TestResolveThemeBrokenGlobalEvenWhenOverriddenFails(t *testing.T) {
	localPath := isolateThemesFile(t, `
themes:
  custom:
    status:
      ok:
        fg: "#31748f"
`, false)

	configDir, err := os.UserConfigDir()
	require.NoError(t, err)

	globalPath := filepath.Join(configDir, "panix", theme.ThemesFileName)
	require.NoError(t, os.MkdirAll(filepath.Dir(globalPath), 0o750))
	require.NoError(t, os.WriteFile(globalPath, []byte(`
themes:
  custom:
    status:
      ok:
        fg: "#zzzzzz"
`), 0o600))

	conf := &Config{}
	conf.Flags.Tui.Theme = "custom"

	resolveErr := conf.resolveTheme()
	require.Error(t, resolveErr, "a broken theme in the global file must fail even when the local file overrides the field")
	assert.Contains(t, resolveErr.Error(), strconv.Quote(globalPath))
	assert.Contains(t, resolveErr.Error(), "status.ok.fg")
	assert.NotContains(t, resolveErr.Error(), strconv.Quote(localPath))
}

// TestConfigHoldsOnlyRegistryAndName pins the Config shape: only the theme
// registry and the chosen name are stored, and ResolveTheme derives the resolved
// theme from them on use. Not parallel: isolateThemesFile is not parallel-safe.
//
//nolint:paralleltest // isolateThemesFile uses t.Chdir/t.Setenv
func TestConfigHoldsOnlyRegistryAndName(t *testing.T) {
	isolateThemesFile(t, "", false)

	conf := &Config{}
	conf.Flags.Tui.Theme = "rose-pine"
	require.NoError(t, conf.resolveTheme())

	assert.Equal(t, "rose-pine", conf.Theme.Name)
	require.NotNil(t, conf.Theme.Registry)

	expected, ok := conf.Theme.Registry.Build(conf.Theme.Name)
	require.True(t, ok)

	resolved, ok := conf.ResolveTheme()
	require.True(t, ok)
	assert.Equal(t, expected, resolved, "ResolveTheme must build from the registry and name")

	// The resolved theme is derived on use, never cached: changing the name
	// changes what ResolveTheme returns.
	conf.Theme.Name = "panix"

	panix, ok := conf.ResolveTheme()
	require.True(t, ok)
	assert.Equal(t, theme.DefaultTheme(), panix)
	assert.NotEqual(t, resolved, panix)
}

// TestResolveThemeWithoutRegistry asserts ResolveTheme reports ok=false for a
// hand-built Config with no resolved registry.
func TestResolveThemeWithoutRegistry(t *testing.T) {
	t.Parallel()

	conf := &Config{}

	theme, ok := conf.ResolveTheme()
	assert.False(t, ok)
	assert.Nil(t, theme)
}

// configuredThemesFileConfig is a minimal loadable panix config whose flag
// section points at themesPath.
func configuredThemesFileConfig(themesPath string) string {
	return "fleet:\n" +
		"  flakes:\n" +
		"    my-flake:\n" +
		"      url: path:./my-flake\n" +
		"      installables:\n" +
		"        nixosConfigurations:\n" +
		"          my-config:\n" +
		"            machines:\n" +
		"              my-machine:\n" +
		"flags:\n" +
		"  tui:\n" +
		"    theme: custom\n" +
		"    themes_file: " + strconv.Quote(themesPath) + "\n"
}

// loadConfigWithThemesFile calls LoadConfig with the kong defaults a real CLI
// parse would supply, so tests can focus on the themes file wiring.
func loadConfigWithThemesFile(t *testing.T, configPath string) (*Config, error) {
	t.Helper()

	return LoadConfig(flags.Flags{
		WorkflowFlags: flags.WorkflowFlags{
			ConfigFlags: flags.ConfigFlags{Config: configPath},
			Runtime: flags.Runtime{
				DisconnectTimeout:      5 * time.Minute,
				ReconnectTimeout:       10 * time.Minute,
				SSHReachabilityTimeout: 2 * time.Second,
				FlakeValidationTimeout: 60 * time.Second,
			},
		},
	}, map[string]bool{})
}

// TestLoadConfigUsesConfiguredThemesFile proves the tui.themes_file key reaches
// theme resolution end to end: a theme defined only in the configured file
// resolves through LoadConfig. Not parallel: t.Chdir/t.Setenv and the global
// logger are not parallel-safe.
//
//nolint:paralleltest // t.Chdir/t.Setenv are not parallel-safe
func TestLoadConfigUsesConfiguredThemesFile(t *testing.T) {
	workDir := t.TempDir()
	t.Chdir(workDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	themesPath := filepath.Join(workDir, "custom-themes.yml")
	require.NoError(t, os.WriteFile(themesPath, []byte(`
themes:
  custom:
    status:
      ok:
        fg: "#31748f"
`), 0o600))

	configPath := filepath.Join(workDir, "panix.yml")
	require.NoError(t, os.WriteFile(configPath, []byte(configuredThemesFileConfig(themesPath)), 0o600))

	conf, err := loadConfigWithThemesFile(t, configPath)
	require.NoError(t, err)

	assert.Equal(t, "custom", conf.Theme.Name)
	require.NotNil(t, conf.Theme.Registry)
	assert.Equal(t, themesPath, conf.Theme.Registry.Source("custom"))

	resolved, ok := conf.ResolveTheme()
	require.True(t, ok)
	assert.Equal(t, style.Color("#31748f"), resolved.Status.OK.Style.GetForeground())
}

// TestLoadConfigFailsForMissingConfiguredThemesFile proves a configured themes
// file that does not exist is a hard config error naming the path. Not parallel:
// t.Chdir/t.Setenv are not parallel-safe.
//
//nolint:paralleltest // t.Chdir/t.Setenv are not parallel-safe
func TestLoadConfigFailsForMissingConfiguredThemesFile(t *testing.T) {
	workDir := t.TempDir()
	t.Chdir(workDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	missingPath := filepath.Join(workDir, "missing-themes.yml")
	configPath := filepath.Join(workDir, "panix.yml")
	require.NoError(t, os.WriteFile(configPath, []byte(configuredThemesFileConfig(missingPath)), 0o600))

	_, err := loadConfigWithThemesFile(t, configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), strconv.Quote(missingPath), "the error must name the missing path")
	assert.Contains(t, err.Error(), "failed to load user themes")
}
