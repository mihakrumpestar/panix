package theme

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
)

func TestResolveThemesFilesRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		localFile  bool
		localDir   bool
		globalFile bool
		wantLocal  bool
		wantGlobal bool
	}{
		{name: "local and global both returned when both present", localFile: true, globalFile: true, wantLocal: true, wantGlobal: true},
		{name: "global file returned when no local", globalFile: true, wantGlobal: true},
		{name: "nothing found yields two empty paths"},
		{name: "local file returned when no global", localFile: true, wantLocal: true},
		{name: "local directory is ignored", localDir: true, globalFile: true, wantGlobal: true},
		{name: "local directory alone yields two empty paths", localDir: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			workDir := t.TempDir()
			configDir := t.TempDir()

			if tt.localFile {
				require.NoError(t, os.WriteFile(filepath.Join(workDir, ThemesFileName), []byte("themes: {}"), 0o600))
			}

			if tt.localDir {
				require.NoError(t, os.Mkdir(filepath.Join(workDir, ThemesFileName), 0o750))
			}

			if tt.globalFile {
				globalDir := filepath.Join(configDir, "panix")
				require.NoError(t, os.MkdirAll(globalDir, 0o750))
				require.NoError(t, os.WriteFile(filepath.Join(globalDir, ThemesFileName), []byte("themes: {}"), 0o600))
			}

			local, global, err := resolveThemesFiles(workDir, configDir, "")
			require.NoError(t, err)

			wantLocalPath, wantGlobalPath := "", ""

			if tt.wantLocal {
				wantLocalPath = filepath.Join(workDir, ThemesFileName)
			}

			if tt.wantGlobal {
				wantGlobalPath = filepath.Join(configDir, "panix", ThemesFileName)
			}

			assert.Equal(t, wantLocalPath, local)
			assert.Equal(t, wantGlobalPath, global)
		})
	}
}

// TestResolveThemesFilesExplicitPath pins the explicit-path rule: a configured path
// replaces the implicit local slot, must be an existing regular file and never
// falls back to ./panix-themes.yml, while the global slot is unchanged.
//
//nolint:funlen // single resolution table with fixtures
func TestResolveThemesFilesExplicitPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		explicitFile bool
		explicitDir  bool
		implicitFile bool
		globalFile   bool
		wantLocal    bool
		wantGlobal   bool
		wantErr      bool
	}{
		{
			name:         "explicit file wins over the implicit local file",
			explicitFile: true,
			implicitFile: true,
			wantLocal:    true,
		},
		{
			name:         "explicit file is used without an implicit file",
			explicitFile: true,
			wantLocal:    true,
		},
		{
			name:         "explicit file keeps the global candidate unchanged",
			explicitFile: true,
			globalFile:   true,
			wantLocal:    true,
			wantGlobal:   true,
		},
		{
			name:    "missing explicit file is an error",
			wantErr: true,
		},
		{
			name:        "explicit directory is an error",
			explicitDir: true,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			workDir := t.TempDir()
			configDir := t.TempDir()
			explicitPath := filepath.Join(workDir, "custom-themes.yml")

			if tt.explicitFile {
				require.NoError(t, os.WriteFile(explicitPath, []byte("themes: {}"), 0o600))
			}

			if tt.explicitDir {
				require.NoError(t, os.Mkdir(explicitPath, 0o750))
			}

			if tt.implicitFile {
				require.NoError(t, os.WriteFile(filepath.Join(workDir, ThemesFileName), []byte("themes: {}"), 0o600))
			}

			if tt.globalFile {
				globalDir := filepath.Join(configDir, "panix")
				require.NoError(t, os.MkdirAll(globalDir, 0o750))
				require.NoError(t, os.WriteFile(filepath.Join(globalDir, ThemesFileName), []byte("themes: {}"), 0o600))
			}

			local, global, err := resolveThemesFiles(workDir, configDir, explicitPath)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), strconv.Quote(explicitPath), "the error must name the explicit path")
				assert.Empty(t, local)
				assert.Empty(t, global)

				return
			}

			require.NoError(t, err)

			wantLocal := ""
			if tt.wantLocal {
				wantLocal = explicitPath
			}

			wantGlobal := ""
			if tt.wantGlobal {
				wantGlobal = filepath.Join(configDir, "panix", ThemesFileName)
			}

			assert.Equal(t, wantLocal, local)
			assert.Equal(t, wantGlobal, global)
		})
	}
}

