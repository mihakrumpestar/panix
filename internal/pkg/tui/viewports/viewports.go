package viewports

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/internal/pkg/cache"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
)

const (
	scrollThumb    = "█"
	scrollTrack    = "│"
	scrollbarWidth = 2
	borderOverhead = 2
)

type Dimensions struct{ Width, Height int }

// Kind classifies viewport rendering style.
type Kind int

const (
	KindContent    Kind = iota // bordered, word-wrapped, maxHeight-capped command output
	KindLabel                  // borderless, word-wrapped, highlight-on-active label
	KindMain                   // fills available space, no border, no wrap cap (main scroll area)
	KindFullscreen             // fills available space, bordered, word-wrapped (fullscreen view)
)

type Viewports struct {
	items       *atomicorderedmap.AtomicOrderedMap[xpath.Xpath, *item]
	dimensions  *Dimensions
	conf        *config.Config
	fullscreen  xpath.Xpath
	mainXpath   xpath.Xpath
	activeXpath xpath.Xpath
}

type item struct {
	model    viewport.Model
	content  string
	zoneBase xpath.Xpath
	cache    cache.Cache[string]
}

func NewViewports(dimensions *Dimensions, conf *config.Config) *Viewports {
	mainXpath := xpath.New("main")

	return &Viewports{
		items:       atomicorderedmap.New[xpath.Xpath, *item](),
		dimensions:  dimensions,
		conf:        conf,
		mainXpath:   mainXpath,
		activeXpath: mainXpath,
	}
}

// Fullscreen

func (v *Viewports) IsFullscreen() bool {
	return v.fullscreen.Depth() > 0
}
func (v *Viewports) GetFullscreenXpath() xpath.Xpath {
	return v.fullscreen
}
func (v *Viewports) SetFullscreen(xp xpath.Xpath) {
	v.fullscreen = xp
}
func (v *Viewports) ExitFullscreen() {
	v.fullscreen = xpath.New()
}

// ContentWidth returns available width accounting for scrollbar.
func (v *Viewports) ContentWidth() int { return v.dimensions.Width - scrollbarWidth }

// Factory methods

func (v *Viewports) GetOrCreateViewport(xp xpath.Xpath, content string, indent int) string {
	return v.render(xp, content, indent, KindContent)
}

func (v *Viewports) GetOrCreateLabelViewport(xp xpath.Xpath, content string, indent int) string {
	return v.render(xp, content, indent, KindLabel)
}

func (v *Viewports) GetOrCreateMainViewport(content string, footerHeaderHeight int) string {
	h := v.dimensions.Height - footerHeaderHeight

	return v.render(v.mainXpath, content, 0, KindMain, withHeight(h)) + "\n"
}

func (v *Viewports) RenderFullscreenViewport(xp xpath.Xpath, content string, footerHeaderHeight int) string {
	h := max(1, v.dimensions.Height-footerHeaderHeight-borderOverhead)
	w := max(1, v.dimensions.Width-scrollbarWidth-borderOverhead)

	return v.render(xp, content, 0, KindFullscreen, withHeight(h), withWidth(w)) + "\n"
}

func (v *Viewports) RemoveIfExistsViewport(xp xpath.Xpath) {
	v.items.Del(xp)

	if v.activeXpath == xp {
		v.activeXpath = v.mainXpath
	}
}

// Active queries

func (v *Viewports) HasActiveInner() bool {
	return v.activeXpath.Depth() > 0 && v.activeXpath != v.mainXpath
}

func (v *Viewports) GetActiveInnerViewportContent() (string, bool) {
	if !v.HasActiveInner() {
		return "", false
	}

	it, ok := v.items.Get(v.activeXpath)
	if !ok {
		return "", false
	}

	return it.content, true
}

func (v *Viewports) GetActiveInnerViewportXpath() xpath.Xpath {
	if v.HasActiveInner() {
		return v.activeXpath
	}

	return xpath.Xpath{}
}

