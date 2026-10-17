package footer

import (
	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/keymap"
	"github.com/mihakrumpestar/panix/pkg/tui/notification"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
)

type KeyDef struct {
	Keys   []string
	Help   string
	Active func() bool
	// Disabled makes the key unusable (dispatch skips it) and hides it from the
	// rendered keymap.
	Disabled func() bool
	Handler  func() zeroterm.Cmd
}

const minFooterHeight = 3

type Footer struct {
	keyDefs      []KeyDef
	keymap       *keymap.Keymap
	notification *notification.Notification

	helpBuf      *buffer.LinesBuf
	cachedRender *buffer.LinesBuf

	cachedWidth       int
	cachedNotifVer    uint64
	cachedKeymapWidth int
	cachedKeymask     uint64
}

func New(keyDefs []KeyDef, scheme *theme.Theme) *Footer {
	footer := &Footer{
		keyDefs:      keyDefs,
		keymap:       keymap.New(pairsFromKeyDefs(keyDefs), keymap.Styles{}),
		notification: notification.New(notification.Styles{}),

		helpBuf:      buffer.NewLinesBuf(),
		cachedRender: buffer.NewLinesBuf(),

		cachedWidth: -1,
	}
	footer.ApplyTheme(scheme)

	return footer
}

// ApplyTheme re-applies the keymap and notification styles and invalidates all
// footer render caches.
func (f *Footer) ApplyTheme(scheme *theme.Theme) {
	f.keymap.ApplyTheme(keymapStyles(scheme.Footer))
	f.notification.ApplyTheme(notificationStyles(scheme))
	f.invalidateCache()
}

func (f *Footer) KeyDefs() []KeyDef { return f.keyDefs }

// SetKeyDefs replaces the key bindings and the rendered keymap together and
// invalidates render caches. The model swaps them when a modal overlay owns the
// keyboard; display-only bindings (no handler) are never dispatched.
func (f *Footer) SetKeyDefs(keyDefs []KeyDef) {
	f.keyDefs = keyDefs
	f.keymap.SetPairs(pairsFromKeyDefs(keyDefs))
	f.invalidateCache()
}

func (f *Footer) Notification() *notification.Notification { return f.notification }

func (f *Footer) Len() int { return f.cachedRender.Len() }

// Render builds the footer (keymap + notification), horizontally joined, and
// returns the cached result when nothing changed. A cache hit returns
// cachedRender without allocating; on a miss helpBuf is reused as the join output
// before the two buffers swap.
func (f *Footer) Render(width int) *buffer.LinesBuf {
	notifBuf := f.notification.Render()
	notifVer := notifBuf.Version()
	keymapWidth, keymask := f.keymap.CacheKey()

	if width == f.cachedWidth && notifVer == f.cachedNotifVer &&
		keymapWidth == f.cachedKeymapWidth && keymask == f.cachedKeymask {
		return f.cachedRender
	}

	f.cachedWidth = width
	f.cachedNotifVer = notifVer
	f.cachedKeymapWidth = keymapWidth
	f.cachedKeymask = keymask

	// Raw keymap content into helpBuf.
	f.helpBuf.Reset()
	f.helpBuf.EmptyLine()
	f.keymap.Render(f.helpBuf, width)

	for f.helpBuf.Len() < minFooterHeight {
		f.helpBuf.EmptyLine()
	}

	// Apply the width constraint and style the help into cachedRender.
	helpWidth := width

	if notifBuf != nil {
		notifSnap := notifBuf.Snapshot()

		w := style.MaxLineWidth(notifSnap)
		if w > 0 {
			helpWidth = width - w
		}

		f.cachedRender.Reset()
		style.NewStyle().Width(helpWidth).MaxWidth(helpWidth).RenderIntoBuf(f.cachedRender, f.helpBuf)

		// Join with the notification, reusing helpBuf as the join output.
		f.helpBuf.Reset()
		style.JoinHorizontalBufs(f.helpBuf, style.Center, f.cachedRender, notifSnap)

		// Swap so cachedRender holds the joined result and helpBuf is disposable.
		f.helpBuf, f.cachedRender = f.cachedRender, f.helpBuf

		notifSnap.Release()
	} else {
		f.cachedRender.Reset()
		style.NewStyle().Width(helpWidth).MaxWidth(helpWidth).RenderIntoBuf(f.cachedRender, f.helpBuf)
	}

	return f.cachedRender
}

func (f *Footer) Update(msg zeroterm.Msg) zeroterm.Cmd {
	return f.notification.Update(msg)
}

// notificationStyles maps the resolved theme onto the notification widget.
// Warning keeps the marker in the warning color while the message uses primary
// row text: the accent is too low contrast for body text on light themes
// (e.g. rose-pine-dawn).
func notificationStyles(scheme *theme.Theme) notification.Styles {
	return notification.Styles{
		DefaultFg: scheme.Notification.DefaultFgColor,
		Base:      scheme.Footer.NotificationBaseStyle.Style,
		Border:    scheme.Border.NotificationGlyphs,
		Info: notification.LevelStyle{
			FrameColor: scheme.Notification.DefaultFgColor,
		},
		Success: notification.LevelStyle{
			FrameColor: scheme.Status.OK.Style.GetForeground(),
		},
		Warning: notification.LevelStyle{
			Marker:      "! ",
			MarkerStyle: scheme.Status.Warning.Style,
			TextStyle:   scheme.Table.Row.Style,
			FrameColor:  scheme.Status.Warning.Style.GetForeground(),
		},
		Error: notification.LevelStyle{
			FrameColor: scheme.Status.Failed.Style.GetForeground(),
		},
	}
}

// invalidateCache forces the next Render to rebuild. cachedWidth -1 can never
// match a real width.
func (f *Footer) invalidateCache() {
	f.cachedWidth = -1
	f.cachedNotifVer = 0
	f.cachedKeymapWidth = 0
	f.cachedKeymask = 0
}

func keymapStyles(scheme theme.ThemeFooter) keymap.Styles {
	return keymap.Styles{
		Key:          scheme.HelpKey.Style,
		Desc:         scheme.HelpDesc.Style,
		Separator:    scheme.HelpSeparator.Style,
		SelectedKey:  scheme.HelpSelectedKey.Style,
		SelectedDesc: scheme.HelpSelectedDesc.Style,
	}
}

func pairsFromKeyDefs(keyDefs []KeyDef) []keymap.Pair {
	pairs := make([]keymap.Pair, len(keyDefs))
	for i := range keyDefs {
		pairs[i] = keymap.Pair{
			Key:      keyDefs[i].Keys[0],
			Desc:     keyDefs[i].Help,
			Active:   keyDefs[i].Active,
			Disabled: keyDefs[i].Disabled,
		}
	}

	return pairs
}