//nolint:funlen // large themes file parsing table
func TestParseThemesFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		content        *string
		missingFile    bool
		wantThemes     int
		wantErrSubstrs []string
	}{
		{
			name:       "valid file",
			content:    new("themes:\n  custom:\n    status:\n      ok:\n        fg: \"#31748f\"\n"),
			wantThemes: 1,
		},
		{
			name:       "valid file with empty themes map",
			content:    new("themes: {}\n"),
			wantThemes: 0,
		},
		{
			name:       "valid partial theme",
			content:    new("themes:\n  custom:\n    chars:\n      phase_arrow: \"=>\"\n"),
			wantThemes: 1,
		},
		{
			name:       "empty file yields no themes and no error",
			content:    new(""),
			wantThemes: 0,
		},
		{
			name:           "file with null themes map fails",
			content:        new("themes:\n"),
			wantErrSubstrs: []string{"no themes"},
		},
		{
			name:           "unknown top-level key fails (strict decoding, same as panix.yml)",
			content:        new("other: {}\n"),
			wantErrSubstrs: []string{"other"},
		},
		{
			name:           "invalid YAML fails",
			content:        new("themes: [broken\n"),
			wantErrSubstrs: []string{"themes file"},
		},
		{
			name:           "invalid duration fails",
			content:        new("themes:\n  custom:\n    spinner:\n      interval: banana\n"),
			wantErrSubstrs: []string{"banana"},
		},
		{
			name:           "unknown field fails (strict decoding, same as panix.yml)",
			content:        new("themes:\n  custom:\n    nope: 1\n"),
			wantErrSubstrs: []string{"nope"},
		},
		{
			name:           "missing file fails",
			missingFile:    true,
			wantErrSubstrs: []string{"themes file"},
		},
		{
			name:           "bad color fails",
			content:        new("themes:\n  custom:\n    status:\n      ok:\n        fg: \"#zzzzzz\"\n"),
			wantErrSubstrs: []string{`theme "custom"`, "status.ok.fg", `"#zzzzzz"`},
		},
		{
			name:           "bad background color fails",
			content:        new("themes:\n  custom:\n    table:\n      row:\n        bg: \"999\"\n"),
			wantErrSubstrs: []string{`theme "custom"`, "table.row.bg", `"999"`},
		},
		{
			name:           "bad gradient pair side fails",
			content:        new("themes:\n  custom:\n    phase_status:\n      failed: [\"#5f1414\", \"#zzzzzz\"]\n"),
			wantErrSubstrs: []string{`theme "custom"`, "phase_status.failed[1]", `"#zzzzzz"`},
		},
		{
			name:           "bad notification color fails",
			content:        new("themes:\n  custom:\n    notification:\n      default_fg_color: \"#zzzzzz\"\n"),
			wantErrSubstrs: []string{`theme "custom"`, "notification.default_fg_color", `"#zzzzzz"`},
		},
		{
			name:           "bad entity color fails",
			content:        new("themes:\n  custom:\n    machine:\n      color: \"#zzzzzz\"\n"),
			wantErrSubstrs: []string{`theme "custom"`, "machine.color", `"#zzzzzz"`},
		},
		{
			name:           "unknown border preset fails",
			content:        new("themes:\n  custom:\n    border:\n      overlay: squiggly\n"),
			wantErrSubstrs: []string{`theme "custom"`, "border.overlay", "squiggly"},
		},
		{
			name:           "empty spinner frame fails",
			content:        new("themes:\n  custom:\n    spinner:\n      frames: [\"\", \"|\"]\n"),
			wantErrSubstrs: []string{`theme "custom"`, "spinner.frames[0]", "must not be empty"},
		},
		{
			name:           "spinner interval below the minimum fails",
			content:        new("themes:\n  custom:\n    spinner:\n      interval: 120ns\n"),
			wantErrSubstrs: []string{`theme "custom"`, "spinner.interval", "120ns", "between"},
		},
		{
			name:           "half gradient pair fails",
			content:        new("themes:\n  custom:\n    phase_status:\n      running: [\"#c4a7e7\"]\n"),
			wantErrSubstrs: []string{`theme "custom"`, "phase_status.running[1]", "incomplete"},
		},
		{
			name:           "oversized padding fails",
			content:        new("themes:\n  custom:\n    phase_status:\n      pill:\n        pad_x: 5000\n"),
			wantErrSubstrs: []string{`theme "custom"`, "phase_status.pill.pad_x", "5000", "between 0 and 16"},
		},
		{
			name:           "negative padding fails",
			content:        new("themes:\n  custom:\n    table:\n      row:\n        pad_y: -5\n"),
			wantErrSubstrs: []string{`theme "custom"`, "table.row.pad_y", "-5", "between 0 and 16"},
		},
		{
			name:           "invalid mode fails",
			content:        new("themes:\n  custom:\n    mode: sepia\n"),
			wantErrSubstrs: []string{`theme "custom"`, "mode", "sepia"},
		},
		{
			name:           "control character in a glyph fails",
			content:        new("themes:\n  custom:\n    chars:\n      phase_arrow: \"->\\u001b[31m\"\n"),
			wantErrSubstrs: []string{`theme "custom"`, "chars.phase_arrow", "control characters"},
		},
		{
			name:           "mixed-width spinner frames fail",
			content:        new("themes:\n  custom:\n    spinner:\n      frames: [\"--\", \"|\"]\n"),
			wantErrSubstrs: []string{`theme "custom"`, "spinner.frames[1]", "width"},
		},
		{
			name:       "palette index colors are valid",
			content:    new("themes:\n  custom:\n    status:\n      ok:\n        fg: \"139\"\n"),
			wantThemes: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), ThemesFileName)
			if !tt.missingFile {
				require.NoError(t, os.WriteFile(path, []byte(*tt.content), 0o600))
			}

			themes, err := ParseThemesFile(path)

			if len(tt.wantErrSubstrs) > 0 {
				require.Error(t, err)
				assert.Contains(t, err.Error(), strconv.Quote(path), "the error must name the themes file path")

				for _, substr := range tt.wantErrSubstrs {
					assert.Contains(t, err.Error(), substr)
				}

				assert.Nil(t, themes)

				return
			}

			require.NoError(t, err)
			assert.Len(t, themes, tt.wantThemes)
		})
	}
}