func (v *Viewports) GetViewportContent(xp xpath.Xpath) string {
	if it, ok := v.items.Get(xp); ok {
		return it.content
	}

	return ""
}

func (v *Viewports) DeselectAll() {
	v.activeXpath = v.mainXpath
}

// Update

func (v *Viewports) Update(msg tea.Msg) tea.Cmd {
	if v == nil {
		return nil
	}

	// Set active viewport by click
	mouseClick, ok := msg.(tea.MouseClickMsg)
	if ok {
		clicked := v.mostSpecific(v.underMouse(mouseClick))

		if clicked.Depth() > 0 {
			v.activeXpath = clicked
		} else {
			v.activeXpath = v.mainXpath
		}

		return nil
	}

	activeViewport := v.activeViewport()
	if activeViewport == nil {
		return nil
	}

	activeViewport.model, _ = activeViewport.model.Update(msg) // Never returns cmd

	return nil
}

// Debug

func (v *Viewports) Debug() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "\nViewports: %d (%dx%d)\n", v.items.Len(), v.dimensions.Width, v.dimensions.Height)

	for _, pair := range v.items.Pairs() {
		fmt.Fprintf(&builder, "  '%s': %dx%d c:%d", pair.Key, pair.Value.model.Width(), pair.Value.model.Height(), pair.Value.model.TotalLineCount())

		if pair.Key == v.activeXpath {
			builder.WriteString(" [A]")
		}

		if pair.Value.model.ScrollPercent() == 1 {
			builder.WriteString(" @btm")
		}

		builder.WriteByte('\n')
	}

	return builder.String()
}

// Internal: render options

type renderOpts struct {
	explicitWidth  int
	explicitHeight int
}

func withWidth(w int) func(*renderOpts)  { return func(o *renderOpts) { o.explicitWidth = w } }
func withHeight(h int) func(*renderOpts) { return func(o *renderOpts) { o.explicitHeight = h } }

// Internal: unified render

func (v *Viewports) render(xp xpath.Xpath, content string, indent int, kind Kind, optsFns ...func(*renderOpts)) string {
	var opts renderOpts
	for _, fn := range optsFns {
		fn(&opts)
	}

	width := opts.explicitWidth
	if width == 0 {
		width = max(1, v.dimensions.Width-indent-scrollbarWidth)
	}

	maxH := v.conf.Flags.Tui.CommandOutputMaxHeight
	if kind == KindMain || kind == KindFullscreen {
		maxH = 0
	}

	height := max(1, opts.explicitHeight)

	itemI, exists := v.items.Get(xp)
	if !exists {
		itemI = &item{
			model:    viewport.New(viewport.WithWidth(width), viewport.WithHeight(height)),
			zoneBase: xp.NewXpathWithAppend("scrollbar"),
			content:  content,
		}
		itemI.model.GotoBottom()

		v.items.Set(xp, itemI)
	} else {
		itemI.content = content
	}

	view := itemI.cache.Get(
		func() (string, bool) {
			wasAtBottom := itemI.model.ScrollPercent() == 1 && xp != v.mainXpath
			yOffset := itemI.model.YOffset()

			contentH := v.setContent(itemI, content, width)
			itemI.model.SetWidth(width)

			// Scrollbar narrows content for capped viewports
			if kind == KindContent && maxH > 0 && contentH > maxH {
				narrowW := max(1, width-scrollbarWidth)
				itemI.model.SetWidth(narrowW)
				contentH = v.setContent(itemI, content, narrowW)
			}

			finalH := contentH
			if maxH > 0 && contentH > maxH && kind == KindContent {
				finalH = maxH
			} else if kind == KindMain || kind == KindFullscreen {
				finalH = max(1, height)
			}

			itemI.model.SetHeight(finalH)

			if wasAtBottom {
				itemI.model.GotoBottom()
			} else {
				itemI.model.SetYOffset(min(yOffset, max(0, contentH-finalH)))
			}

			scrollbar, _ := v.scrollbar(itemI.model.ScrollPercent(), contentH, finalH)
			view := v.withScrollbar(itemI.model.View(), scrollbar, itemI.zoneBase)

			return view, true
		},
		width, height, content, itemI.model.ScrollPercent())

	active := xp == v.activeXpath
	view = v.applyActiveStyle(view, kind, active)
	view = zone.Mark(xp.String(), view)

	return view
}

