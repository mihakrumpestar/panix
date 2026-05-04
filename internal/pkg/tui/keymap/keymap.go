package keymap

import (
	"strings"

	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
)

type Pair struct {
	Key  string
	Desc string
}

type Styles struct {
	Key       style.Style
	Desc      style.Style
	Separator style.Style
}

type Keymap struct {
	pairs  []Pair
	styles Styles

	cacheWidth  int
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
// available width would be exceeded. Results are cached by maxWidth.
func (k *Keymap) View(maxWidth int) string {
	if len(k.pairs) == 0 {
		return ""
	}

	if k.cacheResult != "" && k.cacheWidth == maxWidth {
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
		item := k.styles.Key.Render(pair.Key) + " " + k.styles.Desc.Render(pair.Desc)
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

	result := strings.Join(lines, "\n")

	k.cacheWidth = maxWidth
	k.cacheResult = result

	return result
}
