package viewports

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	tuiviewport "github.com/mihakrumpestar/panix/pkg/tui/viewport"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

// autoShrinkDelay is how long an auto-sized viewport holds a larger height
// after its content last changed. Output that briefly adds lines would
// otherwise resize its box on every frame and shift the surrounding layout.
const autoShrinkDelay = 2 * time.Second

type Dimensions struct{ Width, Height int }

type Viewports struct {
	items                        *atomicorderedmap.AtomicOrderedMap[xpath.Xpath, *item]
	dimensions                   *Dimensions
	commandOutputMaxHeight       int
	borderGlyphs                 style.Border
	border                       style.Style
	selectionHighlightBackground style.Style
	selectionHighlightBorder     style.Style
	fullscreenXp                 xpath.Xpath
	mainXpath                    xpath.Xpath
	activeXpath                  xpath.Xpath
	highlightBuf                 *buffer.LinesBuf
	lastActiveXpath              xpath.Xpath
	lastActiveYOffset            int
	now                          func() time.Time
	shrinkTickAt                 time.Time
	shrinkTickGeneration         uint64
	// hasPendingShrink is set only by trackShrink; every other shrinkSince
	// write is a clear. nextShrinkAt recomputes it from the source of truth.
	hasPendingShrink bool
	dirty            bool
}

type item struct {
	model          tuiviewport.Viewport
	contentBuf     *buffer.LinesBuf
	contentVersion uint64
	lastWidth      int
	lastHeight     int
	shrinkSince    time.Time // last content update while holding a larger height
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

// invalidate forces the next render to re-sync and re-render this item's
// viewport despite unchanged versions, as on theme changes.
func (itm *item) invalidate() {
	itm.contentVersion = ^uint64(0)
	itm.model.InvalidateCache()
}

// releaseHeight drops the auto-height hold; the caller decides whether the
// cached render must be invalidated as well.
func (itm *item) releaseHeight() {
	itm.model.AllowShrink()
	itm.shrinkSince = time.Time{}
}

// releaseShrink drops the hold and forces the next render to re-sync it.
func (itm *item) releaseShrink() {
	itm.releaseHeight()
	itm.invalidate()
}

func New(dimensions *Dimensions, commandOutputMaxHeight int, border, selectionHighlightBackground, selectionHighlightBorder style.Style) *Viewports {
	mainXpath := xpath.New("main")

	viewports := &Viewports{
		items:                  atomicorderedmap.New[xpath.Xpath, *item](),
		dimensions:             dimensions,
		commandOutputMaxHeight: commandOutputMaxHeight,
		mainXpath:              mainXpath,
		activeXpath:            mainXpath,
		lastActiveXpath:        mainXpath,
		highlightBuf:           buffer.NewLinesBuf(),
		now:                    time.Now,
	}
	viewports.ApplyTheme(border, selectionHighlightBackground, selectionHighlightBorder, style.RoundedBorder())

	return viewports
}

// ApplyTheme stores the styles and glyphs, then restyles every live viewport and
// invalidates its render cache and content sync. It is the single theme entry
// point; items created later pick the glyphs up via the constructor option.
func (v *Viewports) ApplyTheme(border, selectionHighlightBackground, selectionHighlightBorder style.Style, glyphs style.Border) {
	v.border = border
	v.selectionHighlightBackground = selectionHighlightBackground
	v.selectionHighlightBorder = selectionHighlightBorder
	v.borderGlyphs = glyphs

	sbColor := v.borderColor(false)

	v.items.ForEach(func(xp xpath.Xpath, itm *item) bool {
		itm.model.ApplyTheme(v.borderColor(xp == v.activeXpath), sbColor, sbColor, glyphs)
		itm.invalidate()

		return true
	})
}

// Dimensions

func (v *Viewports) ContentWidth() int { return v.dimensions.Width - tuiviewport.ScrollbarColWidth() }

func (v *Viewports) ContentVersion(xp xpath.Xpath) uint64 {
	itm, ok := v.items.Get(xp)
	if !ok {
		return 0
	}

	return itm.contentVersion
}

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

	itm, ok := v.items.Get(v.activeXpath)
	if !ok {
		return nil, false
	}

	if itm.contentBuf == nil {
		return nil, false
	}

	return itm.contentBuf.Lines(), true
}

