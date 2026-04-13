package footer

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mihakrumpestar/panix/internal/pkg/cache"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/notifications"
)

type KeyDef struct {
	Keys    []string
	Help    string
	Handler func() (tea.Model, tea.Cmd)
}

var notificationBaseStyle = lipgloss.NewStyle().Bold(true)

type Footer struct {
	keyDefs      []KeyDef
	keymapHelp   help.Model
	notification *notifications.Notification
	cache        cache.Cache[string]
}

func New(keyDefs []KeyDef) *Footer {
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	h.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))

	return &Footer{
		keyDefs:      keyDefs,
		keymapHelp:   h,
		notification: notifications.New(),
	}
}

func (f *Footer) KeyDefs() []KeyDef { return f.keyDefs }

func (f *Footer) Notification() *notifications.Notification { return f.notification }

func (f *Footer) Keymap() Keymap {
	bindings := make([]key.Binding, len(f.keyDefs))
	for i := range f.keyDefs {
		bindings[i] = key.NewBinding(
			key.WithKeys(f.keyDefs[i].Keys...),
			key.WithHelp(f.keyDefs[i].Keys[0], f.keyDefs[i].Help),
		)
	}
	return Keymap{bindings: bindings}
}

type Keymap struct {
	bindings []key.Binding
}

func (k Keymap) ShortHelp() []key.Binding  { return k.bindings }
func (k Keymap) FullHelp() [][]key.Binding { return [][]key.Binding{k.bindings} }

func (f *Footer) View(width int) string {
	notifBox, notifWidth := f.notification.RenderBox(notificationBaseStyle)

	helpText := f.cache.Get(func() (string, bool) {
		return "\n\n" + wrapKeybindingsByPair(f.keymapHelp, f.Keymap(), width), true
	}, width)

	styledHelp := lipgloss.NewStyle().Width(width).MaxWidth(max(width-notifWidth, 1)).Render(helpText)

	parts := []string{styledHelp}
	if notifWidth != 0 {
		parts = append(parts, "\n"+notifBox)
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

func (f *Footer) Update(msg tea.Msg) tea.Cmd {
	return f.notification.Update(msg)
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
