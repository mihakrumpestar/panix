package viewports

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kirill-scherba/omap"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
)

const (
	scrollThumb      = "█"
	scrollTrack      = "│"
	scrollbarWidth   = 2
	borderHeight     = 2
	borderWidth      = 2
	minTerminalWidth = 40
)

// Dimensions represents terminal size
type Dimensions struct{ Width, Height int }

// Viewports manages all viewport instances
type Viewports struct {
	viewports              *omap.Omap[attributes.Xpath, *Viewport]
	dimensions             *Dimensions
	colors                 *config.ColorScheme
	debug                  *strings.Builder
	fullscreenXpath        attributes.Xpath
	commandOutputMaxHeight int
	mainXpath              attributes.Xpath
	footerHeight           int
}

// Viewport wraps a bubbletea viewport with additional state
type Viewport struct {
	model         viewport.Model
	active        bool
	content       string
	scrollbarZone attributes.Xpath
}

// NewViewports creates a new viewport manager
func NewViewports(dimensions *Dimensions, colors *config.ColorScheme, dbg *strings.Builder, maxHeight int) *Viewports {
	viewportsMap, _ := omap.New[attributes.Xpath, *Viewport]()
	// Ensure minimum height of 1
	if maxHeight < 1 {
		maxHeight = 1
	}

	return &Viewports{
		viewports:              viewportsMap,
		dimensions:             dimensions,
		colors:                 colors,
		debug:                  dbg,
		commandOutputMaxHeight: maxHeight,
		mainXpath:              attributes.NewXpath("main"),
	}
}

// Fullscreen management
func (v *Viewports) IsFullscreen() bool                   { return v.fullscreenXpath.Depth() > 0 }
func (v *Viewports) GetFullscreenXpath() attributes.Xpath { return v.fullscreenXpath }
func (v *Viewports) SetFullscreen(xpath attributes.Xpath) { v.fullscreenXpath = xpath }
func (v *Viewports) ExitFullscreen()                      { v.fullscreenXpath = attributes.Xpath{} }

// ContentWidth returns available width accounting for scrollbar
func (v *Viewports) ContentWidth() int { return v.dimensions.Width - scrollbarWidth }

// Viewport factory methods

func (v *Viewports) GetOrCreateViewport(xpath attributes.Xpath, content string, indent int) string {
	return v.createViewport(xpath, content, indent, viewportOptions{
		maxHeight:   v.commandOutputMaxHeight,
		wrapContent: true,
		useBorder:   true,
	})
}

func (v *Viewports) GetOrCreateLabelViewport(xpath attributes.Xpath, content string, indent int) string {
	return v.createViewport(xpath, content, indent, viewportOptions{wrapContent: true, noPadding: true})
}

func (v *Viewports) GetOrCreateMainViewport(content string, footerHeight int) string {
	v.footerHeight = footerHeight
	h := v.dimensions.Height - footerHeight
	return v.createViewport(v.mainXpath, content, 0, viewportOptions{
		height:    h,
		maxHeight: h,
		full:      true,
	})
}

func (v *Viewports) RenderFullscreenViewport(xpath attributes.Xpath, content string, footerHeight int) string {
	v.footerHeight = footerHeight
	height := max(1, v.dimensions.Height-footerHeight-borderHeight)
	width := max(1, v.dimensions.Width-scrollbarWidth-borderWidth)

	viewport, exists := v.viewports.Get(xpath)
	if !exists {
		return v.createViewport(xpath, content, 0, viewportOptions{
			availableWidth: width, height: height, maxHeight: height,
			wrapContent: true, useBorder: true, full: true,
		})
	}

	yOffset := viewport.model.YOffset()
	viewport.model.SetWidth(width)
	viewport.model.SetHeight(height)
	viewport.content = content
	proc := lipgloss.NewStyle().Width(width).Render(content)
	viewport.model.SetContent(proc)
	viewport.model.SetYOffset(min(yOffset, max(0, lipgloss.Height(proc)-height)))

	return v.renderViewport(viewport, height, true, false)
}

func (v *Viewports) RemoveIfExistsViewport(xpath attributes.Xpath) { v.viewports.Del(xpath) }

// Active viewport queries

