package tui

import (
	"context"
	"slices"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/snapshot"
	"github.com/mihakrumpestar/panix/internal/tui/footer"
	"github.com/mihakrumpestar/panix/pkg/tui/clipboard"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

//nolint:lll
func (m *model) keyDefs() []footer.KeyDef {
	kds := []footer.KeyDef{
		{Keys: []string{"q"}, Help: "quit", Handler: m.handleQuit},
		{Keys: []string{"h"}, Help: "toggle inspect/secrets logs", Active: func() bool { return m.conf.Flags.Tui.ShowAllBuildLogs }, Handler: m.handleToggle},
		{Keys: []string{"a"}, Help: "toggle active only", Active: func() bool { return m.conf.Flags.Tui.ShowActiveOnly }, Handler: m.handleToggleActiveOnly},
		{Keys: []string{"c"}, Help: "toggle descriptions/commands", Active: func() bool { return m.conf.Flags.Tui.ShowCommandsInLabels }, Handler: m.handleToggleCommands},
		{Keys: []string{"ctrl+c"}, Help: "copy", Handler: m.handleCopy},
		{Keys: []string{"m"}, Help: "fullscreen", Active: func() bool { return m.viewports.IsFullscreen() }, Handler: m.handleFullscreen},
	}

	if !m.isSnapshot {
		kds = append(kds,
			footer.KeyDef{Keys: []string{"s"}, Help: "snapshot", Handler: m.handleSnapshot},
		)

		if !m.conf.Flags.RequireAllSuccess {
			kds = append(kds,
				footer.KeyDef{Keys: []string{"r"}, Help: "retry", Handler: m.handleRetry},
				footer.KeyDef{Keys: []string{"ctrl+r"}, Help: "restart", Handler: m.handleRestart},
			)
		}
	}

	return kds
}

func (m *model) HandleKeyInput(msg zeroterm.KeyPressMsg) []zeroterm.Cmd {
	if msg.String() == "esc" {
		return m.handleEsc()
	}

	hasActiveInner := m.viewports.HasActiveInner()

	key := msg.String()

	if m.statsTable.HandleNavigation(key, hasActiveInner) {
		m.phaseFlow.Reset()

		return nil
	}

	if m.phaseFlow.HandleNavigation(key, hasActiveInner) {
		m.statsTable.Reset()

		return nil
	}

	for _, kd := range m.footer.KeyDefs() {
		if slices.Contains(kd.Keys, key) {
			cmd := kd.Handler()
			if cmd != nil {
				return []zeroterm.Cmd{cmd}
			}

			return nil
		}
	}

	return nil
}

func (m *model) handleCopy() zeroterm.Cmd {
	resetable := m.resetable.Load()
	if resetable == nil {
		return nil
	}

	content, isInner := m.viewports.GetActiveInnerViewportContent()
	if !isInner {
		return m.footer.Notification().Set("Select an inner viewport to copy", m.conf.ColorScheme.Status.Warning.GetForeground())
	}

	if content == "" {
		return m.footer.Notification().Set("No content to copy", m.conf.ColorScheme.Status.Warning.GetForeground())
	}

	err := clipboard.CopyToClipboard(content)
	if err != nil {
		return m.footer.Notification().Set("Copy failed: "+err.Error(), m.conf.ColorScheme.Status.Failed.GetForeground())
	}

	return m.footer.Notification().Set("Copied to clipboard", m.conf.ColorScheme.Status.OK.GetForeground())
}

func (m *model) handleQuit() zeroterm.Cmd {
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

	return zeroterm.QuitCmd
}

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

func (m *model) handleToggle() zeroterm.Cmd {
	m.conf.Flags.Tui.ShowAllBuildLogs = !m.conf.Flags.Tui.ShowAllBuildLogs

	return nil
}

func (m *model) handleToggleCommands() zeroterm.Cmd {
	m.conf.Flags.Tui.ShowCommandsInLabels = !m.conf.Flags.Tui.ShowCommandsInLabels

	return nil
}

func (m *model) handleToggleActiveOnly() zeroterm.Cmd {
	m.conf.Flags.Tui.ShowActiveOnly = !m.conf.Flags.Tui.ShowActiveOnly

	return nil
}

func (m *model) handleRetry() zeroterm.Cmd {
	resetable := m.resetable.Load()
	if resetable == nil || resetable.workflow == nil {
		return nil
	}

	if m.conf.Flags.Snapshot.OnRetry {
		m.captureSnapshot(config.SnaphsotReasonRetry)
	}

	resetable.workflow.State().Retry.Trigger()

	return nil
}

func (m *model) handleRestart() zeroterm.Cmd {
	// The notification cmd also dispatches the restartMsg as a side effect,
	// so both actions are triggered from a single cmd.
	notifCmd := m.footer.Notification().Set("Restarting workflow...", m.conf.ColorScheme.Status.OK.GetForeground())

	return func() zeroterm.Msg {
		// Run the notification first, then send restartMsg.
		notifCmd()

		return restartMsg{}
	}
}

func (m *model) handleFullscreen() zeroterm.Cmd {
	if m.viewports.IsFullscreen() {
		m.viewports.ExitFullscreen()

		return nil
	}

	activeInnerXpath := m.viewports.GetActiveInnerViewportXpath()

	if activeInnerXpath.Depth() > 0 {
		m.viewports.SetFullscreen(activeInnerXpath)
	} else {
		return m.footer.Notification().Set("Select a viewport first", m.conf.ColorScheme.Status.Warning.GetForeground())
	}

	return nil
}

func (m *model) handleEsc() []zeroterm.Cmd {
	switch {
	case m.viewports.IsFullscreen():
		m.viewports.ExitFullscreen()
	case m.viewports.HasActiveInner():
		m.viewports.DeselectAll()
	case m.statsTable.SelectedIndex() >= 0:
		m.statsTable.Reset()
	case m.phaseFlow.Selected.Index >= 0:
		m.phaseFlow.Reset()
	}

	return nil
}

func (m *model) handleSnapshot() zeroterm.Cmd {
	m.captureSnapshot(config.SnaphsotReasonManual)

	return m.footer.Notification().Set("Snapshot saved", m.conf.ColorScheme.Status.OK.GetForeground())
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
