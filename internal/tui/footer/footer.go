package footer

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/pkg/cache"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/notifications"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/render"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
	"github.com/mihakrumpestar/panix/internal/tui/header"
)

type KeyDef struct {
	Keys    []string
	Help    string
	Handler func() []render.Cmd
}

var notificationBaseStyle = style.NewStyle().Bold(true)

type footerCacheKey struct {
	width int
}

type Footer struct {
	keyDefs      []KeyDef
	notification *notifications.Notification
	conf         *config.Config

	cache cache.Cache[header.ContentAndHeight, footerCacheKey]
}

func New(keyDefs []KeyDef, conf *config.Config) *Footer {
	return &Footer{
		keyDefs:      keyDefs,
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
	notifBox, notifWidth := f.notification.View(notificationBaseStyle)

	helpText := f.cache.Get(func() (header.ContentAndHeight, bool) {
		content := wrapKeybindingsByPair(f.Keymap(), width, colorScheme.Footer.HelpKey)
		height := style.CountLines(content)

		return header.ContentAndHeight{Content: content, Height: height}, true
	}, footerCacheKey{width: width})

	sty := style.NewStyle().Width(width).MaxWidth(width - notifWidth)
	if f.conf.Flags.Debug {
		sty = sty.Background(colorScheme.Footer.DebugBackground.GetBackground())
	}

	styledHelp := sty.Render("\n" + helpText.Content)

	parts := []string{styledHelp}
	if notifWidth != 0 {
		parts = append(parts, notifBox)
	}

	finalContent := style.JoinHorizontal(style.Center, parts...)

	return header.ContentAndHeight{Content: finalContent, Height: helpText.Height}
}

func (f *Footer) Update(msg render.Msg) render.Cmd {
	return f.notification.Update(msg)
}

func wrapKeybindingsByPair(keyMap Keymap, maxWidth int, keyStyle style.Style) string {
	bindings := keyMap.ShortHelp()
	if len(bindings) == 0 {
		return ""
	}

	separator := keyStyle.Render(" • ")
	sepWidth := style.CellWidth(separator)

	var (
		lines        []string
		currentLine  strings.Builder
		currentWidth int
	)

	for _, binding := range bindings {
		if !binding.Enabled() {
			continue
		}

		item := keyStyle.Render(binding.Help().Key)
		item += " " + keyStyle.Render(binding.Help().Desc)

		itemWidth := style.CellWidth(item)

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

	if len(lines) == 1 {
		linesString += "\n"
	}

	return linesString
}
