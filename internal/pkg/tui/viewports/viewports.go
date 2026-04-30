package viewports

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/logs/command"
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

type Viewports struct {
	items      *atomicorderedmap.AtomicOrderedMap[xpath.Xpath, *item]
	dimensions *Dimensions
	conf       *config.Config
	activation activation
}

type activation struct {
	fullscreen  xpath.Xpath
	mainXpath   xpath.Xpath
	activeXpath xpath.Xpath
}

type viewportCacheKey struct {
	width     int
	height    int
	scrollPct float64
	version   uint64
}

type item struct {
	model          viewport.Model
	content        string
	contentVersion uint64
	cache          cache.Cache[string, viewportCacheKey]
}

type renderConfig struct {
	bordered        bool
	highlightActive bool
	maxHeight       int
	explicitHeight  int
	explicitWidth   int
	indent          int
	output          *command.AtomicCommandOutput
	version         uint64
}

func NewViewports(dimensions *Dimensions, conf *config.Config) *Viewports {
	mainXpath := xpath.New("main")

	return &Viewports{
		items:      atomicorderedmap.New[xpath.Xpath, *item](),
		dimensions: dimensions,
		conf:       conf,
		activation: activation{
			mainXpath:   mainXpath,
			activeXpath: mainXpath,
		},
	}
}

// Fullscreen

func (v *Viewports) IsFullscreen() bool {
	return v.activation.fullscreen.Depth() > 0
}

func (v *Viewports) GetFullscreenXpath() xpath.Xpath {
	return v.activation.fullscreen
}

func (v *Viewports) SetFullscreen(xp xpath.Xpath) {
	v.activation.fullscreen = xp
}

func (v *Viewports) ExitFullscreen() {
	v.activation.fullscreen = xpath.New()
}

// Dimensions

func (v *Viewports) ContentWidth() int { return v.dimensions.Width - scrollbarWidth }

// Factory methods

func (v *Viewports) GetOrCreateViewportVersioned(xp xpath.Xpath, output *command.AtomicCommandOutput, indent int) string {
	return v.render(xp, "", renderConfig{
		bordered:  true,
		maxHeight: v.conf.Flags.Tui.CommandOutputMaxHeight,
		indent:    indent,
		output:    output,
	})
}

func (v *Viewports) GetOrCreateLabelViewport(xp xpath.Xpath, content string, version uint64, indent int) string {
	return v.render(xp, content, renderConfig{
		highlightActive: true,
		indent:          indent,
		version:         version,
	})
}

func (v *Viewports) GetOrCreateMainViewport(content string, version uint64, footerHeaderHeight int) string {
	h := v.dimensions.Height - footerHeaderHeight

	return v.render(v.activation.mainXpath, content, renderConfig{
		explicitHeight: h,
		version:        version,
	}) + "\n"
}

func (v *Viewports) RenderFullscreenViewport(xp xpath.Xpath, content string, version uint64, footerHeaderHeight int) string {
	h := max(1, v.dimensions.Height-footerHeaderHeight-borderOverhead)
	w := max(1, v.dimensions.Width-scrollbarWidth-borderOverhead)

	return v.render(xp, content, renderConfig{
		bordered:       true,
		explicitHeight: h,
		explicitWidth:  w,
		version:        version,
	}) + "\n"
}

func (v *Viewports) RemoveIfExistsViewport(xp xpath.Xpath) {
	v.items.Del(xp)

	if v.activation.activeXpath == xp {
		v.activation.activeXpath = v.activation.mainXpath
	}
}

// Active queries

func (v *Viewports) HasActiveInner() bool {
	return v.activation.activeXpath.Depth() > 0 && v.activation.activeXpath != v.activation.mainXpath
}

func (v *Viewports) GetActiveInnerViewportContent() (string, bool) {
	if !v.HasActiveInner() {
		return "", false
	}

	it, ok := v.items.Get(v.activation.activeXpath)
	if !ok {
		return "", false
	}

	return it.content, true
}

func (v *Viewports) GetActiveInnerViewportXpath() xpath.Xpath {
	if v.HasActiveInner() {
		return v.activation.activeXpath
	}

	return ""
}

func (v *Viewports) GetViewportContent(xp xpath.Xpath) string {
	it, ok := v.items.Get(xp)
	if ok {
		return it.content
	}

	return ""
}

func (v *Viewports) DeselectAll() {
	v.activation.activeXpath = v.activation.mainXpath
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
			v.activation.activeXpath = clicked
		} else {
			v.activation.activeXpath = v.activation.mainXpath
		}

		return nil
	}

	itm := v.activeViewport()
	if itm == nil {
		return nil
	}

	var cmd tea.Cmd

	itm.model, cmd = itm.model.Update(msg)

	return cmd
}

// Debug

func (v *Viewports) Debug() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "\nViewports: %d (%dx%d)\n", v.items.Len(), v.dimensions.Width, v.dimensions.Height)

	for _, pair := range v.items.Pairs() {
		fmt.Fprintf(&builder, "  '%s': %dx%d l:%d", pair.Key, pair.Value.model.Width(), pair.Value.model.Height(), pair.Value.model.TotalLineCount())

		if pair.Key == v.activation.activeXpath {
			builder.WriteString(" [A]")
		}

		if pair.Value.model.ScrollPercent() == 1 {
			builder.WriteString(" @btm")
		}

		builder.WriteByte('\n')
	}

	return builder.String()
}