// setContent word-wraps content and sets it on the viewport model.
// Returns the total line count (minimum 1).
func (v *Viewports) setContent(it *item, content string, width int) int {
	it.model.SetContent(lipgloss.Wrap(content, width, ""))

	h := it.model.TotalLineCount()
	if h == 0 {
		h = 1
	}

	return h
}

func (v *Viewports) applyActiveStyle(view string, kind Kind, active bool) string {
	switch kind {
	case KindLabel:
		if active {
			return v.conf.ColorScheme.Table.SelectionHighlightBackground.Render(view)
		}

		return view
	case KindContent, KindFullscreen:
		border := v.conf.ColorScheme.Table.Border.GetForeground()
		if active {
			border = v.conf.ColorScheme.Table.SelectionHighlightBorder.GetBackground()
		}

		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Render(view)
	default: // KindMain
		return view
	}
}

// scrollbar renders a vertical scrollbar when content exceeds visible area.
func (v *Viewports) scrollbar(pct float64, total, visible int) (string, int) {
	if total <= visible {
		return "", 0
	}

	thumb := max(1, visible*visible/total)
	pos := int(float64(visible-thumb) * clamp(pct, 0, 1))
	end := pos + thumb

	var builder strings.Builder

	for i := range visible {
		if i > 0 {
			builder.WriteByte('\n')
		}

		if i >= pos && i < end {
			builder.WriteString(scrollThumb)
		} else {
			builder.WriteString(scrollTrack)
		}
	}

	return lipgloss.NewStyle().
		Foreground(v.conf.ColorScheme.Table.Border.GetForeground()).
		Render(builder.String()), 1
}

// withScrollbar appends scrollbar lines to the right side of the viewport view.
func (v *Viewports) withScrollbar(view, bar string, barZone xpath.Xpath) string {
	if bar == "" {
		return view
	}

	vLines := strings.Split(view, "\n")
	bLines := strings.Split(bar, "\n")
	result := make([]string, len(vLines))

	for i, line := range vLines {
		if i < len(bLines) {
			result[i] = line + " " + zone.Mark(barZone.NewXpathWithAppend(strconv.Itoa(i)).String(), bLines[i])
		} else {
			result[i] = line
		}
	}

	return strings.Join(result, "\n")
}

// Input helpers

func (v *Viewports) activeViewport() *item {
	activeXpath := v.activeXpath

	if activeXpath.String() == "" {
		activeXpath = v.mainXpath
	}

	it, ok := v.items.Get(activeXpath)
	if ok {
		return it
	}

	return nil
}

func (v *Viewports) underMouse(m tea.MouseMsg) []xpath.Xpath {
	var result []xpath.Xpath

	for xp := range v.items.Records() {
		if zone.Get(xp.String()).InBounds(m) {
			result = append(result, xp)
		}
	}

	return result
}

func (v *Viewports) mostSpecific(xpathI []xpath.Xpath) xpath.Xpath {
	if len(xpathI) == 0 {
		return xpath.Xpath{}
	}

	sort.Slice(xpathI, func(i, j int) bool { return xpathI[i].Depth() > xpathI[j].Depth() })

	return xpathI[0]
}

func clamp(v, low, high float64) float64 { //nolint:varnamelen
	if v < low {
		return low
	}

	if v > high {
		return high
	}

	return v
}
