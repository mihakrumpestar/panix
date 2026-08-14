package keymap

import (
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

type Pair struct {
	Key    string
	Desc   string
	Active func() bool
	// Disabled hides the pair from the rendered keymap and (via the caller's
	// dispatch loop) makes it unusable.
	Disabled func() bool
}

type Styles struct {
	Key          style.Style
	Desc         style.Style
	Separator    style.Style
	SelectedKey  style.Style
	SelectedDesc style.Style
}

// CacheKey returns (width, stateMask), the two values that determine if
// the rendered output changed. Callers can compare against a previous
// cache key to skip re-rendering.
func (k *Keymap) CacheKey() (int, uint64) {
	return k.cacheWidth, k.stateMask()
}

type Keymap struct {
	pairs  []Pair
	styles Styles

	cacheWidth int
	cacheState uint64 // bitmask of active/disabled states for cache invalidation
	cacheBuf   *buffer.LinesBuf
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
// by maxWidth and pair state (active/disabled). On cache hit, a single
// AppendFrom copies all lines with zero allocation.
func (k *Keymap) Render(buf *buffer.LinesBuf, maxWidth int) {
	if len(k.pairs) == 0 {
		return
	}

	mask := k.stateMask()
	if k.isCached(maxWidth, mask) {
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
		// Disabled pairs are hidden entirely: they are unusable, so showing
		// them would misrepresent what actually works.
		if pair.Disabled != nil && pair.Disabled() {
			continue
		}

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
	k.cacheState = mask

	buf.AppendFrom(k.cacheBuf)
}

// isCached reports whether the rendered buffer still matches the given width
// and pair state (active/disabled).
func (k *Keymap) isCached(maxWidth int, mask uint64) bool {
	return k.cacheBuf.Len() > 0 && k.cacheWidth == maxWidth && k.cacheState == mask
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

// stateMask computes a bitmask of current active/disabled states (two bits
// per pair) for cache comparison.
func (k *Keymap) stateMask() uint64 {
	var mask uint64

	for idx, pair := range k.pairs {
		if pair.Active != nil && pair.Active() {
			mask |= 1 << (2 * idx)
		}

		if pair.Disabled != nil && pair.Disabled() {
			mask |= 1 << (2*idx + 1)
		}
	}

	return mask
}
