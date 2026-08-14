package tui

import (
	"bytes"
	"slices"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/snapshot"
	"github.com/mihakrumpestar/panix/internal/tui/footer"
	"github.com/mihakrumpestar/panix/pkg/clipboard"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/rs/zerolog/log"
)

//nolint:lll
func (m *model) keyDefs() []footer.KeyDef {
	kds := []footer.KeyDef{
		{Keys: []string{"q"}, Help: "quit", Handler: m.handleQuit},
		{Keys: []string{"ctrl+q"}, Help: "force quit", Handler: m.handleForceQuit},
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

	// While quitting, only ctrl+q (force quit) remains usable; every other
	// key is disabled and hidden from the keymap. Centralized here instead
	// of per-handler guards: e.g. ctrl+r while quitting would call the
	// blocking workflow.Cancel() inside the event loop (re-freezing the TUI)
	// and the leftover quitting flag would kill the restarted run on its
	// first done message.
	for i := range kds {
		if slices.Contains(kds[i].Keys, "ctrl+q") {
			continue
		}

		kds[i].Disabled = func() bool { return m.quitting }
	}

	return kds
}

// navigable is the contract for components that respond to left/right
// navigation and can be deselected. Both *statstable.StatsTable and
// *phaseflow.PhaseFlow satisfy it.
type navigable interface {
	HandleNavigation(key string, hasActiveInnerViewport bool) bool
	Reset()
}

func (m *model) HandleKeyInput(msg zeroterm.KeyPressMsg) zeroterm.Cmd {
	key := msg.String()

	if key == "esc" {
		return m.handleEsc()
	}

	hasActiveInner := m.viewports.HasActiveInner()

	// Try the active component first. When nothing is active, the stats
	// table gets priority for initial selection. If the active component
	// is at a boundary it can't consume, the key spills over to the other.
	primary, secondary := navigable(m.statsTable), navigable(m.phaseFlow)

	primaryComp, secondaryComp := compStatsTable, compPhaseFlow
	if m.active == compPhaseFlow {
		primary, secondary = secondary, primary
		primaryComp, secondaryComp = secondaryComp, primaryComp
	}

	if primary.HandleNavigation(key, hasActiveInner) {
		if m.active != primaryComp {
			secondary.Reset()

			m.active = primaryComp
		}

		m.cachedTree.InvalidateCache()

		return nil
	}

	if secondary.HandleNavigation(key, hasActiveInner) {
		primary.Reset()

		m.active = secondaryComp
		m.cachedTree.InvalidateCache()

		return nil
	}

	for _, keyDef := range m.footer.KeyDefs() {
		if slices.Contains(keyDef.Keys, key) {
			if m.isKeyDisabled(keyDef) {
				return nil
			}

			cmd := keyDef.Handler()
			if cmd != nil {
				return cmd
			}

			return nil
		}
	}

	return nil
}

// isKeyDisabled reports whether the key binding is currently unusable.
func (m *model) isKeyDisabled(keyDef footer.KeyDef) bool {
	return keyDef.Disabled != nil && keyDef.Disabled()
}

func (m *model) handleCopy() zeroterm.Cmd {
	if m.workflow == nil {
		return nil
	}

	content, isInner := m.viewports.GetActiveInnerViewportContent()
	if !isInner {
		return m.footer.Notification().Set("Select an inner viewport to copy", m.conf.ColorScheme.Status.Warning.GetForeground())
	}

	if len(content) == 0 {
		return m.footer.Notification().Set("No content to copy", m.conf.ColorScheme.Status.Warning.GetForeground())
	}

	err := clipboard.CopyToClipboard(string(bytes.Join(content, []byte("\n"))))
	if err != nil {
		return m.footer.Notification().Set("Copy failed: "+err.Error(), m.conf.ColorScheme.Status.Failed.GetForeground())
	}

	return m.footer.Notification().Set("Copied to clipboard", m.conf.ColorScheme.Status.OK.GetForeground())
}

// quitCmd centralizes TUI exit: captures the exit snapshot when enabled, logs,
// and returns the quit command. Every exit site routes through here so the
// snapshot is taken exactly once per exit.
func (m *model) quitCmd() zeroterm.Cmd {
	if m.conf.Flags.Snapshot.OnExit {
		m.captureSnapshot(config.SnaphsotReasonExit)
	}

	log.Debug().Msg("Context done, exiting TUI")

	return zeroterm.QuitCmd
}

func (m *model) handleQuit() zeroterm.Cmd {
	if m.quitting {
		return nil
	}

	m.quitting = true

	// Nothing to wait for without a workflow (or when it already finished,
	// since no workflowDoneMsg will arrive again): quit immediately.
	if m.workflow == nil {
		m.setFailedMachinesErrorIfNil()

		return m.quitCmd()
	}

	select {
	case <-m.workflow.Done():
		m.setFailedMachinesErrorIfNil()

		return m.quitCmd()
	default:
	}

	// Cancel asynchronously: Cancel() blocks until every command finishes,
	// which would freeze the zeroterm event loop. CancelAsync is a plain
	// context cancel (non-blocking, idempotent, goroutine-safe). The TUI keeps
	// rendering and exits from workflowDoneMsgCmd once the workflow completes.
	m.workflow.CancelAsync()

	return m.footer.Notification().SetPersistent(
		"Quitting, waiting for running commands to finish... (ctrl+q to force quit)",
		m.conf.ColorScheme.Status.Warning.GetForeground(),
	)
}

func (m *model) handleForceQuit() zeroterm.Cmd {
	m.quitting = true

	// Cancel synchronously so pond tasks stop writing fleet state before the
	// final stdout render reads it.
	if m.workflow != nil {
		m.workflow.CancelAsync()
	}

	m.setFailedMachinesErrorIfNil()

	return m.quitCmd()
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
	m.cachedTree.InvalidateCache()

	return nil
}

func (m *model) handleToggleCommands() zeroterm.Cmd {
	m.conf.Flags.Tui.ShowCommandsInLabels = !m.conf.Flags.Tui.ShowCommandsInLabels
	m.cachedTree.InvalidateCache()

	return nil
}

func (m *model) handleToggleActiveOnly() zeroterm.Cmd {
	m.conf.Flags.Tui.ShowActiveOnly = !m.conf.Flags.Tui.ShowActiveOnly
	m.cachedTree.InvalidateCache()

	return nil
}

func (m *model) handleRetry() zeroterm.Cmd {
	if m.workflow == nil {
		return nil
	}

	if m.conf.Flags.Snapshot.OnRetry {
		m.captureSnapshot(config.SnaphsotReasonRetry)
	}

	notifCmd := m.footer.Notification().Set("Retrying failed...", m.conf.ColorScheme.Status.OK.GetForeground())

	return zeroterm.BatchCmd(notifCmd, retryCmd)
}

func (m *model) handleRestart() zeroterm.Cmd {
	// The notification cmd also dispatches the restartMsg as a side effect,
	// so both actions are triggered from a single cmd.
	notifCmd := m.footer.Notification().Set("Restarting workflow...", m.conf.ColorScheme.Status.OK.GetForeground())

	return zeroterm.BatchCmd(notifCmd, restartCmd)
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

func (m *model) handleEsc() zeroterm.Cmd {
	switch {
	case m.viewports.IsFullscreen():
		m.viewports.ExitFullscreen()
	case m.viewports.HasActiveInner():
		m.viewports.DeselectAll()
	}

	return nil
}

func (m *model) handleSnapshot() zeroterm.Cmd {
	m.captureSnapshot(config.SnaphsotReasonManual)

	return m.footer.Notification().Set("Snapshot saved", m.conf.ColorScheme.Status.OK.GetForeground())
}

func (m *model) captureSnapshot(reason config.SnaphsotReason) {
	if m.workflow == nil {
		return
	}

	snap := snapshot.Capture(m.conf, reason, m.err)

	err := snapshot.Write(m.conf.Flags.Snapshot.Dir, snap)
	if err != nil {
		log.Error().Err(err).Msg("failed to write snapshot")
	}
}
