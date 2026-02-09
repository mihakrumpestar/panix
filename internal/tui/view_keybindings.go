package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_clipboard"
	"github.com/pkg/errors"
)

var (
	keymapHelp = func() help.Model {
		h := help.New()
		h.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
		h.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
		return h
	}()
)

// notificationMsg is sent when the notification should fade out
type notificationMsg struct{}

// keymap defines all the key bindings for the TUI
type keymap struct {
	quit       key.Binding
	toggle     key.Binding
	retry      key.Binding
	copy       key.Binding
	fullscreen key.Binding
}

// ShortHelp returns the key bindings to show in the short help view
func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{k.quit, k.toggle, k.retry, k.copy, k.fullscreen}
}

// FullHelp returns the key bindings to show in the full help view
func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.quit, k.copy},
		{k.toggle, k.retry},
		{k.fullscreen},
	}
}

// newKeymap creates a new keymap with all the necessary key bindings
func newKeymap() keymap {
	return keymap{
		quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		toggle: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "show/hide hidden logs"),
		),
		retry: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "retry failed"),
		),
		copy: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "copy viewport"),
		),
		fullscreen: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "toggle fullscreen"),
		),
	}
}

// HandleKeyInput handles key input using the keymap and returns the updated model and command
func (m *model) HandleKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keys := newKeymap()

	switch {
	case key.Matches(msg, keys.copy):
		return m.handleCopy()
	case key.Matches(msg, keys.quit):
		return m.handleQuit()
	case key.Matches(msg, keys.toggle):
		return m.handleToggle()
	case key.Matches(msg, keys.retry):
		return m.handleRetry()
	case key.Matches(msg, keys.fullscreen):
		return m.handleFullscreen()
	case msg.String() == "esc":
		return m.handleEsc()
	}

	return m, nil
}

func (m *model) handleCopy() (tea.Model, tea.Cmd) {
	content, isInner := m.modelView.viewports.GetActiveInnerViewportContent()
	if !isInner {
		return m.setNotification("Select an inner viewport to copy", m.workflow.State().Conf.ColorScheme.StatusWarning)
	}

	if content == "" {
		return m.setNotification("No content to copy", m.workflow.State().Conf.ColorScheme.StatusWarning)
	}

	if err := tui_clipboard.CopyToClipboard(content); err != nil {
		return m.setNotification("Copy failed: "+err.Error(), m.workflow.State().Conf.ColorScheme.StatusError)
	}

	return m.setNotification("Copied to clipboard", m.workflow.State().Conf.ColorScheme.StatusOK)
}

func (m *model) handleQuit() (tea.Model, tea.Cmd) {
	m.quitting = true
	m.workflow.Cancel()()

	ctx := m.workflow.Ctx()
	<-ctx.Done()

	if m.err != nil && ctx.Err() != nil {
		if ctx.Err() != context.Canceled {
			m.err = errors.Wrap(m.err, ctx.Err().Error())
		}
	} else if ctx.Err() != nil && ctx.Err() != context.Canceled {
		m.err = ctx.Err()
	}

	m.modelView.debugOutput.WriteString("CTX done\n")

	return m, tea.Quit
}

func (m *model) handleToggle() (tea.Model, tea.Cmd) {
	state := m.workflow.State()
	state.Conf.Flags.Tui.ShowAllBuildLogs = !state.Conf.Flags.Tui.ShowAllBuildLogs
	return m, nil
}

func (m *model) handleRetry() (tea.Model, tea.Cmd) {
	m.workflow.State().Retry.Trigger()
	return m, nil
}

func (m *model) handleFullscreen() (tea.Model, tea.Cmd) {
	if m.modelView.viewports.IsFullscreen() {
		m.modelView.viewports.ExitFullscreen()
		return m, nil
	}

	activeInnerXpath := m.modelView.viewports.GetActiveInnerViewportXpath()
	if activeInnerXpath.Depth() > 0 {
		m.modelView.viewports.SetFullscreen(activeInnerXpath)
	} else {
		return m.setNotification("Select a viewport first", m.workflow.State().Conf.ColorScheme.StatusWarning)
	}

	return m, nil
}

