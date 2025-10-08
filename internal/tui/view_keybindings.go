package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/errors"
)

var (
	keymapHelp = help.New()
)

// keymap defines all the key bindings for the TUI
type keymap struct {
	quit   key.Binding
	toggle key.Binding
	retry  key.Binding
}

// ShortHelp returns the key bindings to show in the short help view
func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{k.quit, k.toggle, k.retry}
}

// FullHelp returns the key bindings to show in the full help view
func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.quit},
		{k.toggle, k.retry},
	}
}

// newKeymap creates a new keymap with all the necessary key bindings
func newKeymap() keymap {
	return keymap{
		quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
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
	}
}

// HandleKeyInput handles key input using the keymap and returns the updated model and command
func (m *model) HandleKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keys := newKeymap()

	switch {
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

		m.modelView.debugOutput.WriteString("CTX done")

		return m, tea.Quit
	case key.Matches(msg, keys.toggle):
		m.workflow.State().Conf.Tui.ShowAllBuildLogs = !m.workflow.State().Conf.Tui.ShowAllBuildLogs

		return m, nil
	case key.Matches(msg, keys.retry):
		m.workflow.State().Retry.Trigger()

		return m, nil
	}

	return m, nil
}

// ViewKeybindings returns the help view with key bindings positioned at the bottom
func (m *model) ViewKeybindings(builderAbove strings.Builder) string {
	return "\n" + keymapHelp.View(newKeymap())
}
