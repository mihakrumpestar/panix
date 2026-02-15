// Package tui_notifications provides notification functionality for the TUI.
package tui_notifications

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Msg is the tea.Msg that clears the notification after timeout.
type Msg struct{}

// Notification holds the state of a notification.
type Notification struct {
	text  string
	color lipgloss.Style
	time  time.Time
}

// New creates a new empty Notification.
func New() *Notification {
	return &Notification{}
}

// Set sets the notification text and color, and returns a tea.Cmd that will
// fire after 3 seconds to clear the notification.
func (n *Notification) Set(text string, color lipgloss.Style) tea.Cmd {
	n.text = text
	n.color = color
	n.time = time.Now()
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return Msg{} })
}

// SetModel sets the notification and returns the model and command.
// This is a convenience method for use in Update methods.
func (n *Notification) SetModel(model tea.Model, text string, color lipgloss.Style) (tea.Model, tea.Cmd) {
	return model, n.Set(text, color)
}

// SetCmd returns a tea.Cmd that sets the notification when executed.
// This is useful when you need to return a command that sets the notification
// later in the update cycle.
func (n *Notification) SetCmd(text string, color lipgloss.Style) tea.Cmd {
	return func() tea.Msg {
		n.text = text
		n.color = color
		n.time = time.Now()
		return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return Msg{} })()
	}
}

// Clear clears the notification state.
func (n *Notification) Clear() {
	n.text = ""
	n.time = time.Time{}
}

// IsEmpty returns true if there's no notification to display.
func (n *Notification) IsEmpty() bool {
	return n.text == ""
}

// GetText returns the notification text.
func (n *Notification) GetText() string {
	return n.text
}

// GetColor returns the notification color style.
func (n *Notification) GetColor() lipgloss.Style {
	return n.color
}

// GetTime returns the notification timestamp.
func (n *Notification) GetTime() time.Time {
	return n.time
}

// Render renders the notification with the given base style.
// It handles the fade-out effect based on elapsed time.
func (n *Notification) Render(baseStyle lipgloss.Style) string {
	if n.text == "" {
		return ""
	}

	elapsed := time.Since(n.time)
	const duration = 3 * time.Second
	const fadeDuration = time.Second

	if elapsed > duration {
		return ""
	}

	style := baseStyle
	if elapsed > duration-fadeDuration {
		style = style.Faint(true)
	}

	return style.Inherit(n.color).Render(n.text)
}
