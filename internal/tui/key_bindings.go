package tui

import (
	"bytes"
	"slices"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/snapshot"
	"github.com/mihakrumpestar/panix/internal/tui/footer"
	"github.com/mihakrumpestar/panix/pkg/clipboard"
	"github.com/mihakrumpestar/panix/pkg/tui/notification"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/rs/zerolog/log"
)

// Quitting key metadata, the single source of truth for the quitting keys and
// their footer help texts. The global keymap and the theme picker footer share
// it so both always show the same keys and wording.
const (
	quitKey       = "q"
	quitHelp      = "quit"
	forceQuitKey  = "ctrl+q"
	forceQuitHelp = "force quit"
)

// quittingKeyDefs builds the quitting bindings shared by the global keymap and
// the theme picker footer (which strips the handlers).
func (m *model) quittingKeyDefs() []footer.KeyDef {
	return []footer.KeyDef{
		{Keys: []string{quitKey}, Help: quitHelp, Handler: m.handleQuit},
		{Keys: []string{forceQuitKey}, Help: forceQuitHelp, Handler: m.handleForceQuit},
	}
}

// displayOnlyKeyDefs clears the handlers so the defs become display-only modal
// footer entries, never dispatched (see isKeyDisabled). defs is modified in
// place and returned.
func displayOnlyKeyDefs(defs []footer.KeyDef) []footer.KeyDef {
	for i := range defs {
		defs[i].Handler = nil
	}

	return defs
}

//nolint:lll
func (m *model) keyDefs() []footer.KeyDef {
	kds := m.quittingKeyDefs()
	kds = append(kds, []footer.KeyDef{
		{Keys: []string{"h"}, Help: "toggle inspect/secrets logs", Active: func() bool { return m.conf.Flags.Tui.ShowAllBuildLogs }, Handler: m.handleToggle},
		{Keys: []string{"a"}, Help: "toggle active only", Active: func() bool { return m.conf.Flags.Tui.ShowActiveOnly }, Handler: m.handleToggleActiveOnly},
		{Keys: []string{"c"}, Help: "toggle descriptions/commands", Active: func() bool { return m.conf.Flags.Tui.ShowCommandsInLabels }, Handler: m.handleToggleCommands},
		{Keys: []string{"ctrl+c"}, Help: "copy", Handler: m.handleCopy},
		{Keys: []string{"m"}, Help: "fullscreen", Active: func() bool { return m.viewports.IsFullscreen() }, Handler: m.handleFullscreen},
		{Keys: []string{"ctrl+t"}, Help: "themes", Active: func() bool { return m.themePicker.open }, Handler: m.handleThemePickerToggle},
	}...)

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

	// While quitting only ctrl+q stays usable; every other key is disabled and
	// hidden. Centralized here because e.g. ctrl+r while quitting would call the
	// blocking workflow.Cancel() inside the event loop (re-freezing the TUI) and
	// the leftover quitting flag would kill the restarted run on its first done
	// message.
	for i := range kds {
		if slices.Contains(kds[i].Keys, forceQuitKey) {
			continue
		}

		kds[i].Disabled = func() bool { return m.quitting }
	}

	return kds
}

// modalKeyDefs returns the footer bindings for the modal context: the modal's
// display-only bindings while an overlay owns the keyboard, the global bindings
// otherwise. The picker appends the shared quitting bindings display-only: the
// picker does not block quitting and the footer keeps showing them.
func (m *model) modalKeyDefs(kind modalKind) []footer.KeyDef {
	switch kind {
	case modalThemePicker:
		return append([]footer.KeyDef{
			{Keys: []string{"enter"}, Help: "apply"},
			{Keys: []string{"up/down"}, Help: "select"},
			{Keys: []string{"esc"}, Help: "cancel"},
		}, displayOnlyKeyDefs(m.quittingKeyDefs())...)
	default:
		return m.keyDefs()
	}
}

