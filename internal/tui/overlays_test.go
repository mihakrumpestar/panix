package tui

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/internal/tui/buildlogs"
	"github.com/mihakrumpestar/panix/internal/tui/footer"
	"github.com/mihakrumpestar/panix/internal/tui/header"
	"github.com/mihakrumpestar/panix/internal/tui/phaseflow"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/tree"
	"github.com/mihakrumpestar/panix/pkg/tui/viewports"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// themeTestModel builds a snapshot-mode model by hand for the theme overlay
// tests: a rendered header (so the overlay region starts below it), every widget
// wired, and a cached tree so a full frame render works.
func themeTestModel(t *testing.T) *model {
	t.Helper()

	conf := makeTestConfig()
	conf.Flags.DryRun = true

	scheme := theme.DefaultTheme()
	dims := &viewports.Dimensions{Width: 200, Height: 80}

	mdl := &model{
		ctx:        context.Background(),
		conf:       conf,
		dimensions: dims,
		isSnapshot: true,
		theme:      scheme,
		header:     header.New(true, config.Snapshot{}, scheme),
		spinners:   spinners.New(scheme.Spinner.FrameBytes, scheme.Spinner.Interval.Duration()),
		statsTable: statstable.New(conf.Fleet, scheme),
		phaseFlow:  phaseflow.New(conf.Fleet, scheme, conf.Phases),
		cachedTree: tree.NewTree(scheme.Tree.Enumerator.Style, buildlogs.TreeStep, treeGlyphs(scheme.Tree)),
		content:    buffer.NewLinesBuf(),
		viewports: viewports.New(dims, 0, scheme.Table.Border.Style,
			scheme.Table.SelectionHighlightBackground.Style, scheme.Table.SelectionHighlightBorder.Style),
	}
	mdl.footer = footer.New(mdl.keyDefs(), scheme)

	return mdl
}

// themeConfigContent is a minimal panix config with a resolvable theme flag; the
// enter tests pin that theme choices are never written back into it.
const themeConfigContent = "flags:\n  tui:\n    theme: panix\n\nfleet: {}\n"

// themeOverlayModel returns a themeTestModel wired for the modal overlay tests: a
// resolved theme registry, a runnable workflow and a temp panix config file that
// theme choices are never written back to.
func themeOverlayModel(t *testing.T, configContent string) *model {
	t.Helper()

	mdl := themeTestModel(t)
	mdl.content = buffer.NewLinesBuf() // set by New(); themeTestModel builds the model by hand

	w, err := workflow.NewWorkflow(mdl.ctx, mdl.conf)
	require.NoError(t, err)

	mdl.workflow = w
	mdl.conf.Theme.Registry = theme.NewRegistry()
	mdl.previewTheme(themeNameFirst) // the overlay tests open on the first built-in and browse from there
	require.Equal(t, themeNameFirst, mdl.conf.Theme.Name, "the first built-in theme must activate")

	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte(configContent), 0o600))

	mdl.conf.Flags.Config = path

	return mdl
}

func renderModelFrame(mdl *model) string {
	buf := buffer.NewLinesBufDiff()
	mdl.Render(buf, 0)

	return buf.String()
}

// TestModalOverlayPreservesBackgroundContent pins the overlay compositing
// contract: the modal box dims the background content instead of blanking it and
// the content around the box stays visible.
func TestModalOverlayPreservesBackgroundContent(t *testing.T) {
	t.Parallel()

	mdl := themeOverlayModel(t, themeConfigContent)

	plain := renderModelFrame(mdl)
	assert.NotContains(t, plain, "\x1b[2m", "no dim without an open overlay")

	mustOpenThemePicker(t, mdl)

	frame := renderModelFrame(mdl)
	visible := string(style.StripANSI([]byte(frame)))

	assert.Contains(t, frame, "\x1b[2m", "the content area must be dimmed under the overlay")
	assert.Contains(t, visible, "nixosConfigurations", "background content must stay visible around the box")
	assert.Contains(t, visible, themePickerTitle, "the box must render over the dimmed content")
}

// TestComposeOverlayKeepsRowSides pins the row-level compositing contract: a
// composed row keeps its own content left and right of the box and carries the
// dim of the content area.
func TestComposeOverlayKeepsRowSides(t *testing.T) {
	t.Parallel()

	mdl := themeOverlayModel(t, themeConfigContent)
	renderModelFrame(mdl) // populate the header/footer heights

	const width = 200

	contentRow := "L" + strings.Repeat("x", width-2) + "R"

	buf := buffer.NewLinesBufDiff()

	for range mdl.header.Len() {
		buf.WriteString(strings.Repeat("h", width))
	}

	for range 8 {
		buf.WriteString(contentRow)
	}

	for range mdl.footer.Len() {
		buf.WriteString(strings.Repeat("f", width))
	}

	mdl.composeOverlay(buf, mdl.themePickerBox())

	assert.Equal(t, strings.Repeat("h", width), string(buf.Line(0)), "header rows must stay untouched")
	assert.Equal(t, strings.Repeat("f", width), string(buf.Line(buf.Len()-1)), "footer rows must stay untouched")

	for row := mdl.header.Len(); row < buf.Len()-mdl.footer.Len(); row++ {
		line := buf.Line(row)
		visible := string(style.StripANSI(line))

		assert.Contains(t, string(line), "\x1b[2m", "row %d must be dimmed", row)
		assert.True(t, strings.HasPrefix(visible, "L"), "row %d must keep its left content: %q", row, visible)
		assert.True(t, strings.HasSuffix(visible, "R"), "row %d must keep its right content: %q", row, visible)
	}
}

// colorLiteralPattern matches hardcoded terminal colors: hex literals and
// style.Color literal conversions. Colors must come from the theme registry.
var colorLiteralPattern = regexp.MustCompile(`#[0-9a-fA-F]{6}\b|style\.Color\("`)

// TestNoHardcodedColorLiteralsInTUI guards the theming contract: no TUI source
// outside the theme registry may hardcode a color, so all themes stay readable.
func TestNoHardcodedColorLiteralsInTUI(t *testing.T) {
	t.Parallel()

	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		raw, err := os.ReadFile(path) //nolint:gosec // test walks its own package sources
		if err != nil {
			return errors.Wrap(err, "failed to read "+path)
		}

		for line := range strings.SplitSeq(string(raw), "\n") {
			assert.Empty(t, colorLiteralPattern.FindString(line), "%s: hardcoded color literal: %q", path, line)
		}

		return nil
	})
	require.NoError(t, err)
}

// TestOverlayBoxStyle_UsesOverlayRegionBorder proves the modal boxes draw the
// themed overlay region border preset.
func TestOverlayBoxStyle_UsesOverlayRegionBorder(t *testing.T) {
	t.Parallel()

	heavyBorder, ok := style.BorderByName("heavy")
	require.True(t, ok)

	merged := theme.DefaultTheme().Merge(theme.Theme{
		Border: theme.ThemeBorder{Overlay: "heavy"},
	})

	got, _ := overlayBoxStyle(&merged).GetBorder()

	assert.Equal(t, heavyBorder, got)
}
