package keymap

import (
	"strings"

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

type Keymap struct {
	pairs  []Pair
	styles Styles

	cacheWidth  int
	cacheActive uint64 // bitmask of active states for cache invalidation
	cacheResult string
}

func New(pairs []Pair, styles Styles) *Keymap {
	return &Keymap{pairs: pairs, styles: styles}
}

// SetPairs replaces the keybinding pairs and invalidates the cache.
func (k *Keymap) SetPairs(pairs []Pair) {
	k.pairs = pairs
	k.cacheResult = ""
}

// View renders the keymap as a horizontal list of keybinding pairs separated
// by a centered dot, wrapping to a new line at pair boundaries when the
// available width would be exceeded. Results are cached by maxWidth and
// active state.
func (k *Keymap) View(maxWidth int) string {
	if len(k.pairs) == 0 {
		return ""
	}

	mask := k.activeMask()
	if k.cacheResult != "" && k.cacheWidth == maxWidth && k.cacheActive == mask {
		return k.cacheResult
	}

	separator := k.styles.Separator.Render(" • ")
	sepWidth := style.CellWidth(separator)

	var (
		lines        []string
		currentLine  strings.Builder
		currentWidth int
	)

	for _, pair := range k.pairs {
		item, itemWidth := k.renderPair(pair)

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

	result := strings.Join(lines, "\n")

	k.cacheWidth = maxWidth
	k.cacheActive = mask
	k.cacheResult = result

	return result
}

// renderPair returns the rendered key+desc string and its cell width.
func (k *Keymap) renderPair(pair Pair) (string, int) {
	keySty := k.styles.Key

	descSty := k.styles.Desc
	if pair.Active != nil && pair.Active() {
		keySty = k.styles.SelectedKey
		descSty = k.styles.SelectedDesc
	}

	item := keySty.Render(pair.Key) + " " + descSty.Render(pair.Desc)

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
