package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/notifications"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/internal/snapshot"
	"github.com/pkg/errors"
)

type snapshotInfo struct {
	reason       string
	snapshotTime time.Time
}

// NewSnapshotTui loads snapshot data and displays it in the TUI.
func NewSnapshotTui(ctx context.Context, snap *snapshot.Snapshot) error {
	conf, err := snapshot.Restore(snap)
	if err != nil {
		return errors.Wrap(err, "failed to restore snapshot")
	}

	zone.NewGlobal()
	defer zone.Close()
	defer setupSIGINTHandler(ctx)()

	dimensions := &viewports.Dimensions{
		Width:  initialWidth,
		Height: initialHeight,
	}

	spinners, err := spinners.NewSpinners()
	if err != nil {
		return errors.Wrap(err, "failed to create spinners")
	}

	r := &resetable{
		workflow:    nil,
		spinners:    spinners,
		viewports:   viewports.NewViewports(dimensions, conf),
		statsTable:  NewStatsTable(),
		phaseStatus: NewPhaseStatus(),
	}

	m := &model{
		ctx:          ctx,
		conf:         conf,
		dimensions:   dimensions,
		notification: notifications.New(),
		footer:       newSnapshotFooter(),
		isSnapshot:   true,
		snapshotInfo: &snapshotInfo{
			reason:       string(snap.Reason),
			snapshotTime: time.Unix(snap.SnapshotTime, 0),
		},
	}

	m.resetable.Store(r)

	program := tea.NewProgram(m)

	finalModel, err := program.Run()
	if err != nil {
		return errors.Wrap(err, "snapshot TUI runtime error")
	}

	finalModelTUI, ok := finalModel.(*model)
	if !ok {
		return ErrTypeAssertionFinalModel
	}

	if finalModelTUI.quitting {
		content := finalModelTUI.viewMainContent()
		fmt.Println(content)
	}

	return nil
}

// snapshotKeyDefs defines keybindings available in snapshot viewing mode.
var snapshotKeyDefs = []keyDef{
	{[]string{"q"}, "quit", (*model).handleQuitSnapshot},
	{[]string{"h"}, "toggle inspect/secrets logs", (*model).handleToggle},
	{[]string{"a"}, "toggle active only", (*model).handleToggleActiveOnly},
	{[]string{"c"}, "toggle descriptions/commands", (*model).handleToggleCommands},
	{[]string{"ctrl+c"}, "copy", (*model).handleCopy},
	{[]string{"m"}, "fullscreen", (*model).handleFullscreen},
}

func (m *model) handleQuitSnapshot() (tea.Model, tea.Cmd) {
	m.quitting = true

	return m, tea.Quit
}

func newSnapshotFooter() *Footer {
	bindings := make([]key.Binding, len(snapshotKeyDefs))
	for i := range snapshotKeyDefs {
		bindings[i] = key.NewBinding(
			key.WithKeys(snapshotKeyDefs[i].keys...),
			key.WithHelp(snapshotKeyDefs[i].keys[0], snapshotKeyDefs[i].help),
		)
	}

	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	h.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))

	return &Footer{
		keymapHelp:  h,
		keyBindings: bindings,
	}
}

func (m *model) viewSnapshotHeader() string {
	if m.snapshotInfo == nil {
		return ""
	}

	si := m.snapshotInfo

	style := m.conf.ColorScheme.Header.Title

	reasonStyle := m.conf.ColorScheme.Status.Warning

	parts := []string{
		style.Render("panix snapshot viewer"),
		m.conf.ColorScheme.Table.Row.Render(fmt.Sprintf("v%s", m.conf.PanixVersion)),
		reasonStyle.Render(si.reason),
		m.conf.ColorScheme.Table.Row.Render(fmt.Sprintf("started: %s", m.conf.StartTime.Format("2006-01-02 15:04:05"))),
		m.conf.ColorScheme.Table.Row.Render(fmt.Sprintf("snapshot: %s", si.snapshotTime.Format("2006-01-02 15:04:05"))),
	}

	return strings.Join(parts, "  ") + "\n"
}