func TestParseThemesFileValues(t *testing.T) {
	t.Parallel()

	path := writeThemesFile(t, `
themes:
  custom:
    status:
      ok:
        fg: "#31748f"
        bold: false
    phase_status:
      running: ["#c4a7e7", "#9ccfd8"]
    spinner:
      frames: ["|", "/"]
      interval: 120ms
    chars:
      phase_arrow: "->"
`)

	themes, err := ParseThemesFile(path)
	require.NoError(t, err)
	require.Contains(t, themes, "custom")

	theme := themes["custom"]

	assert.Equal(t, style.Color("#31748f"), theme.Status.OK.Fg)
	require.NotNil(t, theme.Status.OK.Bold)
	assert.False(t, *theme.Status.OK.Bold, "deliberate false must survive decoding")
	assert.Equal(t, ThemePair{"#c4a7e7", "#9ccfd8"}, theme.PhaseStatus.Running)
	assert.Equal(t, []string{"|", "/"}, theme.Spinner.Frames)
	assert.Equal(t, yamlx.Duration(120000000), theme.Spinner.Interval)
	assert.Equal(t, "->", theme.Chars.PhaseArrow)
}

// TestResolveThemesFilesBestEffortWithoutHome proves theme lookup never fails just
// because the user config directory is unavailable: built-ins and absolute
// explicit paths still work, and the global candidate is simply empty.
//
//nolint:paralleltest // t.Setenv/t.Chdir are not parallel-safe
func TestResolveThemesFilesBestEffortWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	workDir := t.TempDir()
	t.Chdir(workDir)

	local, global, err := ResolveThemesFiles("")
	require.NoError(t, err, "an unavailable user config dir is not fatal")
	assert.Empty(t, global)
	assert.Empty(t, local, "no implicit local file exists")

	reg := NewRegistry()
	require.NoError(t, reg.LoadUserThemes(""))
	assert.Equal(t, NewRegistry().Names(), reg.Names(), "built-in themes remain available")

	explicitPath := filepath.Join(workDir, "custom-themes.yml")
	require.NoError(t, os.WriteFile(explicitPath, []byte("themes:\n  custom: {}\n"), 0o600))

	reg = NewRegistry()
	require.NoError(t, reg.LoadUserThemes(explicitPath), "an absolute explicit path needs no user config dir")
	assert.Contains(t, reg.Names(), "custom")
}