func (v *Viewports) viewWidth(cfg renderConfig) int {
	if cfg.explicitWidth > 0 {
		return cfg.explicitWidth
	}

	return max(1, v.dimensions.Width-cfg.indent-scrollbarWidth)
}

func (v *Viewports) finalHeight(totalLines int, cfg renderConfig) int {
	if cfg.explicitHeight > 0 {
		return max(1, cfg.explicitHeight)
	}

	if cfg.maxHeight > 0 && totalLines > cfg.maxHeight {
		return cfg.maxHeight
	}

	return totalLines
}

func (v *Viewports) syncItem(itm *item, xp xpath.Xpath, content string, cfg renderConfig) {
	wasAtBottom := itm.model.ScrollPercent() == 1 && xp != v.activation.mainXpath
	yOffset := itm.model.YOffset()

	width := v.viewWidth(cfg)
	itm.model.SetWidth(width)
	itm.model.SetContent(lipgloss.Wrap(content, width, ""))

	totalLines := max(1, itm.model.TotalLineCount())

	if cfg.maxHeight > 0 && totalLines > cfg.maxHeight {
		contentW := max(1, width-scrollbarWidth)
		itm.model.SetWidth(contentW)
		itm.model.SetContent(lipgloss.Wrap(content, contentW, ""))
		totalLines = max(1, itm.model.TotalLineCount())
	}

	finalH := v.finalHeight(totalLines, cfg)
	itm.model.SetHeight(finalH)

	if wasAtBottom {
		itm.model.GotoBottom()
	} else {
		itm.model.SetYOffset(min(yOffset, max(0, totalLines-finalH)))
	}
}

func (v *Viewports) render(xpath xpath.Xpath, content string, cfg renderConfig) string {
	version := v.resolveVersion(cfg)

	itm := v.getOrCreateItem(xpath, content, cfg)
	content = v.resolveContent(itm, version, content, cfg)

	width := v.viewWidth(cfg)

	if itm.contentVersion == version && itm.model.Width() == width {
		key := viewportCacheKey{
			width:     width,
			height:    itm.model.Height(),
			scrollPct: itm.model.ScrollPercent(),
			version:   version,
		}

		view, hit := itm.cache.GetCheck(key)
		if hit {
			active := xpath == v.activation.activeXpath
			view = v.applyActiveStyle(view, cfg, active)
			view = zone.Mark(xpath.String(), view)

			return view
		}
	}

	v.syncItem(itm, xpath, content, cfg)

	key := viewportCacheKey{
		width:     itm.model.Width(),
		height:    itm.model.Height(),
		scrollPct: itm.model.ScrollPercent(),
		version:   version,
	}

	view := itm.cache.Get(
		func() (string, bool) {
			scrollbar, _ := v.scrollbar(itm.model.ScrollPercent(), itm.model.TotalLineCount(), itm.model.Height())

			return v.withScrollbar(itm.model.View(), scrollbar), true
		},
		key,
	)

	active := xpath == v.activation.activeXpath
	view = v.applyActiveStyle(view, cfg, active)
	view = zone.Mark(xpath.String(), view)

	return view
}

func (v *Viewports) resolveVersion(cfg renderConfig) uint64 {
	if cfg.output != nil {
		return cfg.output.Version()
	}

	return cfg.version
}

func (v *Viewports) resolveContent(itm *item, version uint64, currentContent string, cfg renderConfig) string {
	if itm.contentVersion != version {
		if cfg.output != nil {
			itm.content = cfg.output.String()
		} else {
			itm.content = currentContent
		}

		itm.contentVersion = version
	}

	return itm.content
}

func (v *Viewports) getOrCreateItem(xpath xpath.Xpath, content string, cfg renderConfig) *item {
	itm, exists := v.items.Get(xpath)
	if exists {
		return itm
	}

	w := v.viewWidth(cfg)
	h := max(1, cfg.explicitHeight)

	itm = &item{
		model:   viewport.New(viewport.WithWidth(w), viewport.WithHeight(h)),
		content: content,
	}
	itm.model.GotoBottom()
	v.items.Set(xpath, itm)

	return itm
}

func (v *Viewports) applyActiveStyle(view string, cfg renderConfig, active bool) string {
	switch {
	case cfg.highlightActive:
		if active {
			return v.conf.ColorScheme.Table.SelectionHighlightBackground.Render(view)
		}

		return view
	case cfg.bordered:
		border := v.conf.ColorScheme.Table.Border.GetForeground()
		if active {
			border = v.conf.ColorScheme.Table.SelectionHighlightBorder.GetBackground()
		}

		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Render(view)
	default:
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
func (v *Viewports) withScrollbar(view, bar string) string {
	if bar == "" {
		return view
	}

	vLines := strings.Split(view, "\n")
	bLines := strings.Split(bar, "\n")
	result := make([]string, len(vLines))

	for i, line := range vLines {
		if i < len(bLines) {
			result[i] = line + " " + bLines[i]
		} else {
			result[i] = line
		}
	}

	return strings.Join(result, "\n")
}

// Input helpers

func (v *Viewports) activeViewport() *item {
	activeXpath := v.activation.activeXpath

	if activeXpath.String() == "" {
		activeXpath = v.activation.mainXpath
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
		return ""
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
