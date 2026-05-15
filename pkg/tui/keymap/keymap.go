package keymap

import (
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

type Pair struct {
	Key    string
	Desc   string
	Active func() bool
}

type Styles struct {
	Key          style.Style
	Desc         style.Style
	Separator    style.Style
	SelectedKey  style.Style
	SelectedDesc style.Style
}

// CacheKey returns (width, activeMask) — the two values that determine if
// the rendered output changed. Callers can compare against a previous
// cache key to skip re-rendering.
func (k *Keymap) CacheKey() (int, uint64) {
	return k.cacheWidth, k.activeMask()
}

type Keymap struct {
	pairs  []Pair
	styles Styles

	cacheWidth  int
	cacheActive uint64 // bitmask of active states for cache invalidation
	cacheBuf    *buffer.LinesBuf
}

func New(pairs []Pair, styles Styles) *Keymap {
	return &Keymap{
		pairs:    pairs,
		styles:   styles,
		cacheBuf: buffer.NewLinesBuf(),
	}
}

// SetPairs replaces the keybinding pairs and invalidates the cache.
func (k *Keymap) SetPairs(pairs []Pair) {
	k.pairs = pairs
	k.cacheBuf.Reset()
}

// Render writes the keymap into the provided LinesBuf. Results are cached
// by maxWidth and active state. On cache hit, a single AppendFrom copies
// all lines with zero allocation.
func (k *Keymap) Render(buf *buffer.LinesBuf, maxWidth int) {
	if len(k.pairs) == 0 {
		return
	}

	mask := k.activeMask()
	if k.cacheBuf.Len() > 0 && k.cacheWidth == maxWidth && k.cacheActive == mask {
		buf.AppendFrom(k.cacheBuf)

		return
	}

	sepBytes := k.styles.Separator.RenderLine([]byte(" • "))
	sepWidth := style.CellWidth(sepBytes)

	k.cacheBuf.Reset()

	var (
		lineBuf      buffer.LineBuf
		currentWidth int
	)

	for _, pair := range k.pairs {
		item, itemWidth := k.renderPair(pair)

		if currentWidth > 0 && currentWidth+sepWidth+itemWidth > maxWidth {
			k.cacheBuf.WriteLine(lineBuf.Bytes())
			lineBuf.Reset()

			currentWidth = 0
		}

		if currentWidth > 0 {
			lineBuf.Write(sepBytes)

			currentWidth += sepWidth
		}

		lineBuf.Write(item)

		currentWidth += itemWidth
	}

	if lineBuf.Len() > 0 {
		k.cacheBuf.WriteLine(lineBuf.Bytes())
		lineBuf.Reset()
	}

	k.cacheWidth = maxWidth
	k.cacheActive = mask

	buf.AppendFrom(k.cacheBuf)
}

// renderPair returns the rendered key+desc bytes and its cell width.
func (k *Keymap) renderPair(pair Pair) ([]byte, int) {
	keySty := k.styles.Key

	descSty := k.styles.Desc
	if pair.Active != nil && pair.Active() {
		keySty = k.styles.SelectedKey
		descSty = k.styles.SelectedDesc
	}

	keyBytes := keySty.RenderLine([]byte(pair.Key))
	descBytes := descSty.RenderLine([]byte(pair.Desc))

	item := make([]byte, 0, len(keyBytes)+1+len(descBytes))
	item = append(item, keyBytes...)
	item = append(item, ' ')
	item = append(item, descBytes...)

	return item, style.CellWidth(item)
}

// activeMask computes a bitmask of current active states for cache comparison.
func (k *Keymap) activeMask() uint64 {
	var mask uint64

	for i, pair := range k.pairs {
		if pair.Active != nil && pair.Active() {
			mask |= 1 << i
		}
	}

	return mask
}
