package tui

import (
	"context"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/clipboard"
	"github.com/mihakrumpestar/panix/internal/snapshot"
	"github.com/mihakrumpestar/panix/internal/tui/footer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func (m *model) keyDefs() []footer.KeyDef {
	kds := []footer.KeyDef{
		{Keys: []string{"q"}, Help: "quit", Handler: m.handleQuit},
		{Keys: []string{"h"}, Help: "toggle inspect/secrets logs", Handler: m.handleToggle},
		{Keys: []string{"a"}, Help: "toggle active only", Handler: m.handleToggleActiveOnly},
		{Keys: []string{"c"}, Help: "toggle descriptions/commands", Handler: m.handleToggleCommands},
		{Keys: []string{"ctrl+c"}, Help: "copy", Handler: m.handleCopy},
		{Keys: []string{"m"}, Help: "fullscreen", Handler: m.handleFullscreen},
	}

	if !m.isSnapshot {
		kds = append(kds,
			footer.KeyDef{Keys: []string{"s"}, Help: "snapshot", Handler: m.handleSnapshot},
		)

		if !m.conf.Flags.ExitOnComplete {
			kds = append(kds,
				footer.KeyDef{Keys: []string{"r"}, Help: "retry", Handler: m.handleRetry},
				footer.KeyDef{Keys: []string{"ctrl+r"}, Help: "restart", Handler: m.handleRestart},
			)
		}
	}

	return kds
}

func (m *model) HandleKeyInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		return m.handleEsc()
	}

	resetable := m.resetable.Load()
	if resetable == nil {
		return m, nil
	}

	hasActiveInner := resetable.viewports.HasActiveInner()

	statsTable := m.conf.Fleet.StatsTable
	if statsTable.HandleNavigation(msg.String(), hasActiveInner) {
		m.conf.Fleet.PhaseStatus.Reset()

		return m, nil
	}

	phaseStatus := m.conf.Fleet.PhaseStatus
	if phaseStatus.HandleNavigation(msg.String(), hasActiveInner) {
		m.conf.Fleet.StatsTable.Reset()

		return m, nil
	}

	for _, kd := range m.footer.KeyDefs() {
		if slices.Contains(kd.Keys, msg.String()) {
			return kd.Handler()
		}
	}

	return m, nil
}

func (m *model) handleCopy() (tea.Model, tea.Cmd) {
	resetable := m.resetable.Load()
	if resetable == nil {
		return m, nil
	}

	content, isInner := resetable.viewports.GetActiveInnerViewportContent()
	if !isInner {
		return m, m.footer.Notification().Set("Select an inner viewport to copy", m.conf.ColorScheme.Status.Warning)
	}

	if content == "" {
		return m, m.footer.Notification().Set("No content to copy", m.conf.ColorScheme.Status.Warning)
	}

	err := clipboard.CopyToClipboard(content)
	if err != nil {
		return m, m.footer.Notification().Set("Copy failed: "+err.Error(), m.conf.ColorScheme.Status.Failed)
	}

	return m, m.footer.Notification().Set("Copied to clipboard", m.conf.ColorScheme.Status.OK)
}

func (m *model) handleQuit() (tea.Model, tea.Cmd) {
	m.quitting = true
	resetable := m.resetable.Load()

	if resetable != nil && resetable.workflow != nil {
		if m.conf.Flags.Snapshot.OnExit {
			m.captureSnapshot(config.SnaphsotReasonExit)
		}

		err := resetable.workflow.Cancel()
		if m.err != nil && err != nil && !errors.Is(err, context.Canceled) {
			m.err = errors.Wrap(m.err, err.Error())
		} else if err != nil && !errors.Is(err, context.Canceled) {
			m.err = err
		}
	}

	m.setFailedMachinesErrorIfNil()

	log.Debug().Msg("Context done, exiting TUI")

	return m, tea.Quit
}

