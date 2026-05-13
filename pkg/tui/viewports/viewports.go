package viewports

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	tuiviewport "github.com/mihakrumpestar/panix/pkg/tui/viewport"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

type Dimensions struct{ Width, Height int }

type Viewports struct {
	items                        *atomicorderedmap.AtomicOrderedMap[xpath.Xpath, *item]
	dimensions                   *Dimensions
	commandOutputMaxHeight       int
	border                       style.Style
	selectionHighlightBackground style.Style
	selectionHighlightBorder     style.Style
	fullscreenXp                 xpath.Xpath
	mainXpath                    xpath.Xpath
	activeXpath                  xpath.Xpath
	highlightBuf                 *buffer.LinesBuf
}

type item struct {
	model          tuiviewport.Viewport
	content        [][]byte
	contentBuf     *buffer.LinesBuf
	contentVersion uint64
	lastWidth      int
	lastHeight     int
	zoneID         zeroterm.ZoneID
	zonedOutput    *buffer.LinesBuf
}

func (itm *item) release() {
	if itm.zonedOutput != nil {
		itm.zonedOutput.Release()
		itm.zonedOutput = nil
	}

	if itm.contentBuf != nil {
		itm.contentBuf.Release()
		itm.contentBuf = nil
	}
}

func New(dimensions *Dimensions, commandOutputMaxHeight int, border, selectionHighlightBackground, selectionHighlightBorder style.Style) *Viewports {
	mainXpath := xpath.New("main")

	return &Viewports{
		items:                        atomicorderedmap.New[xpath.Xpath, *item](),
		dimensions:                   dimensions,
		commandOutputMaxHeight:       commandOutputMaxHeight,
		border:                       border,
		selectionHighlightBackground: selectionHighlightBackground,
		selectionHighlightBorder:     selectionHighlightBorder,
		mainXpath:                    mainXpath,
		activeXpath:                  mainXpath,
		highlightBuf:                 buffer.NewLinesBuf(),
	}
}

// Dimensions

func (v *Viewports) ContentWidth() int { return v.dimensions.Width - tuiviewport.ScrollbarColWidth() }

// Fullscreen

func (v *Viewports) IsFullscreen() bool              { return v.fullscreenXp.Depth() > 0 }
func (v *Viewports) GetFullscreenXpath() xpath.Xpath { return v.fullscreenXp }
func (v *Viewports) SetFullscreen(xp xpath.Xpath)    { v.fullscreenXp = xp }
func (v *Viewports) ExitFullscreen()                 { v.fullscreenXp = xpath.New() }

// Active queries

func (v *Viewports) HasActiveInner() bool {
	return v.activeXpath.Depth() > 0 && v.activeXpath != v.mainXpath
}

func (v *Viewports) GetActiveInnerViewportContent() ([][]byte, bool) {
	if !v.HasActiveInner() {
		return nil, false
	}

	it, ok := v.items.Get(v.activeXpath)
	if !ok {
		return nil, false
	}

	return it.content, true
}

func (v *Viewports) GetActiveInnerViewportXpath() xpath.Xpath {
	if v.HasActiveInner() {
		return v.activeXpath
	}

	return ""
}

func (v *Viewports) GetViewportContent(xp xpath.Xpath) [][]byte {
	it, ok := v.items.Get(xp)
	if ok {
		return it.content
	}

	return nil
}

func (v *Viewports) DeselectAll() { v.activeXpath = v.mainXpath }

// Reset clears all viewport items and resets fullscreen/active state.
func (v *Viewports) Reset() {
	v.items.DeleteFunc(func(_ xpath.Xpath, itm *item) bool {
		itm.release()

		return true
	})

	v.fullscreenXp = xpath.New()
	v.activeXpath = v.mainXpath
}

// RemoveIfExistsViewport removes a viewport by xpath.

func (v *Viewports) RemoveIfExistsViewport(xp xpath.Xpath) {
	itm, ok := v.items.Get(xp)
	if ok {
		itm.release()
	}

	v.items.Del(xp)

	if v.activeXpath == xp {
		v.activeXpath = v.mainXpath
	}
}