// TestResolveThemesFilesEmptyDirectories pins the pure rule: an empty working or
// config directory disables only the candidate that needs it.
func TestResolveThemesFilesEmptyDirectories(t *testing.T) {
	t.Parallel()

	local, global, err := resolveThemesFiles("", "", "")
	require.NoError(t, err)
	assert.Empty(t, local)
	assert.Empty(t, global)
}

// TestLoadUserThemesIntegration exercises the real resolution paths (working
// directory and os.UserConfigDir). Not parallel: t.Chdir and t.Setenv.
//
//nolint:funlen // integration scenario with several stages
func TestLoadUserThemesIntegration(t *testing.T) {
	workDir := t.TempDir()
	configDir := t.TempDir()
	t.Chdir(workDir)
	t.Setenv("XDG_CONFIG_HOME", configDir)

	reg := NewRegistry()
	require.NoError(t, reg.LoadUserThemes(""), "no themes file: no error")
	assert.Equal(t, NewRegistry().Names(), reg.Names())

	globalPath := filepath.Join(configDir, "panix", ThemesFileName)
	globalContent := `
themes:
  global-theme: {}
  work:
    status:
      warning:
        fg: "#c4a7e7"
    chars:
      phase_arrow: "=>"
`

	require.NoError(t, os.MkdirAll(filepath.Dir(globalPath), 0o750))
	require.NoError(t, os.WriteFile(globalPath, []byte(globalContent), 0o600))

	reg = NewRegistry()
	require.NoError(t, reg.LoadUserThemes(""))
	assert.Equal(t, sortedBuiltinNamesPlus("global-theme", "work"), reg.Names())
	assert.Equal(t, globalPath, reg.Source("global-theme"))

	localPath := filepath.Join(workDir, ThemesFileName)
	localContent := `
themes:
  local-theme: {}
  work:
    status:
      ok:
        fg: "#31748f"
`
	require.NoError(t, os.WriteFile(localPath, []byte(localContent), 0o600))

	reg = NewRegistry()
	require.NoError(t, reg.LoadUserThemes(""))
	assert.Equal(t, sortedBuiltinNamesPlus("global-theme", "local-theme", "work"), reg.Names(),
		"both files load and merge with the local file on top")
	assert.Equal(t, localPath, reg.Source("local-theme"))
	assert.Equal(t, globalPath, reg.Source("global-theme"), "a theme defined only in the global file keeps the global provenance")
	assert.Equal(t, localPath, reg.Source("work"), "a merged theme reports the highest precedence contributor")

	scheme, ok := reg.Build("work")
	require.True(t, ok)
	assert.Equal(t, style.Color("#31748f"), scheme.Status.OK.Style.GetForeground(), "local field must win")
	assert.Equal(t, style.Color("#c4a7e7"), scheme.Status.Warning.Style.GetForeground(), "unset local field must inherit from the global theme")
	assert.Equal(t, "=>", scheme.Chars.PhaseArrow, "unset local field must inherit from the global theme")

	require.NoError(t, os.WriteFile(localPath, []byte("themes: [broken\n"), 0o600))

	reg = NewRegistry()
	loadErr := reg.LoadUserThemes("")
	require.Error(t, loadErr, "a broken local file fails the load")
	assert.Contains(t, loadErr.Error(), strconv.Quote(localPath), "the error must name the broken local file")
	assert.Equal(t, NewRegistry().Names(), reg.Names(), "a failed load must not extend the registry")
}

