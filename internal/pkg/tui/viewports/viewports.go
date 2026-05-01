package viewports

import (
	"fmt"
	"image/color"
	"os"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	tuiviewport "github.com/mihakrumpestar/panix/internal/pkg/tui/viewport"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
)

type Dimensions struct{ Width, Height int }

type Viewports struct {
	items        *atomicorderedmap.AtomicOrderedMap[xpath.Xpath, *item]
	dimensions   *Dimensions
	conf         *config.Config
	fullscreenXp xpath.Xpath
	mainXpath    xpath.Xpath
	activeXpath  xpath.Xpath
}

type item struct {
	model          tuiviewport.Viewport
	content        string
	contentVersion uint64
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

	return ""
}

func (v *Viewports) GetViewportContent(xp xpath.Xpath) string {
	it, ok := v.items.Get(xp)
	if ok {
		return it.content
	}

	return ""
}

func (v *Viewports) DeselectAll() { v.activeXpath = v.mainXpath }

// RemoveIfExistsViewport removes a viewport by xpath.

func (v *Viewports) RemoveIfExistsViewport(xp xpath.Xpath) {
	v.items.Del(xp)

	if v.activeXpath == xp {
		v.activeXpath = v.mainXpath
	}
}

// Update

func (v *Viewports) Update(msg tea.Msg) tea.Cmd {
	if v == nil {
		return nil
	}

	click, ok := msg.(tea.MouseClickMsg)
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

func (v *Viewports) GetOrCreateViewportVersioned(xp xpath.Xpath, output *command.AtomicCommandOutput, indent int) string {
	return v.render(xp, "", indent, 0, 0, true, true, false, output, 0)
}

func (v *Viewports) GetOrCreateLabelViewport(xp xpath.Xpath, content string, version uint64, indent int) string {
	return v.render(xp, content, indent, 0, 0, false, false, true, nil, version)
}

func (v *Viewports) GetOrCreateMainViewport(content string, version uint64, footerHeaderHeight int) string {
	height := v.dimensions.Height - footerHeaderHeight

	return v.render(v.mainXpath, content, 0, height, 0, false, true, false, nil, version) + "\n"
}

func (v *Viewports) RenderFullscreenViewport(xp xpath.Xpath, content string, version uint64, footerHeaderHeight int) string {
	height := max(1, v.dimensions.Height-footerHeaderHeight)
	width := max(1, v.dimensions.Width)

	return v.render(xp, content, 0, height, width, true, true, false, nil, version) + "\n"
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
	content string,
	indent, explicitHeight, explicitWidth int,
	bordered, scrollbar, highlightActive bool,
	output *command.AtomicCommandOutput,
	version uint64,
) string {
	active := xpath == v.activeXpath
	itm := v.getOrCreateItem(xpath, content, indent, explicitHeight, explicitWidth, bordered, scrollbar)

	version = v.resolveVersion(output, version)
	content = v.resolveContent(itm, version, content, output)

	if bordered {
		itm.model.SetBorderStyle(v.borderColor(active))
	}

	width := v.viewWidth(indent, explicitWidth)

	err := itm.model.Sync(content, width, explicitHeight)
	if err != nil {
		fmt.Fprintf(os.Stderr, "viewport: %v\n", err)
	}

	view := itm.model.View()

	if highlightActive && active {
		view = v.conf.ColorScheme.Table.SelectionHighlightBackground.Render(view)
	}

	return zone.Mark(xpath.String(), view)
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

func (v *Viewports) resolveVersion(output *command.AtomicCommandOutput, version uint64) uint64 {
	if output != nil {
		return output.Version()
	}

	return version
}

func (v *Viewports) resolveContent(itm *item, version uint64, content string, output *command.AtomicCommandOutput) string {
	if itm.contentVersion == version {
		return itm.content
	}

	if output != nil {
		itm.content = output.String()
	} else {
		itm.content = content
	}

	itm.contentVersion = version

	return itm.content
}

func (v *Viewports) getOrCreateItem(
	xpath xpath.Xpath,
	content string,
	indent, explicitHeight, explicitWidth int,
	bordered, scrollbar bool,
) *item {
	itm, exists := v.items.Get(xpath)
	if exists {
		return itm
	}

	opts := []tuiviewport.Option{
		tuiviewport.WithWidth(v.viewWidth(indent, explicitWidth)),
		tuiviewport.WithHeight(max(1, explicitHeight)),
	}

	if scrollbar {
		sbColor := v.borderColor(false)
		opts = append(opts, tuiviewport.WithScrollbar("█", "│", sbColor, sbColor))
	}

	if bordered {
		opts = append(opts, tuiviewport.WithBorder(v.borderColor(false)))
	}

	if bordered && scrollbar {
		maxH := v.conf.Flags.Tui.CommandOutputMaxHeight
		if maxH > 0 {
			opts = append(opts, tuiviewport.WithMaxHeight(maxH))
		}
	}

	// Main viewport: indent==0 and explicitWidth==0 and it's the mainXpath
	if xpath == v.mainXpath {
		opts = append(opts, tuiviewport.WithMain())
	}

	itm = &item{
		model:   tuiviewport.New(opts...),
		content: content,
	}

	if xpath != v.mainXpath {
		itm.model.GotoBottom()
	}

	v.items.Set(xpath, itm)

	return itm
}

func (v *Viewports) borderColor(active bool) color.Color {
	if active {
		return v.conf.ColorScheme.Table.SelectionHighlightBorder.GetBackground()
	}

	return v.conf.ColorScheme.Table.Border.GetForeground()
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

func (v *Viewports) clickTarget(m tea.MouseClickMsg) xpath.Xpath {
	var candidates []xpath.Xpath

	for xp := range v.items.Records() {
		if zone.Get(xp.String()).InBounds(m) {
			candidates = append(candidates, xp)
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Depth() > candidates[j].Depth() })

	return candidates[0]
}