func (m *model) handleEsc() (tea.Model, tea.Cmd) {
	if m.modelView.viewports.IsFullscreen() {
		m.modelView.viewports.ExitFullscreen()
	} else {
		m.modelView.viewports.DeselectAll()
	}
	return m, nil
}

// setNotification sets a notification message with color and starts the timer
func (m *model) setNotification(text string, color lipgloss.Style) (tea.Model, tea.Cmd) {
	m.notification = text
	m.notificationColor = color
	m.notificationTime = time.Now()
	return m, m.notificationTimer()
}

// notificationTimer returns a command that waits for the notification duration
func (m *model) notificationTimer() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return notificationMsg{}
	})
}

// clearNotification clears the notification
func (m *model) clearNotification() {
	m.notification = ""
	m.notificationTime = time.Time{}
}

// renderNotification renders the notification with fade effect based on time elapsed
func (m *model) renderNotification() string {
	if m.notification == "" {
		return ""
	}

	elapsed := time.Since(m.notificationTime)
	duration := 3 * time.Second
	fadeDuration := time.Second

	if elapsed > duration {
		return ""
	}

	inFade := elapsed > duration-fadeDuration

	style := lipgloss.NewStyle().Bold(true)
	if inFade {
		style = style.Faint(true)
	}

	return style.Inherit(m.notificationColor).Render(m.notification)
}

func renderScrollPercent(m *model) string {
	pct := m.modelView.viewports.GetActiveViewportScrollPercent()
	pctInt := int(pct * 100)
	return lipgloss.NewStyle().
		Foreground(m.workflow.State().Conf.ColorScheme.TableBorder.GetForeground()).
		Render(fmt.Sprintf("%d%%", pctInt))
}

// centerVertically centers content vertically within a given height
func centerVertically(content string, height int) string {
	contentHeight := lipgloss.Height(content)
	if contentHeight >= height {
		return content
	}
	paddingTop := (height - contentHeight) / 2
	paddingBottom := height - contentHeight - paddingTop
	return lipgloss.NewStyle().
		PaddingTop(paddingTop).
		PaddingBottom(paddingBottom).
		Render(content)
}

// ViewKeybindings returns the help view with key bindings positioned at the bottom
func (m *model) ViewKeybindings(builderAbove strings.Builder) string {
	const footerHeight = 3
	footerWidth := m.modelView.dimensions.Width

	helpContent := keymapHelp.View(newKeymap())
	notificationContent := m.renderNotification()
	scrollPercentContent := renderScrollPercent(m)

	// Prepare notification box if present
	var notificationBox string
	var notifWidth int
	if notificationContent != "" {
		notificationBox = lipgloss.NewStyle().
			Padding(0, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(m.notificationColor.GetForeground()).
			Inherit(m.notificationColor).
			Render(notificationContent)
		notificationBox = centerVertically(notificationBox, footerHeight)
		notifWidth = lipgloss.Width(notificationBox) + 1
	}

	scrollPercentWidth := lipgloss.Width(scrollPercentContent)

	// Calculate available width for help
	availableWidth := footerWidth - notifWidth - scrollPercentWidth - 2
	if availableWidth < 0 {
		availableWidth = 0
	}

	// Truncate help if needed and center vertically
	if availableWidth > 0 {
		helpContent = lipgloss.NewStyle().MaxWidth(availableWidth).Render(helpContent)
	}
	helpContent = centerVertically(helpContent, footerHeight)
	scrollPercentContent = centerVertically(scrollPercentContent, footerHeight)

	// Calculate middle spacing
	middleWidth := footerWidth - lipgloss.Width(helpContent) - notifWidth - scrollPercentWidth
	if middleWidth < 0 {
		middleWidth = 0
	}

	// Build footer row
	var parts []string
	parts = append(parts, helpContent)
	if middleWidth > 0 {
		parts = append(parts, lipgloss.NewStyle().Width(middleWidth).Render(""))
	}
	if notificationBox != "" {
		parts = append(parts, notificationBox)
	}
	parts = append(parts, scrollPercentContent)

	return "\n" + lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