// TestLoadUserThemesExplicitPath proves an explicit themes file becomes the local
// slot: it wins over the implicit ./panix-themes.yml, merges with the global file
// using local precedence, reports its own provenance and fails hard when missing.
//
//nolint:paralleltest // t.Chdir/t.Setenv are not parallel-safe
func TestLoadUserThemesExplicitPath(t *testing.T) {
	workDir := t.TempDir()
	configDir := t.TempDir()
	t.Chdir(workDir)
	t.Setenv("XDG_CONFIG_HOME", configDir)

	globalPath := filepath.Join(configDir, "panix", ThemesFileName)
	require.NoError(t, os.MkdirAll(filepath.Dir(globalPath), 0o750))
	require.NoError(t, os.WriteFile(globalPath, []byte(`
themes:
  global-only: {}
  work:
    status:
      warning:
        fg: "#c4a7e7"
`), 0o600))

	implicitPath := filepath.Join(workDir, ThemesFileName)
	require.NoError(t, os.WriteFile(implicitPath, []byte(`
themes:
  implicit-only: {}
  work:
    status:
      ok:
        fg: "#111111"
`), 0o600))

	explicitPath := filepath.Join(workDir, "custom-themes.yml")
	require.NoError(t, os.WriteFile(explicitPath, []byte(`
themes:
  explicit-only: {}
  work:
    status:
      ok:
        fg: "#31748f"
    chars:
      phase_arrow: "=>"
`), 0o600))

	reg := NewRegistry()
	require.NoError(t, reg.LoadUserThemes(explicitPath))

	assert.Equal(t, sortedBuiltinNamesPlus("explicit-only", "global-only", "work"), reg.Names(),
		"the explicit file replaces the implicit local file")
	assert.Equal(t, explicitPath, reg.Source("explicit-only"))
	assert.Equal(t, explicitPath, reg.Source("work"), "the explicit file is the highest precedence contributor")
	assert.Equal(t, globalPath, reg.Source("global-only"))

	scheme, ok := reg.Build("work")
	require.True(t, ok)
	assert.Equal(t, style.Color("#31748f"), scheme.Status.OK.Style.GetForeground(), "explicit field must win")
	assert.Equal(t, style.Color("#c4a7e7"), scheme.Status.Warning.Style.GetForeground(), "unset explicit field must inherit from the global theme")
	assert.Equal(t, "=>", scheme.Chars.PhaseArrow)

	missingPath := filepath.Join(workDir, "missing-themes.yml")

	reg = NewRegistry()
	loadErr := reg.LoadUserThemes(missingPath)
	require.Error(t, loadErr)
	assert.Contains(t, loadErr.Error(), strconv.Quote(missingPath), "the error must name the missing explicit path")
	assert.Equal(t, NewRegistry().Names(), reg.Names(), "a failed load must not extend the registry")
}

