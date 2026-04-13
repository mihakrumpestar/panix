package tui

import (
	"context"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/clipboard"
	"github.com/mihakrumpestar/panix/internal/snapshot"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type keyDef struct {
	keys    []string
	help    string
	handler func(*model) (tea.Model, tea.Cmd)
}

var keyDefs = []keyDef{
	{[]string{"q"}, "quit", (*model).handleQuit},
	{[]string{"h"}, "toggle inspect/secrets logs", (*model).handleToggle},
	{[]string{"a"}, "toggle active only", (*model).handleToggleActiveOnly},
	{[]string{"c"}, "toggle descriptions/commands", (*model).handleToggleCommands},
	{[]string{"r"}, "retry", (*model).handleRetry},
	{[]string{"ctrl+r"}, "restart", (*model).handleRestart},
	{[]string{"ctrl+c"}, "copy", (*model).handleCopy},
	{[]string{"m"}, "fullscreen", (*model).handleFullscreen},
	{[]string{"s"}, "snapshot", (*model).handleSnapshot},
}

var notificationBaseStyle = lipgloss.NewStyle().Bold(true)

type Footer struct {
	helpCache   string
	width       int
	keymapHelp  help.Model
	keyBindings []key.Binding
}

func newFooter() *Footer {
	bindings := make([]key.Binding, len(keyDefs))
	for i := range keyDefs {
		bindings[i] = key.NewBinding(
			key.WithKeys(keyDefs[i].keys...),
			key.WithHelp(keyDefs[i].keys[0], keyDefs[i].help),
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

func (f *Footer) Keymap() Keymap { return Keymap{bindings: f.keyBindings} }

type Keymap struct {
	bindings []key.Binding
}

func (k Keymap) ShortHelp() []key.Binding  { return k.bindings }
func (k Keymap) FullHelp() [][]key.Binding { return [][]key.Binding{k.bindings} }

func (m *model) ViewFooter() string {
	notifBox, notifWidth := m.notification.RenderBox(notificationBaseStyle)

	if m.footer.helpCache == "" || m.footer.width != m.dimensions.Width {
		m.footer.helpCache = "\n\n" + wrapKeybindingsByPair(m.footer.keymapHelp, m.footer.Keymap(), m.dimensions.Width)
		m.footer.width = m.dimensions.Width
	}

	styledHelp := lipgloss.NewStyle().Width(m.dimensions.Width).MaxWidth(max(m.dimensions.Width-notifWidth, 1)).Render(m.footer.helpCache)

	parts := []string{styledHelp}
	if notifWidth != 0 {
		parts = append(parts, "\n"+notifBox)
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

func (m *model) HandleKeyInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		return m.handleEsc()
	}

	hasActiveInner := m.resetable.Load().viewports.HasActiveInner()

	statsTable := m.conf.Fleet.StatsTable
	if statsTable.HandleNavigation(msg.String(), hasActiveInner) {
		statsTable.Reset()

		return m, nil
	}

	phaseStatus := m.conf.Fleet.PhaseStatus
	if phaseStatus.HandleNavigation(msg.String(), hasActiveInner) {
		phaseStatus.Reset()

		return m, nil
	}

	for _, kd := range keyDefs {
		if slices.Contains(kd.keys, msg.String()) {
			return kd.handler(m)
		}
	}

	return m, nil
}

func (m *model) handleCopy() (tea.Model, tea.Cmd) {
	resetable := m.resetable.Load()

	content, isInner := resetable.viewports.GetActiveInnerViewportContent()
	if !isInner {
		return m, m.notification.Set("Select an inner viewport to copy", m.conf.ColorScheme.Status.Warning)
	}

	if content == "" {
		return m, m.notification.Set("No content to copy", m.conf.ColorScheme.Status.Warning)
	}

	err := clipboard.CopyToClipboard(content)
	if err != nil {
		return m, m.notification.Set("Copy failed: "+err.Error(), m.conf.ColorScheme.Status.Failed)
	}

	return m, m.notification.Set("Copied to clipboard", m.conf.ColorScheme.Status.OK)
}

func (m *model) handleQuit() (tea.Model, tea.Cmd) {
	if m.conf.Flags.Snapshot.OnExit {
		m.captureSnapshot(config.SnaphsotReasonExit)
	}

	m.quitting = true
	resetable := m.resetable.Load()

	err := resetable.workflow.Cancel()
	if resetable.err != nil && err != nil && !errors.Is(err, context.Canceled) {
		resetable.err = errors.Wrap(resetable.err, err.Error())
	} else if err != nil && !errors.Is(err, context.Canceled) {
		resetable.err = err
	}

	log.Debug().Msg("Context done, exiting TUI")

	return m, tea.Quit
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
	if m.conf.Flags.Snapshot.OnRetry {
		m.captureSnapshot(config.SnaphsotReasonRetry)
	}

	m.resetable.Load().workflow.State().Retry.Trigger()

	return m, nil
}

func (m *model) handleRestart() (tea.Model, tea.Cmd) {
	return m, tea.Batch(
		m.notification.Set("Restarting workflow...", m.conf.ColorScheme.Status.OK),
		func() tea.Msg { return restartMsg{} },
	)
}

func (m *model) handleFullscreen() (tea.Model, tea.Cmd) {
	resetable := m.resetable.Load()
	if resetable.viewports.IsFullscreen() {
		resetable.viewports.ExitFullscreen()

		return m, nil
	}

	activeInnerXpath := resetable.viewports.GetActiveInnerViewportXpath()

	if activeInnerXpath.Depth() > 0 {
		resetable.viewports.SetFullscreen(activeInnerXpath)
	} else {
		return m, m.notification.Set("Select a viewport first", m.conf.ColorScheme.Status.Warning)
	}

	return m, nil
}

func (m *model) handleEsc() (tea.Model, tea.Cmd) {
	resetable := m.resetable.Load()

	statsTable := m.conf.Fleet.StatsTable
	phaseStatus := m.conf.Fleet.PhaseStatus

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

	return m, m.notification.Set("Snapshot saved", m.conf.ColorScheme.Status.OK)
}

func (m *model) captureSnapshot(reason config.SnaphsotReason) {
	resetable := m.resetable.Load()
	if resetable == nil || resetable.workflow == nil {
		return
	}

	snap := snapshot.Capture(m.conf, reason, resetable.err)

	err := snapshot.Write(m.conf.Flags.Snapshot.Dir, snap)
	if err != nil {
		log.Error().Err(err).Msg("failed to write snapshot")
	}
}

func wrapKeybindingsByPair(helpModel help.Model, keyMap help.KeyMap, maxWidth int) string {
	bindings := keyMap.ShortHelp()
	if len(bindings) == 0 {
		return ""
	}

	separator := helpModel.Styles.ShortSeparator.Inline(true).Render(helpModel.ShortSeparator)
	sepWidth := lipgloss.Width(separator)

	var (
		lines        []string
		currentLine  strings.Builder
		currentWidth int
	)

	for _, binding := range bindings {
		if !binding.Enabled() {
			continue
		}

		item := helpModel.Styles.ShortKey.Inline(true).Render(binding.Help().Key) + " " +
			helpModel.Styles.ShortDesc.Inline(true).Render(binding.Help().Desc)
		itemWidth := lipgloss.Width(item)

		if currentWidth > 0 && currentWidth+sepWidth+itemWidth > maxWidth {
			lines = append(lines, currentLine.String())
			currentLine.Reset()

			currentWidth = 0
		}

		if currentWidth > 0 {
			currentLine.WriteString(separator)

			currentWidth += sepWidth
		}

		currentLine.WriteString(item)

		currentWidth += itemWidth
	}

	if currentWidth > 0 {
		lines = append(lines, currentLine.String())
	}

	return strings.Join(lines, "\n")
}
