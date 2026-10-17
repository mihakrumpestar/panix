package theme

import (
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:funlen // large merge semantics table
func TestThemeMergeSemantics(t *testing.T) {
	t.Parallel()

	base := defaultTheme()

	tests := []struct {
		name     string
		override Theme
		check    func(t *testing.T, merged Theme)
	}{
		{
			name:     "zero override keeps everything from base",
			override: Theme{},
			check: func(t *testing.T, merged Theme) {
				t.Helper()

				assert.Equal(t, base, merged)
			},
		},
		{
			name:     "set color wins over base",
			override: Theme{Status: ThemeStatus{OK: ThemeStyle{Fg: "#31748f"}}},
			check: func(t *testing.T, merged Theme) {
				t.Helper()

				assert.Equal(t, style.Color("#31748f"), merged.Status.OK.Fg)
				assert.Equal(t, base.Status.Warning.Fg, merged.Status.Warning.Fg, "unset sibling fields must inherit")
			},
		},
		{
			name:     "empty color inherits from base",
			override: Theme{Status: ThemeStatus{OK: ThemeStyle{}}},
			check: func(t *testing.T, merged Theme) {
				t.Helper()

				assert.Equal(t, base.Status.OK.Fg, merged.Status.OK.Fg)
			},
		},
		{
			name:     "partial style keeps base bold and padding",
			override: Theme{Header: ThemeHeader{Title: ThemeStyle{Fg: "#c4a7e7"}}},
			check: func(t *testing.T, merged Theme) {
				t.Helper()

				assert.Equal(t, style.Color("#c4a7e7"), merged.Header.Title.Fg)
				assert.Equal(t, new(true), merged.Header.Title.Bold, "bold must inherit from base")
			},
		},
		{
			name:     "deliberate false overrides base true",
			override: Theme{Header: ThemeHeader{Title: ThemeStyle{Bold: new(false)}}},
			check: func(t *testing.T, merged Theme) {
				t.Helper()

				require.NotNil(t, merged.Header.Title.Bold)
				assert.False(t, *merged.Header.Title.Bold)
				assert.Equal(t, base.Header.Title.Fg, merged.Header.Title.Fg)
			},
		},
		{
			name:     "deliberate zero padding overrides base padding",
			override: Theme{PhaseStatus: ThemePhaseStatus{Pill: ThemeStyle{PadX: new(0)}}},
			check: func(t *testing.T, merged Theme) {
				t.Helper()

				require.NotNil(t, merged.PhaseStatus.Pill.PadX)
				assert.Equal(t, 0, *merged.PhaseStatus.Pill.PadX)
				require.NotNil(t, merged.PhaseStatus.Pill.PadY, "unset padding must inherit")
				assert.Equal(t, 0, *merged.PhaseStatus.Pill.PadY)
			},
		},
		{
			name:     "gradient pair wins when set",
			override: Theme{PhaseStatus: ThemePhaseStatus{Running: ThemePair{"#31748f", "#9ccfd8"}}},
			check: func(t *testing.T, merged Theme) {
				t.Helper()

				assert.Equal(t, ThemePair{"#31748f", "#9ccfd8"}, merged.PhaseStatus.Running)
				assert.Equal(t, base.PhaseStatus.Done, merged.PhaseStatus.Done, "unset pairs must inherit")
			},
		},
		{
			name:     "empty gradient pair inherits",
			override: Theme{PhaseStatus: ThemePhaseStatus{Running: ThemePair{}}},
			check: func(t *testing.T, merged Theme) {
				t.Helper()

				assert.Equal(t, base.PhaseStatus.Running, merged.PhaseStatus.Running)
			},
		},
		{
			name:     "glyph wins when set",
			override: Theme{Chars: ThemeChars{PhaseArrow: "->"}},
			check: func(t *testing.T, merged Theme) {
				t.Helper()

				assert.Equal(t, "->", merged.Chars.PhaseArrow)
				assert.Equal(t, base.Chars.SnapshotIcon, merged.Chars.SnapshotIcon, "unset glyphs must inherit")
			},
		},
		{
			name:     "spinner frames and interval win when set",
			override: Theme{Spinner: ThemeSpinner{Frames: []string{"|", "/"}, Interval: yamlx.Duration(50 * time.Millisecond)}},
			check: func(t *testing.T, merged Theme) {
				t.Helper()

				assert.Equal(t, []string{"|", "/"}, merged.Spinner.Frames)
				assert.Equal(t, yamlx.Duration(50*time.Millisecond), merged.Spinner.Interval)
			},
		},
		{
			name:     "empty spinner inherits",
			override: Theme{Spinner: ThemeSpinner{}},
			check: func(t *testing.T, merged Theme) {
				t.Helper()

				assert.Equal(t, base.Spinner, merged.Spinner)
			},
		},
		{
			name:     "empty non-nil spinner frames inherit",
			override: Theme{Spinner: ThemeSpinner{Frames: []string{}}},
			check: func(t *testing.T, merged Theme) {
				t.Helper()

				assert.Equal(t, base.Spinner, merged.Spinner, "an empty non-nil list is unset, not an override")
			},
		},
		{
			name:     "log entity partial override keeps icon and bold",
			override: Theme{Flake: ThemeLogEntity{Color: "#f6c177"}},
			check: func(t *testing.T, merged Theme) {
				t.Helper()

				assert.Equal(t, style.Color("#f6c177"), merged.Flake.Color)
				assert.Equal(t, base.Flake.Icon, merged.Flake.Icon, "icon must inherit")
				assert.Equal(t, base.Flake.Bold, merged.Flake.Bold, "bold must inherit")
			},
		},
		{
			name:     "status icons inherit when only colors are set",
			override: Theme{Status: ThemeStatus{OK: ThemeStyle{Fg: "#31748f"}}},
			check: func(t *testing.T, merged Theme) {
				t.Helper()

				assert.Equal(t, base.Status.Icons, merged.Status.Icons)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			merged := base.Merge(tt.override)

			tt.check(t, merged)

			assert.Equal(t, defaultTheme(), base, "Merge must not mutate the base theme")
		})
	}
}

func TestThemeMergeDoesNotMutateOverride(t *testing.T) {
	t.Parallel()

	override := Theme{Status: ThemeStatus{OK: ThemeStyle{Fg: "#31748f"}}}
	snapshot := override

	defaultTheme().Merge(override)

	assert.Equal(t, snapshot, override, "Merge must not mutate the override theme")
}

// TestThemeMergeAliasesOverride pins the documented aliasing contract: set
// pointer and slice fields are shared with the override, so a merged theme must
// be cloned when the caller needs independent memory.
func TestThemeMergeAliasesOverride(t *testing.T) {
	t.Parallel()

	frames := []string{"|", "/"}
	bold := true
	override := Theme{
		Status:  ThemeStatus{OK: ThemeStyle{Bold: &bold}},
		Spinner: ThemeSpinner{Frames: frames},
	}

	merged := defaultTheme().Merge(override)

	frames[0] = "X"

	assert.Equal(t, "X", merged.Spinner.Frames[0], "merged slice shares backing memory with the override")

	merged.Spinner.Frames[1] = "Y"

	assert.Equal(t, "Y", override.Spinner.Frames[1], "mutating the merged slice mutates the override")

	*merged.Status.OK.Bold = false

	assert.False(t, bold, "merged pointer is the override pointer")
}

// TestThemeMergeRejectsHalfPair proves a one-sided pair never reaches a merged
// theme: the themes file parse validates it before any merge can run.
func TestThemeMergeRejectsHalfPair(t *testing.T) {
	t.Parallel()

	path := writeThemesFile(t, "themes:\n  custom:\n    phase_status:\n      running: [\"#c4a7e7\"]\n")

	themes, err := ParseThemesFile(path)

	require.Error(t, err, "a half pair must fail validation")
	assert.Contains(t, err.Error(), "phase_status.running[1]")
	assert.Nil(t, themes, "no half pair may reach the registry")

	override := Theme{PhaseStatus: ThemePhaseStatus{Running: ThemePair{"#c4a7e7", ""}}}
	require.Error(t, override.Validate(), "Merge relies on validation rejecting half pairs")
}
