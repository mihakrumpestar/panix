package viewports

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kirill-scherba/omap"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/pkg/cache"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
)

const (
	scrollThumb       = "█"
	scrollTrack       = "│"
	scrollbarWidth    = 2
	borderOverhead    = 2
	mouseScrollAmount = 3
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
	items      *omap.Omap[xpath.Xpath, *item]
	dimensions *Dimensions
	conf       *config.Config
	fullscreen xpath.Xpath
	mainXpath  xpath.Xpath
}

type item struct {
	model    viewport.Model
	active   bool
	content  string
	zoneBase xpath.Xpath
	cache    cache.Cache[string]
}

func NewViewports(dimensions *Dimensions, conf *config.Config) *Viewports {
	m, _ := omap.New[xpath.Xpath, *item]()

	return &Viewports{
		items:      m,
		dimensions: dimensions,
		conf:       conf,
		mainXpath:  xpath.New("main"),
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

func (v *Viewports) RemoveIfExistsViewport(xp xpath.Xpath) { v.items.Del(xp) }

// Active queries

func (v *Viewports) HasActiveInner() bool {
	for xp, it := range v.items.Records() {
		if xp != v.mainXpath && it.active {
			return true
		}
	}

	return false
}

func (v *Viewports) GetActiveInnerViewportContent() (string, bool) {
	for xp, it := range v.items.Records() {
		if it.active && xp != v.mainXpath {
			return it.content, true
		}
	}

	return "", false
}

func (v *Viewports) GetActiveInnerViewportXpath() xpath.Xpath {
	for xp, it := range v.items.Records() {
		if it.active && xp != v.mainXpath {
			return xp
		}
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
	for xp, it := range v.items.Records() {
		it.active = xp == v.mainXpath
	}
}

// Update

func (v *Viewports) Update(msg tea.Msg) tea.Cmd {
	if v == nil {
		return nil
	}

	switch msgParsed := msg.(type) {
	case tea.MouseMsg:
		v.handleMouse(msgParsed)

		return nil
	case tea.KeyPressMsg:
		v.handleKey(msgParsed)

		return nil
	case tea.WindowSizeMsg:
		v.dimensions.Width = msgParsed.Width
		v.dimensions.Height = msgParsed.Height

		return nil
	}

	var cmds []tea.Cmd

	for _, it := range v.items.Records() {
		if it.active {
			if updated, cmd := it.model.Update(msg); cmd != nil {
				it.model = updated

				cmds = append(cmds, cmd)
			}
		}
	}

	if !v.HasActiveInner() {
		if main, ok := v.items.Get(v.mainXpath); ok {
			if updated, cmd := main.model.Update(msg); cmd != nil {
				main.model = updated

				cmds = append(cmds, cmd)
			}
		}
	}

	return tea.Batch(cmds...)
}

// Debug

func (v *Viewports) Debug() string {
	var stringsBuilder strings.Builder
	fmt.Fprintf(&stringsBuilder, "\nViewports: %d (%dx%d)\n", v.items.Len(), v.dimensions.Width, v.dimensions.Height)

	for xp, item := range v.items.Records() {
		fmt.Fprintf(&stringsBuilder, "  '%s': %dx%d c:%d", xp, item.model.Width(), item.model.Height(), item.model.TotalLineCount())

		if item.active {
			stringsBuilder.WriteString(" [A]")
		}

		if item.model.ScrollPercent() == 1 {
			stringsBuilder.WriteString(" @btm")
		}

		stringsBuilder.WriteByte('\n')
	}

	return stringsBuilder.String()
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
			active:   xp == v.mainXpath,
		}
		itemI.model.GotoBottom()
		if err := v.items.Set(xp, itemI); err != nil {
			return ""
		}
	} else {
		itemI.content = content
	}

	return itemI.cache.Get(
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

			r := v.style(itemI, contentH, finalH, kind)
			r = zone.Mark(xp.String(), r)

			return r, true
		},
		width, height, content, itemI.model.ScrollPercent(), itemI.active)
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

// style renders the viewport with border/highlight/scrollbar based on kind.
func (v *Viewports) style(item *item, contentH, visH int, kind Kind) string {
	scrollbar, _ := v.scrollbar(item.model.ScrollPercent(), contentH, visH)
	combined := v.withScrollbar(item.model.View(), scrollbar, item.zoneBase)

	switch kind {
	case KindLabel:
		if item.active {
			combined = v.conf.ColorScheme.Table.SelectionHighlightBackground.Render(combined)
		}

		return combined
	case KindContent, KindFullscreen:
		border := v.conf.ColorScheme.Table.Border.GetForeground()
		if item.active {
			border = v.conf.ColorScheme.Table.SelectionHighlightBorder.GetBackground()
		}

		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Render(combined)
	default: // KindMain
		return combined
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

	var stringsBuilder strings.Builder

	for i := range visible {
		if i > 0 {
			stringsBuilder.WriteByte('\n')
		}

		if i >= pos && i < end {
			stringsBuilder.WriteString(scrollThumb)
		} else {
			stringsBuilder.WriteString(scrollTrack)
		}
	}

	return lipgloss.NewStyle().
		Foreground(v.conf.ColorScheme.Table.Border.GetForeground()).
		Render(stringsBuilder.String()), 1
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

// Input handlers

func (v *Viewports) handleMouse(msg tea.MouseMsg) {
	if wheel, ok := msg.(tea.MouseWheelMsg); ok {
		if it := v.activeViewport(); it != nil {
			if wheel.Button == tea.MouseWheelUp {
				it.model.ScrollUp(mouseScrollAmount)
			} else {
				it.model.ScrollDown(mouseScrollAmount)
			}
		}

		return
	}

	click, ok := msg.(tea.MouseClickMsg)
	if !ok {
		return
	}

	clicked := v.mostSpecific(v.underMouse(click))
	hasClick := clicked.Depth() > 0

	for xp, it := range v.items.Records() {
		it.active = hasClick && xp == clicked
	}

	if !hasClick {
		if main, ok := v.items.Get(v.mainXpath); ok {
			main.active = true
		}
	}
}

func (v *Viewports) handleKey(msg tea.KeyPressMsg) {
	activeViewport := v.activeViewport()
	if activeViewport == nil {
		return
	}

	switch msg.String() {
	case "up", "k":
		activeViewport.model.ScrollUp(1)
	case "down", "j":
		activeViewport.model.ScrollDown(1)
	case "pgup":
		activeViewport.model.HalfPageUp()
	case "pgdown", "space":
		activeViewport.model.HalfPageDown()
	case "home", "g":
		activeViewport.model.GotoTop()
	case "end", "G":
		activeViewport.model.GotoBottom()
	}
}

func (v *Viewports) activeViewport() *item {
	for xp, it := range v.items.Records() {
		if it.active && xp != v.mainXpath {
			return it
		}
	}

	if main, ok := v.items.Get(v.mainXpath); ok {
		return main
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

func (v *Viewports) mostSpecific(xps []xpath.Xpath) xpath.Xpath {
	if len(xps) == 0 {
		return xpath.Xpath{}
	}

	sort.Slice(xps, func(i, j int) bool { return xps[i].Depth() > xps[j].Depth() })

	return xps[0]
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
