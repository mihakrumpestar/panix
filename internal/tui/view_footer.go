package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_clipboard"
	"github.com/pkg/errors"
	zerolog "github.com/rs/zerolog/log"
)

var (
	notificationBaseStyle = lipgloss.NewStyle().Bold(true)
	keymapHelp            = help.New()
)

func init() {
	keymapHelp.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	keymapHelp.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	keyBindings = make([]key.Binding, len(keyDefs))
	for i, kd := range keyDefs {
		keyBindings[i] = key.NewBinding(
			key.WithKeys(kd.keys...),
			key.WithHelp(kd.keys[0], kd.help),
		)
	}
}

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
}

var keyBindings []key.Binding

type Keymap struct{}

func (k Keymap) ShortHelp() []key.Binding  { return keyBindings }
func (k Keymap) FullHelp() [][]key.Binding { return [][]key.Binding{keyBindings} }

func (m *model) HandleKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		return m.handleEsc()
	}

	if m.resetable.statsTable.HandleNavigation(msg.String(), m.resetable.viewports.HasActiveInner()) {
		return m, nil
	}

	for _, kd := range keyDefs {
		if slices.Contains(kd.keys, msg.String()) {
			return kd.handler(m)
		}
	}
	return m, nil
}

func (m *model) ViewFooter() string {
	width := m.dimensions.Width
	scrollPercent := renderScrollPercent(m)
	scrollWidth := lipgloss.Width(scrollPercent)

	notificationBox, notificationBoxWidth := m.notification.RenderBox(notificationBaseStyle)

	helpWidth := max(width-scrollWidth, 1)
	helpContent := "\n\n" + wrapKeybindingsByPair(keymapHelp, Keymap{}, helpWidth)
	helpContent = lipgloss.NewStyle().Width(helpWidth).MaxWidth(max(helpWidth-notificationBoxWidth, 1)).Render(helpContent)

	parts := []string{helpContent}
	if notificationBoxWidth != 0 {
		parts = append(parts, "\n"+notificationBox)
	}
	parts = append(parts, scrollPercent)

	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

// Handlers

func (m *model) handleCopy() (tea.Model, tea.Cmd) {
	content, isInner := m.resetable.viewports.GetActiveInnerViewportContent()
	if !isInner {
		return m, m.notification.Set("Select an inner viewport to copy", m.conf.ColorScheme.StatusWarning)
	}
	if content == "" {
		return m, m.notification.Set("No content to copy", m.conf.ColorScheme.StatusWarning)
	}
	if err := tui_clipboard.CopyToClipboard(content); err != nil {
		return m, m.notification.Set("Copy failed: "+err.Error(), m.conf.ColorScheme.StatusError)
	}
	return m, m.notification.Set("Copied to clipboard", m.conf.ColorScheme.StatusOK)
}

func (m *model) handleQuit() (tea.Model, tea.Cmd) {
	m.quitting = true
	err := m.resetable.workflow.Cancel()
	if m.resetable.err != nil && err != nil && err != context.Canceled {
		m.resetable.err = errors.Wrap(m.resetable.err, err.Error())
	} else if err != nil && err != context.Canceled {
		m.resetable.err = err
	}
	zerolog.Debug().Msg("Context done, exiting TUI")
	return m, tea.Sequence(tea.ExitAltScreen, tea.Quit)
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
	m.resetable.workflow.State().Retry.Trigger()
	return m, nil
}

func (m *model) handleRestart() (tea.Model, tea.Cmd) {
	return m, tea.Batch(
		m.notification.Set("Restarting workflow...", m.conf.ColorScheme.StatusOK),
		func() tea.Msg { return restartMsg{} },
	)
}

func (m *model) handleFullscreen() (tea.Model, tea.Cmd) {
	if m.resetable.viewports.IsFullscreen() {
		m.resetable.viewports.ExitFullscreen()
		return m, nil
	}
	activeInnerXpath := m.resetable.viewports.GetActiveInnerViewportXpath()
	if activeInnerXpath.Depth() > 0 {
		m.resetable.viewports.SetFullscreen(activeInnerXpath)
	} else {
		return m, m.notification.Set("Select a viewport first", m.conf.ColorScheme.StatusWarning)
	}
	return m, nil
}

func (m *model) handleEsc() (tea.Model, tea.Cmd) {
	if m.resetable.viewports.IsFullscreen() {
		m.resetable.viewports.ExitFullscreen()
	} else if m.resetable.viewports.HasActiveInner() {
		m.resetable.viewports.DeselectAll()
	} else {
		m.resetable.statsTable.Reset()
	}
	return m, nil
}

// Utility functions

func wrapKeybindingsByPair(h help.Model, k help.KeyMap, maxWidth int) string {
	bindings := k.ShortHelp()
	if len(bindings) == 0 {
		return ""
	}

	separator := h.Styles.ShortSeparator.Inline(true).Render(h.ShortSeparator)
	sepWidth := lipgloss.Width(separator)
	var lines []string
	var currentLine strings.Builder
	var currentWidth int

	for _, kb := range bindings {
		if !kb.Enabled() {
			continue
		}

		item := h.Styles.ShortKey.Inline(true).Render(kb.Help().Key) + " " +
			h.Styles.ShortDesc.Inline(true).Render(kb.Help().Desc)
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

func renderScrollPercent(m *model) string {
	pct := m.resetable.viewports.GetActiveViewportScrollPercent()
	return lipgloss.NewStyle().PaddingLeft(1).
		Foreground(m.conf.ColorScheme.TableBorder.GetForeground()).
		Render(fmt.Sprintf("%d%%", int(pct*100)))
}