// setFailedMachinesErrorIfNil checks fleet state for failed machines and sets
// m.err when it's nil. This handles the case where the user quits while the
// workflow is still retrying (no workflowDoneMsg received yet).
func (m *model) setFailedMachinesErrorIfNil() {
	if m.err != nil {
		return
	}

	m.conf.Fleet.Recalculate(m.conf.Phases)
	logFinalState(m.conf)

	for _, fleetLeaf := range m.conf.Fleet.AllMachines() {
		if fleetLeaf.Machine.State.Load().Status == stats.Failed {
			m.err = errMachinesFailed

			return
		}
	}
}

func (m *model) handleToggle() (tea.Model, tea.Cmd) {
	m.conf.Flags.Tui.ShowAllBuildLogs = !m.conf.Flags.Tui.ShowAllBuildLogs

	return m, nil
}

func (m *model) handleToggleCommands() (tea.Model, tea.Cmd) {
	m.conf.Flags.Tui.ShowCommandsInLabels = !m.conf.Flags.Tui.ShowCommandsInLabels

	return m, nil
}

func (m *model) handleToggleActiveOnly() (tea.Model, tea.Cmd) {
	m.conf.Flags.Tui.ShowActiveOnly = !m.conf.Flags.Tui.ShowActiveOnly

	return m, nil
}

func (m *model) handleRetry() (tea.Model, tea.Cmd) {
	resetable := m.resetable.Load()
	if resetable == nil || resetable.workflow == nil {
		return m, nil
	}

	if m.conf.Flags.Snapshot.OnRetry {
		m.captureSnapshot(config.SnaphsotReasonRetry)
	}

	resetable.workflow.State().Retry.Trigger()

	return m, nil
}

func (m *model) handleRestart() (tea.Model, tea.Cmd) {
	return m, tea.Batch(
		m.footer.Notification().Set("Restarting workflow...", m.conf.ColorScheme.Status.OK),
		func() tea.Msg { return restartMsg{} },
	)
}

func (m *model) handleFullscreen() (tea.Model, tea.Cmd) {
	resetable := m.resetable.Load()
	if resetable == nil {
		return m, nil
	}

	if resetable.viewports.IsFullscreen() {
		resetable.viewports.ExitFullscreen()

		return m, nil
	}

	activeInnerXpath := resetable.viewports.GetActiveInnerViewportXpath()

	if activeInnerXpath.Depth() > 0 {
		resetable.viewports.SetFullscreen(activeInnerXpath)
	} else {
		return m, m.footer.Notification().Set("Select a viewport first", m.conf.ColorScheme.Status.Warning)
	}

	return m, nil
}

func (m *model) handleEsc() (tea.Model, tea.Cmd) {
	resetable := m.resetable.Load()

	statsTable := m.conf.Fleet.StatsTable
	phaseStatus := m.conf.Fleet.PhaseStatus

	if resetable == nil {
		return m, nil
	}

	switch {
	case resetable.viewports.IsFullscreen():
		resetable.viewports.ExitFullscreen()
	case resetable.viewports.HasActiveInner():
		resetable.viewports.DeselectAll()
	case statsTable.Selected.Index >= 0:
		statsTable.Reset()
	case phaseStatus.Selected.Index >= 0:
		phaseStatus.Reset()
	}

	return m, nil
}

func (m *model) handleSnapshot() (tea.Model, tea.Cmd) {
	m.captureSnapshot(config.SnaphsotReasonManual)

	return m, m.footer.Notification().Set("Snapshot saved", m.conf.ColorScheme.Status.OK)
}

func (m *model) captureSnapshot(reason config.SnaphsotReason) {
	resetable := m.resetable.Load()
	if resetable == nil || resetable.workflow == nil {
		return
	}

	snap := snapshot.Capture(m.conf, reason, m.err)

	err := snapshot.Write(m.conf.Flags.Snapshot.Dir, snap)
	if err != nil {
		log.Error().Err(err).Msg("failed to write snapshot")
	}
}
