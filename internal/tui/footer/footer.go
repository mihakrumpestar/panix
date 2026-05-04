package footer

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/keymap"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/notification"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/zeroterm"
)

type KeyDef struct {
	Keys    []string
	Help    string
	Active  func() bool
	Handler func() []zeroterm.Cmd
}

const minFooterHeight = 3

type Footer struct {
	keyDefs      []KeyDef
	keymap       *keymap.Keymap
	notification *notification.Notification
	conf         *config.Config
}

func New(keyDefs []KeyDef, conf *config.Config, colorScheme *colorscheme.ColorScheme) *Footer {
	return &Footer{
		keyDefs: keyDefs,
		keymap: keymap.New(pairsFromKeyDefs(keyDefs), keymap.Styles{
			Key:          colorScheme.Footer.HelpKey,
			Desc:         colorScheme.Footer.HelpDesc,
			Separator:    colorScheme.Footer.HelpSeparator,
			SelectedKey:  colorScheme.Footer.HelpSelectedKey,
			SelectedDesc: colorScheme.Footer.HelpSelectedDesc,
		}),
		notification: notification.New(colorScheme.Notification.DefaultFgColor),
		conf:         conf,
	}
}

func (f *Footer) KeyDefs() []KeyDef { return f.keyDefs }

func (f *Footer) Notification() *notification.Notification { return f.notification }

func (f *Footer) View(width int, colorScheme *colorscheme.ColorScheme) string {
	notifBox := f.notification.View(colorScheme.Footer.NotificationBaseStyle)
	notifWidth := style.CellWidth(notifBox)

	content := "\n" + f.keymap.View(width)

	if style.CountLines(content) < minFooterHeight {
		content += "\n"
	}

	sty := style.NewStyle().Width(width).MaxWidth(width - notifWidth)
	if f.conf.Flags.Debug {
		sty = sty.Background(colorScheme.Footer.DebugBackground.GetBackground())
	}

	styledHelp := sty.Render(content)

	parts := []string{styledHelp}
	if notifWidth > 0 {
		parts = append(parts, notifBox)
	}

	return style.JoinHorizontal(style.Center, parts...)
}

func (f *Footer) Update(msg zeroterm.Msg) zeroterm.Cmd {
	return f.notification.Update(msg)
}

// Helpers

func pairsFromKeyDefs(keyDefs []KeyDef) []keymap.Pair {
	pairs := make([]keymap.Pair, len(keyDefs))
	for i := range keyDefs {
		pairs[i] = keymap.Pair{
			Key:    keyDefs[i].Keys[0],
			Desc:   keyDefs[i].Help,
			Active: keyDefs[i].Active,
		}
	}

	return pairs
}