// TestLoadThemesFilesMergePrecedence is the merge semantics table for the layered
// global and local themes files: local values win field by field, unset local
// fields inherit from the global theme and fields unset in both inherit from the
// default theme. Provenance follows the same precedence.
//
//nolint:funlen // large merge precedence table
func TestLoadThemesFilesMergePrecedence(t *testing.T) {
	t.Parallel()

	defaultScheme := DefaultTheme()

	tests := []struct {
		name          string
		globalThemes  string
		localThemes   string
		wantName      string
		wantLocalFile bool
		check         func(t *testing.T, scheme *Theme)
	}{
		{
			name: "theme only in global is available unchanged",
			globalThemes: `
themes:
  work:
    status:
      ok:
        fg: "#98c379"
    chars:
      phase_arrow: "=>"
`,
			localThemes:   "themes: {}\n",
			wantName:      "work",
			wantLocalFile: false,
			check: func(t *testing.T, scheme *Theme) {
				t.Helper()

				assert.Equal(t, style.Color("#98c379"), scheme.Status.OK.Style.GetForeground())
				assert.Equal(t, "=>", scheme.Chars.PhaseArrow)
				assert.Equal(t, defaultScheme.Status.Warning.Style.GetForeground(), scheme.Status.Warning.Style.GetForeground(),
					"unset fields must inherit from the default theme")
			},
		},
		{
			name:         "theme only in local is available unchanged",
			globalThemes: "themes: {}\n",
			localThemes: `
themes:
  work:
    status:
      ok:
        fg: "#31748f"
`,
			wantName:      "work",
			wantLocalFile: true,
			check: func(t *testing.T, scheme *Theme) {
				t.Helper()

				assert.Equal(t, style.Color("#31748f"), scheme.Status.OK.Style.GetForeground())
				assert.Equal(t, defaultScheme.Status.Warning.Style.GetForeground(), scheme.Status.Warning.Style.GetForeground(),
					"unset fields must inherit from the default theme")
			},
		},
		{
			name: "local field wins over global, unset local fields inherit global",
			globalThemes: `
themes:
  work:
    status:
      ok:
        fg: "#98c379"
      warning:
        fg: "#c4a7e7"
    chars:
      phase_arrow: "=>"
`,
			localThemes: `
themes:
  work:
    status:
      ok:
        fg: "#31748f"
`,
			wantName:      "work",
			wantLocalFile: true,
			check: func(t *testing.T, scheme *Theme) {
				t.Helper()

				assert.Equal(t, style.Color("#31748f"), scheme.Status.OK.Style.GetForeground(), "local-set field must win")
				assert.Equal(t, style.Color("#c4a7e7"), scheme.Status.Warning.Style.GetForeground(), "local-unset field must inherit from the global theme")
				assert.Equal(t, "=>", scheme.Chars.PhaseArrow, "local-unset field must inherit from the global theme")
				assert.Equal(t, defaultScheme.Status.Failed.Style.GetForeground(), scheme.Status.Failed.Style.GetForeground(),
					"fields unset in both files must inherit from the default theme")
			},
		},
		{
			name: "local full redefine behaves like shadowing",
			globalThemes: `
themes:
  work:
    status:
      ok:
        fg: "#98c379"
      warning:
        fg: "#c4a7e7"
`,
			localThemes: `
themes:
  work:
    status:
      ok:
        fg: "#31748f"
      warning:
        fg: "#123456"
`,
			wantName:      "work",
			wantLocalFile: true,
			check: func(t *testing.T, scheme *Theme) {
				t.Helper()

				assert.Equal(t, style.Color("#31748f"), scheme.Status.OK.Style.GetForeground())
				assert.Equal(t, style.Color("#123456"), scheme.Status.Warning.Style.GetForeground())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			globalPath := writeThemesFile(t, tt.globalThemes)
			localPath := writeThemesFile(t, tt.localThemes)

			reg := NewRegistry()
			require.NoError(t, reg.LoadThemesFiles(localPath, globalPath))

			scheme, ok := reg.Build(tt.wantName)
			require.True(t, ok, "merged theme must be available")

			tt.check(t, scheme)

			wantSource := globalPath
			if tt.wantLocalFile {
				wantSource = localPath
			}

			assert.Equal(t, wantSource, reg.Source(tt.wantName), "provenance is the highest precedence contributor")
		})
	}
}

