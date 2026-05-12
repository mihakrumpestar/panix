package footer

import (
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/keymap"
	"github.com/mihakrumpestar/panix/pkg/tui/notification"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
)

type KeyDef struct {
	Keys    []string
	Help    string
	Active  func() bool
	Handler func() zeroterm.Cmd
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

func New(keyDefs []KeyDef, colorScheme *colorscheme.ColorScheme) *Footer {
	footer := &Footer{
		keyDefs: keyDefs,
		keymap: keymap.New(pairsFromKeyDefs(keyDefs), keymap.Styles{
			Key:          colorScheme.Footer.HelpKey,
			Desc:         colorScheme.Footer.HelpDesc,
			Separator:    colorScheme.Footer.HelpSeparator,
			SelectedKey:  colorScheme.Footer.HelpSelectedKey,
			SelectedDesc: colorScheme.Footer.HelpSelectedDesc,
		}),
		notification: notification.New(colorScheme.Notification.DefaultFgColor),

		helpBuf:      buffer.NewLinesBuf(),
		cachedRender: buffer.NewLinesBuf(),

		cachedWidth: -1,
	}

	footer.notification.SetBaseStyle(colorScheme.Footer.NotificationBaseStyle)

	return footer
}

func (f *Footer) KeyDefs() []KeyDef { return f.keyDefs }

func (f *Footer) Notification() *notification.Notification { return f.notification }

func (f *Footer) Len() int { return f.cachedRender.Len() }

// Render builds the footer (keymap + notification), horizontally joined.
// Returns nil when quitting. Skips re-rendering when nothing changed.
//
// Buffer flow (2 persistent buffers, no allocations on cache hit):
//
//	helpBuf:       raw keymap → (reused as join output) → swap → disposable
//	cachedRender:  styled help  → (reused as join input) → swap → final result
func (f *Footer) Render(quitting bool, width int) *buffer.LinesBuf {
	if quitting {
		return nil
	}

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

	// 1. Raw keymap content into helpBuf
	f.helpBuf.Reset()
	f.helpBuf.EmptyLine()
	f.keymap.Render(f.helpBuf, width)

	for f.helpBuf.Len() < minFooterHeight {
		f.helpBuf.EmptyLine()
	}

	// 2. Apply width constraint → styled help into cachedRender
	helpWidth := width

	if notifBuf != nil {
		if w := style.MaxLineWidth(notifBuf.LinesBuf); w > 0 {
			helpWidth = width - w
		}
	}

	f.cachedRender.Reset()
	style.NewStyle().Width(helpWidth).MaxWidth(helpWidth).RenderInto(f.cachedRender, f.helpBuf.Lines())

	// 3. Join with notification (if present)
	if notifBuf != nil {
		// cachedRender holds styled help (join input) — reuse helpBuf as join output
		f.helpBuf.Reset()
		style.JoinHorizontalBufs(f.helpBuf, style.Center, f.cachedRender, notifBuf.LinesBuf)

		// Swap: cachedRender ← final result, helpBuf ← disposable for next frame
		f.helpBuf, f.cachedRender = f.cachedRender, f.helpBuf
	}

	return f.cachedRender
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