// syncFooterKeyDefs swaps the footer keymap to the modal context. Runs at the
// top of Update (so dispatch never sees display-only bindings) and in Render
// (so the footer is correct in the frame that closed a modal).
func (m *model) syncFooterKeyDefs() {
	kind := m.modal()
	if kind == m.footerKeyCtx {
		return
	}

	m.footerKeyCtx = kind
	m.footer.SetKeyDefs(m.modalKeyDefs(kind))
}

// navigable is the contract for components that respond to left/right
// navigation and can be deselected.
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

	// Try the active component first. When nothing is active the stats table
	// gets priority; at a boundary the key spills over to the other component.
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

// isKeyDisabled reports whether a key binding is unusable. A binding without a
// handler is display-only (modal help) and never dispatched.
func (m *model) isKeyDisabled(keyDef footer.KeyDef) bool {
	return keyDef.Handler == nil || (keyDef.Disabled != nil && keyDef.Disabled())
}

func (m *model) handleCopy() zeroterm.Cmd {
	if m.workflow == nil {
		return nil
	}

	content, isInner := m.viewports.GetActiveInnerViewportContent()
	if !isInner {
		return m.footer.Notification().Set(notification.LevelWarning, "Select an inner viewport to copy")
	}

	if len(content) == 0 {
		return m.footer.Notification().Set(notification.LevelWarning, "No content to copy")
	}

	err := clipboard.CopyToClipboard(string(bytes.Join(content, []byte("\n"))))
	if err != nil {
		return m.footer.Notification().Set(notification.LevelError, "Copy failed: "+err.Error())
	}

	return m.footer.Notification().Set(notification.LevelSuccess, "Copied to clipboard")
}

// quitCmd centralizes TUI exit: captures the exit snapshot when enabled, logs
// and returns the quit command. Every exit site routes through here so the
// snapshot is taken exactly once per exit.
func (m *model) quitCmd() zeroterm.Cmd {
	if m.conf.Flags.Snapshot.OnExit {
		m.captureSnapshot(config.SnapshotReasonExit)
	}

	log.Debug().Msg("Context done, exiting TUI")

	return zeroterm.QuitCmd
}

func (m *model) handleQuit() zeroterm.Cmd {
	if m.quitting {
		return nil
	}

	m.quitting = true

	// Without a workflow (or when it already finished) no workflowDoneMsg will
	// arrive again: quit immediately.
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

	// Cancel asynchronously: Cancel() blocks until every command finishes, which
	// would freeze the zeroterm event loop. CancelAsync is non-blocking and
	// idempotent; the TUI exits from workflowDoneMsgCmd once the workflow ends.
	m.workflow.CancelAsync()

	return m.footer.Notification().SetPersistent(notification.LevelWarning, "Quitting, waiting for running commands to finish... (ctrl+q to force quit)")
}

func (m *model) handleForceQuit() zeroterm.Cmd {
	m.quitting = true

	// Cancel before the final stdout render reads fleet state.
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
		m.captureSnapshot(config.SnapshotReasonRetry)
	}

	notifCmd := m.footer.Notification().Set(notification.LevelSuccess, "Retrying failed...")

	return zeroterm.BatchCmd(notifCmd, retryCmd)
}

func (m *model) handleRestart() zeroterm.Cmd {
	// The notification cmd also dispatches restartMsg as a side effect, so both
	// actions are triggered from a single cmd.
	notifCmd := m.footer.Notification().Set(notification.LevelSuccess, "Restarting workflow...")

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
		return m.footer.Notification().Set(notification.LevelWarning, "Select a viewport first")
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
	m.captureSnapshot(config.SnapshotReasonManual)

	return m.footer.Notification().Set(notification.LevelSuccess, "Snapshot saved")
}

func (m *model) captureSnapshot(reason config.SnapshotReason) {
	if m.workflow == nil {
		return
	}

	snap := snapshot.Capture(m.conf, reason, m.err)

	err := snapshot.Write(m.conf.Flags.Snapshot.Dir, snap)
	if err != nil {
		log.Error().Err(err).Msg("failed to write snapshot")
	}
}
