package footer

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/pkg/cache"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/notifications"
	"github.com/mihakrumpestar/panix/internal/tui/header"
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
	conf         *config.Config

	cache cache.Cache[header.ContentAndHeight]
}

func New(keyDefs []KeyDef, conf *config.Config) *Footer {
	h := help.New()

	return &Footer{
		keyDefs:      keyDefs,
		keymapHelp:   h,
		notification: notifications.New(),
		conf:         conf,
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

func (f *Footer) View(width int, colorScheme *colorscheme.ColorScheme) header.ContentAndHeight {
	f.keymapHelp.Styles.ShortKey = colorScheme.Footer.HelpKey
	f.keymapHelp.Styles.FullKey = colorScheme.Footer.HelpKey

	notifBox, notifWidth := f.notification.View(notificationBaseStyle)

	helpText := f.cache.Get(func() (header.ContentAndHeight, bool) {
		content := wrapKeybindingsByPair(f.keymapHelp, f.Keymap(), width)
		height := lipgloss.Height(content)

		return header.ContentAndHeight{Content: content, Height: height}, true
	}, width)

	style := lipgloss.NewStyle().Width(width).MaxWidth(width - notifWidth)
	if f.conf.Flags.Debug {
		style = style.Background(colorScheme.Footer.DebugBackground.GetBackground())
	}

	styledHelp := style.Render("\n" + helpText.Content)

	parts := []string{styledHelp}
	if notifWidth != 0 {
		parts = append(parts, notifBox)
	}

	finalContent := lipgloss.JoinHorizontal(lipgloss.Center, parts...)

	return header.ContentAndHeight{Content: finalContent, Height: helpText.Height}
}

func (f *Footer) Update(msg tea.Msg) tea.Cmd {
	return f.notification.Update(msg)
}

func wrapKeybindingsByPair(helpModel help.Model, keyMap help.KeyMap, maxWidth int) string {
	bindings := keyMap.ShortHelp()
	if len(bindings) == 0 {
		return ""
	}

	separator := helpModel.Styles.ShortSeparator.Render(helpModel.ShortSeparator)
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

		item := helpModel.Styles.ShortKey.Render(binding.Help().Key)
		item += " " + helpModel.Styles.ShortDesc.Render(binding.Help().Desc)

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

	linesString := strings.Join(lines, "\n")
	linesString += "\n"

	if len(lines) == 1 { // Keep at least 3 in full height, so that notification can appear normally
		linesString += "\n"
	}

	return linesString
}