// Update

func (v *Viewports) Update(msg zeroterm.Msg) zeroterm.Cmd {
	if v == nil {
		return nil
	}

	click, ok := msg.(zeroterm.MouseClickMsg)
	if ok {
		clicked := v.clickTarget(click)
		if clicked.Depth() > 0 {
			v.activeXpath = clicked
		} else {
			v.activeXpath = v.mainXpath
		}

		return nil
	}

	itm := v.activeItem()
	if itm != nil {
		itm.model.Update(msg)
	}

	return nil
}

// Render methods

func (v *Viewports) RenderViewportVersioned(xp xpath.Xpath, content [][]byte, version uint64, indent int) *buffer.LinesBuf {
	return v.render(xp, content, nil, indent, 0, 0, true, true, false, version)
}

func (v *Viewports) RenderLabelViewport(xp xpath.Xpath, content [][]byte, version uint64, indent int) *buffer.LinesBuf {
	return v.render(xp, content, nil, indent, 0, 0, false, false, true, version)
}

// RenderMainViewport renders the main viewport. When explicitHeight > 0 the
// viewport is constrained to that height; when 0 the viewport expands to show
// all content (used for final output on quit).
func (v *Viewports) RenderMainViewport(content *buffer.LinesBuf, version uint64, explicitHeight int) *buffer.LinesBuf {
	return v.render(v.mainXpath, content.Lines(), content, 0, explicitHeight, 0, false, true, false, version)
}

func (v *Viewports) RenderFullscreenViewport(
	xpath xpath.Xpath,
	content [][]byte, version uint64, footerHeaderHeight int,
) *buffer.LinesBuf {
	height := max(1, v.dimensions.Height-footerHeaderHeight)
	width := max(1, v.dimensions.Width)

	return v.render(xpath, content, nil, 0, height, width, true, true, false, version)
}

// Debug

func (v *Viewports) Debug() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "\nViewports: %d (%dx%d) ContentWidth=%d\n", v.items.Len(), v.dimensions.Width, v.dimensions.Height, v.ContentWidth())

	for _, pair := range v.items.Pairs() {
		mdl := pair.Value.model
		fmt.Fprintf(&builder, "  '%s': %dx%d l:%d sb:%v sr:%v main:%v overflows:%v",
			pair.Key, mdl.Width(), mdl.Height(), mdl.TotalLineCount(),
			mdl.HasScrollbar(), mdl.HasScrollbarReserve(), mdl.IsMain(),
			mdl.TotalLineCount() > mdl.Height())

		if pair.Key == v.activeXpath {
			builder.WriteString(" [A]")
		}

		if mdl.ScrollPercent() == 1 {
			builder.WriteString(" @btm")
		}

		builder.WriteByte('\n')
	}

	return builder.String()
}

// Internal

func (v *Viewports) render(
	xpath xpath.Xpath,
	content [][]byte,
	contentBuf *buffer.LinesBuf,
	indent, explicitHeight, explicitWidth int,
	bordered, scrollbar, highlightActive bool,
	version uint64,
) *buffer.LinesBuf {
	active := xpath == v.activeXpath
	itm := v.getOrCreateItem(xpath, content, indent, explicitHeight, explicitWidth, bordered, scrollbar)

	contentChanged := itm.contentVersion != version
	content = v.resolveContent(itm, version, content, contentBuf)

	if bordered {
		itm.model.SetBorderStyle(v.borderColor(active))
	}

	width := v.viewWidth(indent, explicitWidth)
	dimsChanged := width != itm.lastWidth || explicitHeight != itm.lastHeight

	if contentChanged || dimsChanged {
		itm.lastWidth = width
		itm.lastHeight = explicitHeight

		err := itm.model.Sync(content, width, explicitHeight)
		if err != nil {
			fmt.Fprintf(os.Stderr, "viewport: %v\n", err)
		}
	}

	rendered := itm.model.Render()

	if highlightActive && active {
		v.highlightBuf.Reset()
		v.selectionHighlightBackground.RenderInto(v.highlightBuf, rendered.Lines())
		rendered = v.highlightBuf
	}

	itm.zonedOutput.Reset()
	itm.zoneID.MarkLines(rendered, itm.zonedOutput)

	return itm.zonedOutput
}