func (v *Viewports) GetActiveInnerViewportXpath() xpath.Xpath {
	if v.HasActiveInner() {
		return v.activeXpath
	}

	return xpath.Xpath{}
}

func (v *Viewports) GetViewportContent(xp xpath.Xpath) *buffer.LinesBuf {
	itm, ok := v.items.Get(xp)
	if ok {
		return itm.contentBuf
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
	v.lastActiveXpath = v.mainXpath
	v.lastActiveYOffset = 0
	v.shrinkTickAt = time.Time{}
	v.shrinkTickGeneration++
	v.hasPendingShrink = false
	v.dirty = false
}

// RemoveIfExistsViewport removes a viewport by xpath.

func (v *Viewports) RemoveIfExistsViewport(xpath xpath.Xpath) {
	itm, ok := v.items.Get(xpath)
	if ok {
		itm.release()
	}

	v.items.Del(xpath)

	if v.activeXpath == xpath {
		v.activeXpath = v.mainXpath
	}
}

// Update

// shrinkTickMsg fires when a held auto height may be released.
type shrinkTickMsg struct {
	generation uint64
}

func (v *Viewports) Update(msg zeroterm.Msg) zeroterm.Cmd {
	if v == nil {
		return nil
	}

	switch msg := msg.(type) {
	case shrinkTickMsg:
		// A tick from a replaced timer must not disturb the armed one.
		if msg.generation != v.shrinkTickGeneration {
			return nil
		}

		// Clear the armed tick even when the deadline moved (content grew) or
		// the timer fired a hair early, so the next post-render can rearm it.
		v.shrinkTickAt = time.Time{}
		v.releaseDueShrinks(v.now())
		v.checkDirty()

		return nil
	case zeroterm.PostRenderMsg:
		return v.scheduleShrinkTick()
	}

	click, ok := msg.(zeroterm.MouseClickMsg)
	if ok {
		clicked := v.clickTarget(click)
		if clicked.Depth() > 0 {
			v.activeXpath = clicked
		} else {
			v.activeXpath = v.mainXpath
		}

		v.checkDirty()

		return nil
	}

	itm := v.activeItem()
	if itm != nil {
		itm.model.Update(msg)
	}

	v.checkDirty()

	return nil
}

// AllowShrink releases the auto-height hold on a viewport. The viewport resizes
// on its next sync; callers use this when the content is final, such as when a
// command finishes.
func (v *Viewports) AllowShrink(xp xpath.Xpath) {
	if v == nil {
		return
	}

	itm, ok := v.items.Get(xp)
	if !ok {
		return
	}

	itm.releaseShrink()
}

// ConsumeDirty returns true if viewport state changed since last call, then resets the flag.
func (v *Viewports) ConsumeDirty() bool {
	d := v.dirty
	v.dirty = false

	return d
}

// Render methods

func (v *Viewports) RenderViewportVersioned(xp xpath.Xpath, content *buffer.LinesBuf, version uint64, indent int) *buffer.LinesBuf {
	return v.render(xp, content, indent, 0, true, true, false, version)
}

func (v *Viewports) RenderLabelViewport(xp xpath.Xpath, content *buffer.LinesBuf, version uint64, indent int) *buffer.LinesBuf {
	return v.render(xp, content, indent, 0, false, false, true, version)
}

func (v *Viewports) RenderMainViewport(content *buffer.LinesBuf, version uint64, explicitHeight int) *buffer.LinesBuf {
	return v.render(v.mainXpath, content, 0, explicitHeight, false, true, false, version)
}

func (v *Viewports) RenderFullscreenViewport(
	xpath xpath.Xpath,
	content *buffer.LinesBuf, version uint64, footerHeaderHeight int,
) *buffer.LinesBuf {
	height := max(1, v.dimensions.Height-footerHeaderHeight)

	return v.render(xpath, content, 0, height, true, true, false, version)
}

// Debug

func (v *Viewports) Debug(buf *buffer.LinesBuf) {
	buf.EmptyLine()
	buf.WriteString(fmt.Sprintf("Viewports: %d (%dx%d) ContentWidth=%d", v.items.Len(), v.dimensions.Width, v.dimensions.Height, v.ContentWidth()))

	v.items.ForEach(func(xpathKey xpath.Xpath, itm *item) bool {
		mdl := itm.model
		buf.WriteLine(fmt.Appendf(nil, "  '%s': %dx%d l:%d sb:%v sr:%v main:%v overflows:%v",
			xpathKey, mdl.Width(), mdl.Height(), mdl.TotalLineCount(),
			mdl.HasScrollbar(), mdl.HasScrollbarReserve(), mdl.IsMain(),
			mdl.TotalLineCount() > mdl.Height()))

		if xpathKey == v.activeXpath {
			buf.AppendToLine([]byte(" [A]"))
		}

		if mdl.ScrollPercent() == 1 {
			buf.AppendToLine([]byte(" @btm"))
		}

		return true
	})
}

func (v *Viewports) checkDirty() {
	if v.activeXpath != v.lastActiveXpath {
		v.dirty = true
		v.lastActiveXpath = v.activeXpath
	}

	itm := v.activeItem()
	if itm != nil {
		yOffset := itm.model.YOffset()
		if yOffset != v.lastActiveYOffset {
			v.dirty = true
			v.lastActiveYOffset = yOffset
		}
	}
}

// Internal

// scheduleShrinkTick arms a single tick for the earliest held height. It is
// called after every render and only schedules once until that tick fires.
func (v *Viewports) scheduleShrinkTick() zeroterm.Cmd {
	next, ok := v.nextShrinkAt()
	if !ok {
		v.shrinkTickAt = time.Time{}

		return nil
	}

	if !v.shrinkTickAt.IsZero() && !next.Before(v.shrinkTickAt) {
		return nil
	}

	v.shrinkTickAt = next
	v.shrinkTickGeneration++
	generation := v.shrinkTickGeneration
	delay := max(next.Sub(v.now()), 0)

	return zeroterm.TickCmd(delay, func(time.Time) zeroterm.Msg {
		return shrinkTickMsg{generation: generation}
	})
}

func (v *Viewports) nextShrinkAt() (time.Time, bool) {
	if !v.hasPendingShrink {
		return time.Time{}, false
	}

	var next time.Time

	v.items.ForEach(func(_ xpath.Xpath, itm *item) bool {
		if itm.shrinkSince.IsZero() {
			return true
		}

		deadline := itm.shrinkSince.Add(autoShrinkDelay)
		if next.IsZero() || deadline.Before(next) {
			next = deadline
		}

		return true
	})

	v.hasPendingShrink = !next.IsZero()

	return next, v.hasPendingShrink
}

// releaseDueShrinks drops the height hold of every item whose grace period
// expired, invalidating their cached renders so the next frame resizes them.
func (v *Viewports) releaseDueShrinks(now time.Time) {
	v.items.ForEach(func(_ xpath.Xpath, itm *item) bool {
		if itm.shrinkSince.IsZero() || now.Before(itm.shrinkSince.Add(autoShrinkDelay)) {
			return true
		}

		if !itm.model.PendingShrink() {
			itm.shrinkSince = time.Time{}

			return true
		}

		itm.releaseShrink()

		v.dirty = true

		return true
	})
}

func (v *Viewports) render(
	xpath xpath.Xpath,
	contentBuf *buffer.LinesBuf,
	indent, explicitHeight int,
	bordered, scrollbar, highlightActive bool,
	version uint64,
) *buffer.LinesBuf {
	active := xpath == v.activeXpath
	itm := v.getOrCreateItem(xpath, indent, explicitHeight, bordered, scrollbar)

	activity := itm.contentVersion != version
	contentChanged := activity || itm.contentBuf != contentBuf
	v.resolveContent(itm, version, contentBuf)

	if bordered {
		itm.model.SetBorderStyle(v.borderColor(active))
	}

	width := v.viewWidth(indent, 0)
	dimsChanged := width != itm.lastWidth || explicitHeight != itm.lastHeight

	if contentChanged || dimsChanged {
		releaseAutoHold(itm, width, explicitHeight, xpath == v.mainXpath)

		itm.lastWidth = width
		itm.lastHeight = explicitHeight

		err := itm.model.SyncBuf(itm.contentBuf, width, explicitHeight)
		if err != nil {
			fmt.Fprintf(os.Stderr, "viewport: %v\n", err)
		}

		v.trackShrink(itm, activity)

		if xpath == v.mainXpath {
			itm.contentBuf = nil
		}
	}

	rendered := itm.model.Render()

	if highlightActive && active {
		v.highlightBuf.Reset()
		v.selectionHighlightBackground.RenderIntoBuf(v.highlightBuf, rendered)
		rendered = v.highlightBuf
	}

	itm.zonedOutput.Reset()
	itm.zoneID.MarkLines(rendered, itm.zonedOutput)

	return itm.zonedOutput
}

// releaseAutoHold drops an auto-height hold after a width change reflowed the
// content, so the height adapts immediately instead of on the shrink tick. The
// main viewport is auto-sized for the one-shot final render (and when the
// terminal is shorter than the header and footer) and has no grace-release
// cycle, so it must follow content immediately.
func releaseAutoHold(itm *item, width, explicitHeight int, isMain bool) {
	if explicitHeight > 0 {
		return
	}

	if isMain || width != itm.lastWidth {
		itm.releaseHeight()
	}
}

// trackShrink starts (or restarts) the grace period whenever a synced viewport
// is holding more height than its content needs. Any content update while the
// height is held counts as activity, so shrinking only happens once output has
// been quiet for the whole grace period. Invalidations that only refresh a
// cached snapshot keep the original deadline.
func (v *Viewports) trackShrink(itm *item, active bool) {
	if !itm.model.PendingShrink() {
		itm.shrinkSince = time.Time{}

		return
	}

	v.hasPendingShrink = true

	if active || itm.shrinkSince.IsZero() {
		itm.shrinkSince = v.now()
	}
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

func (v *Viewports) resolveContent(itm *item, version uint64, contentBuf *buffer.LinesBuf) {
	if itm.contentBuf != nil && itm.contentBuf != contentBuf {
		itm.contentBuf.Release()
	}

	itm.contentBuf = contentBuf
	itm.contentVersion = version
}

func (v *Viewports) getOrCreateItem(
	xpath xpath.Xpath,
	indent, explicitHeight int,
	bordered, scrollbar bool,
) *item {
	itm, exists := v.items.Get(xpath)
	if exists {
		hasBorder := itm.model.IsBordered()
		hasScrollbar := itm.model.HasScrollbar()

		if hasBorder != bordered || hasScrollbar != scrollbar {
			savedContent := itm.contentBuf
			savedVersion := itm.contentVersion
			itm.contentBuf = nil
			itm.release()
			v.items.Del(xpath)

			itm = &item{
				model:          tuiviewport.New(v.buildViewportOpts(xpath, indent, explicitHeight, 0, bordered, scrollbar)...),
				contentBuf:     savedContent,
				contentVersion: savedVersion,
				zoneID:         zeroterm.NewZoneID(),
				zonedOutput:    buffer.NewLinesBuf(),
			}

			if xpath != v.mainXpath {
				itm.model.GotoBottom()
			}

			v.items.Set(xpath, itm)

			return itm
		} else {
			return itm
		}
	}

	itm = &item{
		model:       tuiviewport.New(v.buildViewportOpts(xpath, indent, explicitHeight, 0, bordered, scrollbar)...),
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
		tuiviewport.WithBorderGlyphs(v.borderGlyphs),
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
		return xpath.Xpath{}
	}

	clickedID, ok := zeroterm.ZoneIDAtCol(click.Lines.Line(click.Y), click.X)
	if !ok {
		return xpath.Xpath{}
	}

	var candidates []xpath.Xpath

	v.items.ForEach(func(k xpath.Xpath, itm *item) bool {
		if itm.zoneID.Equal(clickedID) {
			candidates = append(candidates, k)
		}

		return true
	})

	if len(candidates) == 0 {
		return xpath.Xpath{}
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Depth() > candidates[j].Depth() })

	return candidates[0]
}