// TestLoadThemesFilesFullRedefineMatchesShadowing proves that a local theme which
// redefines every global field yields exactly the scheme of the local file alone.
func TestLoadThemesFilesFullRedefineMatchesShadowing(t *testing.T) {
	t.Parallel()

	globalPath := writeThemesFile(t, `
themes:
  work:
    status:
      ok:
        fg: "#98c379"
      warning:
        fg: "#c4a7e7"
    chars:
      phase_arrow: "=>"
`)
	localPath := writeThemesFile(t, `
themes:
  work:
    status:
      ok:
        fg: "#31748f"
      warning:
        fg: "#123456"
    chars:
      phase_arrow: "->"
`)

	merged := NewRegistry()
	require.NoError(t, merged.LoadThemesFiles(localPath, globalPath))

	mergedScheme, ok := merged.Build("work")
	require.True(t, ok)

	localOnly := NewRegistry()
	require.NoError(t, localOnly.LoadThemesFiles(localPath, ""))

	localScheme, ok := localOnly.Build("work")
	require.True(t, ok)

	assert.Equal(t, localScheme, mergedScheme, "a local theme redefining every global field behaves like shadowing")
}

// TestLoadThemesFilesBrokenFileFails proves a broken themes file in either layer is
// a hard error naming the file: theme problems fail at eval. The global file parses
// first, so with both broken the error names the global file.
func TestLoadThemesFilesBrokenFileFails(t *testing.T) {
	t.Parallel()

	const (
		goodGlobal = "themes:\n  global-theme: {}\n"
		goodLocal  = "themes:\n  local-theme: {}\n"
		broken     = "themes: [broken\n"
	)

	tests := []struct {
		name         string
		brokenGlobal bool
		brokenLocal  bool
	}{
		{name: "broken global fails despite a good local", brokenGlobal: true},
		{name: "broken local fails despite a good global", brokenLocal: true},
		{name: "both broken fail on the global file first", brokenGlobal: true, brokenLocal: true},
		{name: "both clean load fine"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			globalContent, localContent := goodGlobal, goodLocal
			if tt.brokenGlobal {
				globalContent = broken
			}

			if tt.brokenLocal {
				localContent = broken
			}

			globalPath := writeThemesFile(t, globalContent)
			localPath := writeThemesFile(t, localContent)

			reg := NewRegistry()
			err := reg.LoadThemesFiles(localPath, globalPath)

			switch {
			case tt.brokenGlobal:
				require.Error(t, err)
				assert.Contains(t, err.Error(), strconv.Quote(globalPath))
			case tt.brokenLocal:
				require.Error(t, err)
				assert.Contains(t, err.Error(), strconv.Quote(localPath))
			default:
				require.NoError(t, err)
				assert.Equal(t, sortedBuiltinNamesPlus("global-theme", "local-theme"), reg.Names())
			}

			if err != nil {
				assert.Equal(t, NewRegistry().Names(), reg.Names(), "a failed load must not extend the registry")
			}
		})
	}
}

// TestLoadThemesFilesValidateBeforeMerge proves every parsed theme is validated
// before the merge: garbage in one file fails even when the other overrides it.
func TestLoadThemesFilesValidateBeforeMerge(t *testing.T) {
	t.Parallel()

	globalPath := writeThemesFile(t, "themes:\n  work:\n    status:\n      ok:\n        fg: \"#zzzzzz\"\n")
	localPath := writeThemesFile(t, "themes:\n  work:\n    status:\n      ok:\n        fg: \"#31748f\"\n")

	reg := NewRegistry()
	err := reg.LoadThemesFiles(localPath, globalPath)

	require.Error(t, err, "an invalid field must fail even when the local file overrides it")
	assert.Contains(t, err.Error(), strconv.Quote(globalPath))
	assert.Contains(t, err.Error(), `theme "work"`)
	assert.Contains(t, err.Error(), "status.ok.fg")
	assert.Equal(t, NewRegistry().Names(), reg.Names())
}