func (v *Viewports) GetActiveViewportContent() string {
	if vp := v.getActiveViewport(); vp != nil {
		return vp.content
	}

	return ""
}

func (v *Viewports) GetActiveInnerViewportContent() (string, bool) {
	for xpath, vp := range v.viewports.Records() {
		if vp.active && xpath != v.mainXpath {
			return vp.content, true
		}
	}

	return "", false
}

func (v *Viewports) GetActiveInnerViewportXpath() attributes.Xpath {
	for xpath, vp := range v.viewports.Records() {
		if vp.active && xpath != v.mainXpath {
			return xpath
		}
	}

	return attributes.Xpath{}
}

func (v *Viewports) GetViewportContent(xpath attributes.Xpath) string {
	if vp, ok := v.viewports.Get(xpath); ok {
		return vp.content
	}

	return ""
}

func (v *Viewports) GetActiveViewportScrollPercent() float64 {
	if v.IsFullscreen() {
		if vp, ok := v.viewports.Get(v.fullscreenXpath); ok {
			return vp.model.ScrollPercent()
		}
	}

	if vp := v.getActiveViewport(); vp != nil {
		return vp.model.ScrollPercent()
	}

	return 0.0
}

// Update handles messages for all viewports
func (v *Viewports) Update(msg tea.Msg) tea.Cmd {
	if v == nil {
		return nil
	}

	var cmds []tea.Cmd

	switch msgVal := msg.(type) {
	case tea.MouseMsg:
		v.handleMouse(msgVal)
	case tea.KeyPressMsg:
		v.handleKey(msgVal)

		return tea.Batch(cmds...)
	case tea.WindowSizeMsg:
		// Update dimensions and resize all viewports
		v.dimensions.Width = max(msgVal.Width, minTerminalWidth)
		v.dimensions.Height = msgVal.Height
		v.resizeAllViewports()

		return tea.Batch(cmds...)
	}

	hasActiveInner := v.hasActiveInner()

	for _, viewport := range v.viewports.Records() {
		if viewport.active {
			if updated, cmd := viewport.model.Update(msg); cmd != nil {
				viewport.model = updated

				cmds = append(cmds, cmd)
			}
		}
	}

	if mainVpr, ok := v.viewports.Get(v.mainXpath); ok && !hasActiveInner {
		if updated, cmd := mainVpr.model.Update(msg); cmd != nil {
			mainVpr.model = updated

			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

// Debug returns debug info about all viewports
func (v *Viewports) Debug() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("\nViewports: %d (%dx%d)\n", v.viewports.Len(), v.dimensions.Width, v.dimensions.Height))

	for xpath, viewport := range v.viewports.Records() {
		builder.WriteString(fmt.Sprintf("  '%s': %dx%d c:%d", xpath, viewport.model.Width(), viewport.model.Height(), lipgloss.Height(viewport.content)))

		if viewport.active {
			builder.WriteString(" [A]")
		}

		if viewport.model.ScrollPercent() == 1 {
			builder.WriteString(" @btm")
		}

		builder.WriteString("\n")
	}

	return builder.String()
}

// resizeAllViewports updates all viewport dimensions when terminal resizes
func (v *Viewports) resizeAllViewports() {
	for xpath, viewport := range v.viewports.Records() {
		width := max(1, v.dimensions.Width-scrollbarWidth)
		height := max(1, v.dimensions.Height-v.footerHeight)

		if xpath == v.mainXpath {
			// Main viewport gets full height minus footer
			viewport.model.SetWidth(width)
			viewport.model.SetHeight(height)
		} else {
			// Inner viewports: update width, keep existing height or use max height
			viewport.model.SetWidth(width)

			if v.commandOutputMaxHeight > 0 && viewport.model.Height() > v.commandOutputMaxHeight {
				viewport.model.SetHeight(v.commandOutputMaxHeight)
			}
		}
	}
}

// Internal types and helpers

type viewportOptions struct {
	availableWidth int
	height         int
	maxHeight      int
	wrapContent    bool
	useBorder      bool
	full           bool
	noPadding      bool
}

func (v *Viewports) createViewport(xpath attributes.Xpath, content string, indent int, opts viewportOptions) string {
	width := opts.availableWidth
	if width == 0 {
		width = max(10, v.dimensions.Width-indent-scrollbarWidth)
	}

	height := max(1, opts.height)

	viewportInstance, exists := v.viewports.Get(xpath)
	if !exists {
		viewportInstance = &Viewport{
			model:         viewport.New(viewport.WithWidth(width), viewport.WithHeight(height)),
			scrollbarZone: xpath.NewXpathWithAppend("scrollbar"),
			content:       content,
			active:        xpath == v.mainXpath,
		}
		viewportInstance.model.GotoBottom()
		v.viewports.Set(xpath, viewportInstance)
	} else {
		viewportInstance.content = content
	}

	proc := v.processContent(content, width, opts.wrapContent, opts.noPadding)
	contentHeight := lipgloss.Height(proc)

	if contentHeight > opts.maxHeight && opts.maxHeight > 0 && !opts.full {
		width = max(1, width-scrollbarWidth)
		proc = v.processContent(content, width, opts.wrapContent, opts.noPadding)
		contentHeight = lipgloss.Height(proc)
	}

	finalHeight := contentHeight
	if !opts.full && opts.maxHeight > 0 && contentHeight > opts.maxHeight {
		finalHeight = opts.maxHeight
	} else if opts.full {
		finalHeight = height
	}

	viewportInstance.model.SetWidth(width)
	viewportInstance.model.SetHeight(finalHeight)

	yOffset, pct := viewportInstance.model.YOffset(), viewportInstance.model.ScrollPercent()
	viewportInstance.model.SetContent(proc)

	if pct == 1 {
		viewportInstance.model.GotoBottom()
	} else {
		maxOffset := max(0, lipgloss.Height(proc)-finalHeight)
		viewportInstance.model.SetYOffset(min(yOffset, maxOffset))
	}

	return zone.Mark(xpath.String(), v.renderViewport(viewportInstance, finalHeight, opts.useBorder, opts.noPadding))
}

func (v *Viewports) processContent(content string, width int, wrap bool, noPadding bool) string {
	if wrap {
		if noPadding {
			return lipgloss.NewStyle().Width(width).Align(lipgloss.Left).Render(content)
		}

		return lipgloss.NewStyle().Width(width).Render(content)
	}

	return truncateLines(content, width)
}

func (v *Viewports) renderViewport(viewport *Viewport, height int, useBorder bool, noPadding bool) string {
	contentHeight := lipgloss.Height(viewport.content)
	scrollbar, _ := v.renderScrollbar(viewport.model.ScrollPercent(), contentHeight, height)
	combined := v.combineWithScrollbar(viewport.model.View(), scrollbar, viewport.scrollbarZone)

	if noPadding {
		if viewport.active {
			combined = v.colors.SelectionHighlightBackground.Render(combined)
		}

		return combined
	}

	if !useBorder {
		return combined
	}

	borderColor := v.colors.TableBorder.GetForeground()
	if viewport.active {
		borderColor = v.colors.TableBorder.GetBackground()
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Render(combined)
}

func (v *Viewports) renderScrollbar(pct float64, total, visible int) (string, int) {
	if total <= visible {
		return "", 0
	}

	thumb := max(1, int(float64(visible)*float64(visible)/float64(total)))
	maxPos := visible - thumb
	pos := int(float64(maxPos) * clamp(pct, 0, 1))
	endPos := pos + thumb

	var builder strings.Builder
	for i := 0; i < visible; i++ {
		if i > 0 {
			builder.WriteByte('\n')
		}

		if i >= pos && i < endPos {
			builder.WriteString(scrollThumb)
		} else {
			builder.WriteString(scrollTrack)
		}
	}

	return lipgloss.NewStyle().
		Foreground(v.colors.TableBorder.GetForeground()).
		Render(builder.String()), 1
}

func (v *Viewports) combineWithScrollbar(view, bar string, barZone attributes.Xpath) string {
	if bar == "" {
		return view
	}

	vLines := strings.Split(view, "\n")
	bLines := strings.Split(bar, "\n")
	result := make([]string, len(vLines))
	barLen := len(bLines)

	for i, line := range vLines {
		if i < barLen {
			result[i] = line + " " + zone.Mark(barZone.NewXpathWithAppend(strconv.Itoa(i)).String(), bLines[i])
		} else {
			result[i] = line
		}
	}

	return strings.Join(result, "\n")
}

func (v *Viewports) handleMouse(msg tea.MouseMsg) {
	// Handle mouse wheel scrolling first
	if wheel, ok := msg.(tea.MouseWheelMsg); ok {
		if vp := v.getActiveViewport(); vp != nil {
			if wheel.Button == tea.MouseWheelUp {
				vp.model.ScrollUp(3)
			} else {
				vp.model.ScrollDown(3)
			}
		}

		return
	}

	// Handle click-to-activate (only on click messages)
	click, ok := msg.(tea.MouseClickMsg)
	if !ok {
		return
	}

	clicked := v.mostSpecific(v.underMouse(click))
	hasClick := clicked.Depth() > 0

	for xpath, vp := range v.viewports.Records() {
		vp.active = hasClick && xpath == clicked
	}

	if !hasClick {
		if mainVp, ok := v.viewports.Get(v.mainXpath); ok {
			mainVp.active = true
		}
	}
}

func (v *Viewports) handleKey(msg tea.KeyPressMsg) {
	viewport := v.getActiveViewport()
	if viewport != nil {
		switch msg.String() {
		case "up", "k":
			viewport.model.ScrollUp(1)
		case "down", "j":
			viewport.model.ScrollDown(1)
		case "pgup":
			viewport.model.HalfPageUp()
		case "pgdown", "space":
			viewport.model.HalfPageDown()
		case "home", "g":
			viewport.model.GotoTop()
		case "end", "G":
			viewport.model.GotoBottom()
		}
	}
}

func (v *Viewports) getActiveViewport() *Viewport {
	for xpath, vp := range v.viewports.Records() {
		if vp.active && xpath != v.mainXpath {
			return vp
		}
	}

	if mainVp, ok := v.viewports.Get(v.mainXpath); ok {
		return mainVp
	}

	return nil
}

func (v *Viewports) hasActiveInner() bool {
	for xpath, vp := range v.viewports.Records() {
		if xpath != v.mainXpath && vp.active {
			return true
		}
	}

	return false
}

func (v *Viewports) HasActiveInner() bool {
	return v.hasActiveInner()
}

// DeselectAll activates only the main viewport
func (v *Viewports) DeselectAll() {
	for xpath, vp := range v.viewports.Records() {
		vp.active = xpath == v.mainXpath
	}
}

func (v *Viewports) underMouse(m tea.MouseMsg) []attributes.Xpath {
	var result []attributes.Xpath

	for xpath := range v.viewports.Records() {
		if zone.Get(xpath.String()).InBounds(m) {
			result = append(result, xpath)
		}
	}

	return result
}

func (v *Viewports) mostSpecific(xpaths []attributes.Xpath) attributes.Xpath {
	if len(xpaths) == 0 {
		return attributes.Xpath{}
	}

	sort.Slice(xpaths, func(idx, j int) bool { return xpaths[idx].Depth() > xpaths[j].Depth() })

	return xpaths[0]
}

// Utility functions

// truncateLines truncates each line to maxWidth runes (not bytes).
// This is efficient for single-byte encodings but correct for all.
func truncateLines(content string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = truncateToRuneWidth(line, maxWidth)
	}

	return strings.Join(lines, "\n")
}

// truncateToRuneWidth truncates a string to fit within maxWidth runes,
// using lipgloss.Width for accurate display width measurement.
func truncateToRuneWidth(str string, maxWidth int) string {
	width := lipgloss.Width(str)
	if width <= maxWidth {
		return str
	}

	// Binary search for the correct truncation point
	low, high := 0, len(str)
	for low < high {
		mid := (low + high) / 2
		// Find the previous valid UTF-8 boundary
		for mid > low && !utf8.ValidString(str[:mid]) {
			mid--
		}

		if mid <= low {
			break
		}

		w := lipgloss.Width(str[:mid])
		if w > maxWidth {
			high = mid
		} else if w < maxWidth {
			low = mid + 1
		} else {
			return str[:mid]
		}
	}

	// Final adjustment to ensure valid UTF-8
	for low > 0 && !utf8.ValidString(str[:low]) {
		low--
	}

	return str[:low]
}

func clamp(val, minimum, maximum float64) float64 {
	if val < minimum {
		return minimum
	}

	if val > maximum {
		return maximum
	}

	return val
}
