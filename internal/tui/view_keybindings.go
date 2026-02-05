package tui

import (
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
		// Make keys white while keeping descriptions as default
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
		content, isInner := m.modelView.viewports.GetActiveInnerViewportContent()
		if !isInner {
			m.notification = "Select an inner viewport to copy"
			m.notificationColor = m.workflow.State().Conf.Tui.ColorScheme.StatusWarning
			m.notificationTime = time.Now()
			return m, m.notificationTimer()
		}
		if content != "" {
			err := tui_clipboard.CopyToClipboard(content)
			if err != nil {
				m.notification = "Copy failed: " + err.Error()
				m.notificationColor = m.workflow.State().Conf.Tui.ColorScheme.StatusError
			} else {
				m.notification = "Copied to clipboard"
				m.notificationColor = m.workflow.State().Conf.Tui.ColorScheme.StatusOK
			}
			m.notificationTime = time.Now()
			return m, m.notificationTimer()
		}
		m.notification = "No content to copy"
		m.notificationColor = m.workflow.State().Conf.Tui.ColorScheme.StatusWarning
		m.notificationTime = time.Now()
		return m, m.notificationTimer()
	case key.Matches(msg, keys.quit):
		m.quitting = true
		m.workflow.Cancel()()

		// Block quiting until Workflow finalizes and terminates its tasks
		ctx := m.workflow.Ctx()
		<-ctx.Done()

		if m.err != nil && ctx.Err() != nil {
			m.err = errors.Wrap(m.err, ctx.Err().Error())
		} else if ctx.Err() != nil {
			m.err = ctx.Err()
		}

		m.modelView.debugOutput.WriteString("CTX done\n")

		return m, tea.Quit
	case key.Matches(msg, keys.toggle):
		m.workflow.State().Conf.Tui.ShowAllBuildLogs = !m.workflow.State().Conf.Tui.ShowAllBuildLogs

		return m, nil
	case key.Matches(msg, keys.retry):
		m.workflow.State().Retry.Trigger()

		return m, nil
	case key.Matches(msg, keys.fullscreen):
		// Toggle fullscreen for the active inner viewport
		if m.modelView.viewports.IsFullscreen() {
			// Exit fullscreen
			m.modelView.viewports.ExitFullscreen()
		} else {
			// Enter fullscreen for active inner viewport
			activeInnerXpath := m.modelView.viewports.GetActiveInnerViewportXpath()
			if activeInnerXpath.Depth() > 0 {
				m.modelView.viewports.SetFullscreen(activeInnerXpath)
			} else {
				m.notification = "Select a viewport first"
				m.notificationColor = m.workflow.State().Conf.Tui.ColorScheme.StatusWarning
				m.notificationTime = time.Now()
				return m, m.notificationTimer()
			}
		}
		return m, nil
	case msg.String() == "esc":
		// ESC exits fullscreen mode if active, otherwise quits
		if m.modelView.viewports.IsFullscreen() {
			m.modelView.viewports.ExitFullscreen()
			return m, nil
		}
		// If not in fullscreen, fall through to quit behavior
		m.quitting = true
		m.workflow.Cancel()()
		ctx := m.workflow.Ctx()
		<-ctx.Done()
		if m.err != nil && ctx.Err() != nil {
			m.err = errors.Wrap(m.err, ctx.Err().Error())
		} else if ctx.Err() != nil {
			m.err = ctx.Err()
		}
		m.modelView.debugOutput.WriteString("CTX done\n")
		return m, tea.Quit
	}

	return m, nil
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
	fadeDuration := time.Second // 1 second fade for better visibility

	if elapsed > duration {
		return ""
	}

	// Check if we're in the fade period (last 1 second)
	inFade := elapsed > duration-fadeDuration

	// Apply visual fade using Faint during fade period
	style := lipgloss.NewStyle().Bold(true)
	if inFade {
		style = style.Faint(true)
	}

	return style.Inherit(m.notificationColor).Render(m.notification)
}

// renderScrollPercent renders the scroll percentage indicator
func (m *model) renderScrollPercent() string {
	pct := m.modelView.viewports.GetActiveViewportScrollPercent()
	pctInt := int(pct * 100)
	return lipgloss.NewStyle().
		Foreground(m.workflow.State().Conf.Tui.ColorScheme.TableBorder.GetForeground()).
		Render(fmt.Sprintf("%d%%", pctInt))
}

