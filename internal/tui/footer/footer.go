package footer

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/keymap"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/notification"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/render"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
)

type KeyDef struct {
	Keys    []string
	Help    string
	Handler func() []render.Cmd
}

var notificationBaseStyle = style.NewStyle().Bold(true)

const minFooterHeight = 3

type Footer struct {
	keyDefs      []KeyDef
	keymap       *keymap.Keymap
	notification *notification.Notification
	conf         *config.Config
}

func New(keyDefs []KeyDef, conf *config.Config, colorScheme *colorscheme.ColorScheme) *Footer {
	pairs := make([]keymap.Pair, len(keyDefs))
	for i := range keyDefs {
		pairs[i] = keymap.Pair{
			Key:  keyDefs[i].Keys[0],
			Desc: keyDefs[i].Help,
		}
	}

	return &Footer{
		keyDefs: keyDefs,
		keymap: keymap.New(pairs, keymap.Styles{
			Key:       colorScheme.Footer.HelpKey,
			Desc:      colorScheme.Footer.HelpDesc,
			Separator: colorScheme.Footer.HelpSeparator,
		}),
		notification: notification.New(),
		conf:         conf,
	}
}

func (f *Footer) KeyDefs() []KeyDef { return f.keyDefs }

func (f *Footer) Notification() *notification.Notification { return f.notification }

func (f *Footer) View(width int, colorScheme *colorscheme.ColorScheme) string {
	notifBox := f.notification.View(notificationBaseStyle)
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

func (f *Footer) Update(msg render.Msg) render.Cmd {
	return f.notification.Update(msg)
}