func (v *Viewports) viewWidth(indent, explicitWidth int) int {
	if explicitWidth > 0 {
		return explicitWidth
	}

	width := v.dimensions.Width - indent
	if indent > 0 {
		width -= tuiviewport.ScrollbarColWidth()
	}

	return max(1, width)
}

func (v *Viewports) resolveContent(itm *item, version uint64, content [][]byte, contentBuf *buffer.LinesBuf) [][]byte {
	if itm.contentVersion == version {
		return itm.content
	}

	if itm.contentBuf != nil && itm.contentBuf != contentBuf {
		itm.contentBuf.Release()
	}

	itm.content = content
	itm.contentBuf = contentBuf
	itm.contentVersion = version

	return itm.content
}

func (v *Viewports) getOrCreateItem(
	xpath xpath.Xpath,
	content [][]byte,
	indent, explicitHeight, explicitWidth int,
	bordered, scrollbar bool,
) *item {
	itm, exists := v.items.Get(xpath)
	if exists {
		hasBorder := itm.model.IsBordered()
		hasScrollbar := itm.model.HasScrollbar()

		if hasBorder != bordered || hasScrollbar != scrollbar {
			itm.release()
			v.items.Del(xpath)
		} else {
			return itm
		}
	}

	itm = &item{
		model:       tuiviewport.New(v.buildViewportOpts(xpath, indent, explicitHeight, explicitWidth, bordered, scrollbar)...),
		content:     content,
		zoneID:      zeroterm.NewZoneID(),
		zonedOutput: buffer.NewLinesBuf(),
	}

	if xpath != v.mainXpath {
		itm.model.GotoBottom()
	}

	v.items.Set(xpath, itm)

	return itm
}

func (v *Viewports) buildViewportOpts(
	xpath xpath.Xpath,
	indent, explicitHeight, explicitWidth int,
	bordered, scrollbar bool,
) []tuiviewport.Option {
	opts := []tuiviewport.Option{
		tuiviewport.WithWidth(v.viewWidth(indent, explicitWidth)),
		tuiviewport.WithHeight(max(1, explicitHeight)),
	}

	if scrollbar {
		sbColor := v.borderColor(false)
		opts = append(opts, tuiviewport.WithScrollbar("█", "░", sbColor, sbColor))
	}

	if bordered {
		opts = append(opts, tuiviewport.WithBorder(v.borderColor(false)))
	}

	if bordered && scrollbar {
		maxH := v.commandOutputMaxHeight
		if maxH > 0 {
			opts = append(opts, tuiviewport.WithMaxHeight(maxH))
		}
	}

	if xpath == v.mainXpath {
		opts = append(opts, tuiviewport.WithMain())
	}

	return opts
}

func (v *Viewports) borderColor(active bool) style.Color {
	if active {
		return v.selectionHighlightBorder.GetBackground()
	}

	return v.border.GetForeground()
}

func (v *Viewports) activeItem() *item {
	xp := v.activeXpath
	if xp.String() == "" {
		xp = v.mainXpath
	}

	it, ok := v.items.Get(xp)
	if ok {
		return it
	}

	return nil
}

func (v *Viewports) clickTarget(click zeroterm.MouseClickMsg) xpath.Xpath {
	if click.Lines == nil || click.Y < 0 || click.Y >= click.Lines.Len() {
		return ""
	}

	clickedID, ok := zeroterm.ZoneIDAtCol(click.Lines.Line(click.Y), click.X)
	if !ok {
		return ""
	}

	var candidates []xpath.Xpath
	for _, pair := range v.items.Pairs() {
		if pair.Value.zoneID.Equal(clickedID) {
			candidates = append(candidates, pair.Key)
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Depth() > candidates[j].Depth() })

	return candidates[0]
}