// ViewKeybindings returns the help view with key bindings positioned at the bottom
func (m *model) ViewKeybindings(builderAbove strings.Builder) string {
	footerWidth := m.modelView.dimensions.Width
	footerHeight := 3

	// Get help content
	helpContent := keymapHelp.View(newKeymap())

	// Get notification content
	notificationContent := m.renderNotification()

	// Get scroll percent
	scrollPercentContent := m.renderScrollPercent()
	scrollPercentWidth := lipgloss.Width(scrollPercentContent)

	// If no notification, create footer with scroll percent on the right
	if notificationContent == "" {
		// Calculate available width for help (accounting for scroll percent on the right)
		availableWidth := footerWidth - scrollPercentWidth - 2 // -2 for spacing
		if availableWidth < 0 {
			availableWidth = 0
		}

		// Truncate help to fit in available space
		if availableWidth > 0 {
			helpContent = lipgloss.NewStyle().MaxWidth(availableWidth).Render(helpContent)
		}

		// Center help content vertically
		helpHeight := lipgloss.Height(helpContent)
		if helpHeight < footerHeight {
			paddingTop := (footerHeight - helpHeight) / 2
			paddingBottom := footerHeight - helpHeight - paddingTop
			helpContent = lipgloss.NewStyle().
				PaddingTop(paddingTop).
				PaddingBottom(paddingBottom).
				Render(helpContent)
		}

		// Center scroll percent vertically
		scrollPercentHeight := lipgloss.Height(scrollPercentContent)
		if scrollPercentHeight < footerHeight {
			paddingTop := (footerHeight - scrollPercentHeight) / 2
			paddingBottom := footerHeight - scrollPercentHeight - paddingTop
			scrollPercentContent = lipgloss.NewStyle().
				PaddingTop(paddingTop).
				PaddingBottom(paddingBottom).
				Render(scrollPercentContent)
		}

		// Create footer row with scroll percent at the far right
		middleWidth := footerWidth - lipgloss.Width(helpContent) - scrollPercentWidth
		if middleWidth < 0 {
			middleWidth = 0
		}

		footerRow := lipgloss.JoinHorizontal(
			lipgloss.Top,
			helpContent,
			lipgloss.NewStyle().Width(middleWidth).Render(""),
			scrollPercentContent,
		)

		return "\n" + footerRow
	}

	// Create notification box with padding
	notificationBox := lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.notificationColor.GetForeground()).
		Inherit(m.notificationColor).
		Render(notificationContent)

	// Get dimensions
	notifWidth := lipgloss.Width(notificationBox)
	notifHeight := lipgloss.Height(notificationBox)

	// Ensure notification is exactly footerHeight tall by adding vertical padding
	if notifHeight < footerHeight {
		paddingTop := (footerHeight - notifHeight) / 2
		paddingBottom := footerHeight - notifHeight - paddingTop
		notificationBox = lipgloss.NewStyle().
			PaddingTop(paddingTop).
			PaddingBottom(paddingBottom).
			Render(notificationBox)
	}

	// Calculate available width for help (accounting for both notification and scroll percent)
	helpWidth := footerWidth - notifWidth - scrollPercentWidth - 4 // -4 for spacing
	if helpWidth < 0 {
		helpWidth = 0
	}

	// Truncate help to fit in available space
	if helpWidth > 0 {
		helpContent = lipgloss.NewStyle().MaxWidth(helpWidth).Render(helpContent)
	}

	// Center help content vertically
	helpHeight := lipgloss.Height(helpContent)
	if helpHeight < footerHeight {
		paddingTop := (footerHeight - helpHeight) / 2
		paddingBottom := footerHeight - helpHeight - paddingTop
		helpContent = lipgloss.NewStyle().
			PaddingTop(paddingTop).
			PaddingBottom(paddingBottom).
			Render(helpContent)
	}

	// Center scroll percent vertically
	scrollPercentHeight := lipgloss.Height(scrollPercentContent)
	if scrollPercentHeight < footerHeight {
		paddingTop := (footerHeight - scrollPercentHeight) / 2
		paddingBottom := footerHeight - scrollPercentHeight - paddingTop
		scrollPercentContent = lipgloss.NewStyle().
			PaddingTop(paddingTop).
			PaddingBottom(paddingBottom).
			Render(scrollPercentContent)
	}

	// Create footer row with notification and scroll percent at the far right
	middleWidth := footerWidth - lipgloss.Width(helpContent) - notifWidth - scrollPercentWidth - 2
	if middleWidth < 0 {
		middleWidth = 0
	}

	footerRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		helpContent,
		lipgloss.NewStyle().Width(middleWidth).Render(""),
		notificationBox,
		lipgloss.NewStyle().Width(1).Render(" "),
		scrollPercentContent,
	)

	return "\n" + footerRow
}
