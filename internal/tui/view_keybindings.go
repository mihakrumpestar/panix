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
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_notifications"
	"github.com/pkg/errors"
	zerolog "github.com/rs/zerolog/log"
)

var (
	notificationBaseStyle = lipgloss.NewStyle().Bold(true)
	centerStyleCache      = make(map[int]lipgloss.Style)
	keymapHelp            = help.New()
)

func init() {
	keymapHelp.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	keymapHelp.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
}

// NotificationMsg is an alias for tui_notifications.Msg for backward compatibility.
// Deprecated: Use tui_notifications.Msg directly.
type NotificationMsg = tui_notifications.Msg

// keyDef defines a single key binding
type keyDef struct {
	keys    []string
	help    string
	handler func(*model) (tea.Model, tea.Cmd)
}

// keyDefs is the single source of truth for all key bindings
var keyDefs = []keyDef{
	{[]string{"q"}, "quit", (*model).handleQuit},
	{[]string{"h"}, "toggle logs", (*model).handleToggle},
	{[]string{"r"}, "retry", (*model).handleRetry},
	{[]string{"ctrl+r"}, "restart", (*model).handleRestart},
	{[]string{"ctrl+c"}, "copy", (*model).handleCopy},
	{[]string{"m"}, "fullscreen", (*model).handleFullscreen},
}

// pre-initialized bindings for help view (populated in init)
var keyBindings []key.Binding

// Keymap implements help.KeyMap for bubble tea help component
type Keymap struct{}

// ShortHelp returns key bindings for the short help view
func (k Keymap) ShortHelp() []key.Binding {
	return keyBindings
}

// FullHelp returns key bindings for the full help view
func (k Keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{keyBindings}
}

func init() {
	keyBindings = make([]key.Binding, len(keyDefs))
	for i, kd := range keyDefs {
		keyBindings[i] = key.NewBinding(
			key.WithKeys(kd.keys...),
			key.WithHelp(kd.keys[0], kd.help),
		)
	}
}

// HandleKeyInput routes key events to their handlers
func (m *model) HandleKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		return m.handleEsc()
	}

	for _, kd := range keyDefs {
		if slices.Contains(kd.keys, msg.String()) {
			return kd.handler(m)
		}
	}

	return m, nil
}

// ViewKeybindings renders the footer with help, notification, and scroll percent
func (m *model) ViewKeybindings(builderAbove strings.Builder) string {
	const footerHeight = 3
	width := m.dimensions.Width

	// Use bubbles help library for consistent styling
	helpContent := keymapHelp.View(Keymap{})
	helpContent = lipgloss.NewStyle().MaxWidth(width / 2).Render(helpContent)
	helpContent = centerVertically(helpContent, footerHeight)

	// Notification (if any)
	notificationContent := m.notification.Render(notificationBaseStyle)
	var notifWidth int
	var notificationBox string
	if notificationContent != "" {
		notificationBox = lipgloss.NewStyle().
			Padding(0, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(m.notification.GetColor().GetForeground()).
			Inherit(m.notification.GetColor()).
			Render(notificationContent)
		notificationBox = centerVertically(notificationBox, footerHeight)
		notifWidth = lipgloss.Width(notificationBox)
	}

	// Scroll percent
	scrollPercent := renderScrollPercent(m)
	scrollPercent = centerVertically(scrollPercent, footerHeight)
	scrollWidth := lipgloss.Width(scrollPercent)

	// Calculate spacing
	middleWidth := width - lipgloss.Width(helpContent) - notifWidth - scrollWidth
	if middleWidth < 0 {
		middleWidth = 0
	}

	// Assemble footer
	parts := []string{helpContent}
	if middleWidth > 0 {
		parts = append(parts, lipgloss.NewStyle().Width(middleWidth).Render(""))
	}
	if notificationBox != "" {
		parts = append(parts, notificationBox)
	}
	parts = append(parts, scrollPercent)

	return "\n" + lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// Handlers

func (m *model) handleCopy() (tea.Model, tea.Cmd) {
	content, isInner := m.resetable.viewports.GetActiveInnerViewportContent()
	if !isInner {
		return m.notification.SetModel(m, "Select an inner viewport to copy", m.conf.ColorScheme.StatusWarning)
	}
	if content == "" {
		return m.notification.SetModel(m, "No content to copy", m.conf.ColorScheme.StatusWarning)
	}
	if err := tui_clipboard.CopyToClipboard(content); err != nil {
		return m.notification.SetModel(m, "Copy failed: "+err.Error(), m.conf.ColorScheme.StatusError)
	}
	return m.notification.SetModel(m, "Copied to clipboard", m.conf.ColorScheme.StatusOK)
}

func (m *model) handleQuit() (tea.Model, tea.Cmd) {
	m.quitting = true
	err := m.resetable.workflow.Cancel()
	if m.resetable.err != nil && err != nil {
		if err != context.Canceled {
			m.resetable.err = errors.Wrap(m.resetable.err, err.Error())
		}
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

func (m *model) handleRetry() (tea.Model, tea.Cmd) {
	m.resetable.workflow.State().Retry.Trigger()
	return m, nil
}

func (m *model) handleRestart() (tea.Model, tea.Cmd) {
	return m, tea.Batch(
		m.notification.SetCmd("Restarting workflow...", m.conf.ColorScheme.StatusOK),
		func() tea.Msg {
			return restartMsg{}
		},
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
		return m.notification.SetModel(m, "Select a viewport first", m.conf.ColorScheme.StatusWarning)
	}
	return m, nil
}

func (m *model) handleEsc() (tea.Model, tea.Cmd) {
	if m.resetable.viewports.IsFullscreen() {
		m.resetable.viewports.ExitFullscreen()
	} else {
		m.resetable.viewports.DeselectAll()
	}
	return m, nil
}

// Utility functions

func renderScrollPercent(m *model) string {
	pct := m.resetable.viewports.GetActiveViewportScrollPercent()
	pctInt := int(pct * 100)
	return lipgloss.NewStyle().
		Foreground(m.conf.ColorScheme.TableBorder.GetForeground()).
		Render(fmt.Sprintf("%d%%", pctInt))
}

func centerVertically(content string, height int) string {
	contentHeight := lipgloss.Height(content)
	if contentHeight >= height {
		return content
	}
	paddingTop := (height - contentHeight) / 2
	paddingBottom := height - contentHeight - paddingTop
	cacheKey := paddingTop<<16 | paddingBottom
	if style, ok := centerStyleCache[cacheKey]; ok {
		return style.Render(content)
	}
	style := lipgloss.NewStyle().PaddingTop(paddingTop).PaddingBottom(paddingBottom)
	centerStyleCache[cacheKey] = style
	return style.Render(content)
}
