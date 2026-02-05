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

		// Only propagate context errors if they're not from manual cancellation
		// context.Canceled is expected when user presses 'q' to quit
		if m.err != nil && ctx.Err() != nil {
			// Wrap existing error with context error unless it's just a cancellation
			if ctx.Err() != context.Canceled {
				m.err = errors.Wrap(m.err, ctx.Err().Error())
			}
		} else if ctx.Err() != nil && ctx.Err() != context.Canceled {
			// Set error only if it's not a manual cancellation
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
		// ESC exits fullscreen mode, or deselects inner viewport back to main
		if m.modelView.viewports.IsFullscreen() {
			m.modelView.viewports.ExitFullscreen()
		} else {
			m.modelView.viewports.DeselectAll()
		}
		return m, nil
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

func renderScrollPercent(m *model) string {
	pct := m.modelView.viewports.GetActiveViewportScrollPercent()
	pctInt := int(pct * 100)
	return lipgloss.NewStyle().
		Foreground(m.workflow.State().Conf.Tui.ColorScheme.TableBorder.GetForeground()).
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
		notifWidth = lipgloss.Width(notificationBox) + 1 // +1 for spacing
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
