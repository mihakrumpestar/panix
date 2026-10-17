package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/internal/tui/buildlogs"
	"github.com/mihakrumpestar/panix/internal/tui/footer"
	"github.com/mihakrumpestar/panix/internal/tui/header"
	"github.com/mihakrumpestar/panix/internal/tui/phaseflow"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/profile"
	"github.com/mihakrumpestar/panix/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/pkg/tui/tree"
	"github.com/mihakrumpestar/panix/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/pkg/errors"
)

var ErrResolveTheme = errors.New("internal error: failed to resolve the configured theme")

// activeComponent tracks which navigable component receives left/right key
// presses. It must be explicit: both components treat selectedIndex < 0 as
// "select first item on right", so selection state cannot disambiguate them.
type activeComponent int

const (
	compNone activeComponent = iota // nothing selected: stats table gets priority
	compStatsTable
	compPhaseFlow
)

type model struct {
	ctx                context.Context
	conf               *config.Config
	dimensions         *viewports.Dimensions
	quitting           bool
	isSnapshot         bool
	workflow           *workflow.Workflow
	lastWorkflowUpdate time.Time
	err                error

	active     activeComponent
	header     *header.Header
	buildLogs  *buildlogs.BuildLogs
	footer     *footer.Footer
	spinners   *spinners.Spinners
	viewports  *viewports.Viewports
	statsTable *statstable.StatsTable
	phaseFlow  *phaseflow.PhaseFlow
	cachedTree *tree.Node

	theme        *theme.Theme
	themePicker  themePickerState
	footerKeyCtx modalKind

	// content is a persistent buffer for the main content area, reused across
	// frames via Reset().
	content *buffer.LinesBuf
}

func New(ctx context.Context, conf *config.Config, isSnapshot bool) error {
	defer setupSIGINTHandler(ctx)()

	profileStop, err := profile.Start(conf.Flags.Profile)
	if err != nil {
		return errors.Wrap(err, "failed to start profiling")
	}
	defer profileStop()

	theme, ok := conf.ResolveTheme()
	if !ok {
		return ErrResolveTheme
	}

	mdl := newModel(ctx, conf, theme, isSnapshot)

	mdl.applyTheme()

	program := zeroterm.NewProgram(mdl, zeroterm.WithWidthProbe(
		collectProbeGlyphs(conf),
		func(measured map[string]int, elapsed time.Duration) {
			installMeasuredGlyphWidths(conf, measured, elapsed)
			// Widgets size columns from glyph widths, so re-apply the theme after
			// installing so measured widths are the ones they use.
			mdl.applyTheme()
		},
	))

	err = program.Run()
	if err != nil {
		return errors.Wrap(err, "TUI runtime error")
	}

	if mdl.quitting {
		buf := buffer.NewLinesBufDiff()

		buf.AppendFrom(mdl.header.Render(mdl.dimensions.Width))

		if mdl.workflow != nil {
			mdl.viewMainContentInto(buf, 0, 0, true)
		}

		fmt.Println(buf.String())
	}

	if mdl.err != nil {
		return mdl.err
	}

	return nil
}

// newModel builds the model and every widget from the resolved theme and config.
// The theme is runtime state of the TUI; the config stores only the registry and
// the name.
func newModel(ctx context.Context, conf *config.Config, scheme *theme.Theme, isSnapshot bool) *model {
	dimensions := &viewports.Dimensions{}
	tableS := scheme.Table

	mdl := &model{
		ctx:        ctx,
		conf:       conf,
		dimensions: dimensions,
		isSnapshot: isSnapshot,
		theme:      scheme,

		header:   header.New(isSnapshot, conf.Snapshot, scheme),
		spinners: spinners.New(scheme.Spinner.FrameBytes, scheme.Spinner.Interval.Duration()),
		viewports: viewports.New(dimensions, conf.Flags.CommandOutputMaxHeight,
			tableS.Border.Style, tableS.SelectionHighlightBackground.Style, tableS.SelectionHighlightBorder.Style),
		statsTable: statstable.New(conf.Fleet, scheme),
		phaseFlow:  phaseflow.New(conf.Fleet, scheme, conf.Phases),
		cachedTree: tree.NewTree(scheme.Tree.Enumerator.Style, buildlogs.TreeStep, treeGlyphs(scheme.Tree)),
		content:    buffer.NewLinesBuf(),
	}

	mdl.footer = footer.New(mdl.keyDefs(), scheme)

	return mdl
}

func setupSIGINTHandler(ctx context.Context) func() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigChan:
			}
		}
	}()

	return func() {
		signal.Stop(sigChan)
	}
}
