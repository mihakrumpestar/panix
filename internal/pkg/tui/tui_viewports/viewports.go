package tui_viewports

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kirill-scherba/omap"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
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
	viewports              *omap.Omap[config_attributes.Xpath, *Viewport]
	dimensions             *Dimensions
	colors                 *config.ColorScheme
	debug                  *strings.Builder
	fullscreenXpath        config_attributes.Xpath
	commandOutputMaxHeight int
	mainXpath              config_attributes.Xpath
	footerHeight           int
}

// Viewport wraps a bubbletea viewport with additional state
type Viewport struct {
	model         viewport.Model
	active        bool
	content       string
	scrollbarZone config_attributes.Xpath
}

// NewViewports creates a new viewport manager
func NewViewports(d *Dimensions, c *config.ColorScheme, dbg *strings.Builder, maxHeight int) *Viewports {
	v, _ := omap.New[config_attributes.Xpath, *Viewport]()
	// Ensure minimum height of 1
	if maxHeight < 1 {
		maxHeight = 1
	}
	return &Viewports{
		viewports:              v,
		dimensions:             d,
		colors:                 c,
		debug:                  dbg,
		commandOutputMaxHeight: maxHeight,
		mainXpath:              config_attributes.NewXpath("main"),
	}
}

// Fullscreen management
func (v *Viewports) IsFullscreen() bool                          { return v.fullscreenXpath.Depth() > 0 }
func (v *Viewports) GetFullscreenXpath() config_attributes.Xpath { return v.fullscreenXpath }
func (v *Viewports) SetFullscreen(xpath config_attributes.Xpath) { v.fullscreenXpath = xpath }
func (v *Viewports) ExitFullscreen()                             { v.fullscreenXpath = config_attributes.Xpath{} }

// ContentWidth returns available width accounting for scrollbar
func (v *Viewports) ContentWidth() int { return v.dimensions.Width - scrollbarWidth }

// Viewport factory methods

func (v *Viewports) GetOrCreateViewport(xpath config_attributes.Xpath, content string, indent int) string {
	return v.createViewport(xpath, content, indent, viewportOptions{
		maxHeight:   v.commandOutputMaxHeight,
		wrapContent: true,
		useBorder:   true,
	})
}

func (v *Viewports) GetOrCreateLabelViewport(xpath config_attributes.Xpath, content string, indent int) string {
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

func (v *Viewports) RenderFullscreenViewport(xpath config_attributes.Xpath, content string, footerHeight int) string {
	v.footerHeight = footerHeight
	h := max(1, v.dimensions.Height-footerHeight-borderHeight)
	w := max(1, v.dimensions.Width-scrollbarWidth-borderWidth)

	vp, exists := v.viewports.Get(xpath)
	if !exists {
		return v.createViewport(xpath, content, 0, viewportOptions{
			availableWidth: w, height: h, maxHeight: h,
			wrapContent: true, useBorder: true, full: true,
		})
	}

	yOffset := vp.model.YOffset
	vp.model.Width, vp.model.Height = w, h
	vp.content = content
	proc := lipgloss.NewStyle().Width(w).Render(content)
	vp.model.SetContent(proc)
	vp.model.YOffset = min(yOffset, max(0, lipgloss.Height(proc)-h))

	return v.renderViewport(vp, h, true, false)
}

func (v *Viewports) RemoveIfExistsViewport(xpath config_attributes.Xpath) { v.viewports.Del(xpath) }

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

func (v *Viewports) GetActiveInnerViewportXpath() config_attributes.Xpath {
	for xpath, vp := range v.viewports.Records() {
		if vp.active && xpath != v.mainXpath {
			return xpath
		}
	}
	return config_attributes.Xpath{}
}

func (v *Viewports) GetViewportContent(xpath config_attributes.Xpath) string {
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

	switch m := msg.(type) {
	case tea.MouseMsg:
		v.handleMouse(m)
	case tea.KeyMsg:
		v.handleKey(m)
		return tea.Batch(cmds...)
	case tea.WindowSizeMsg:
		// Update dimensions and resize all viewports
		v.dimensions.Width = max(m.Width, minTerminalWidth)
		v.dimensions.Height = m.Height
		v.resizeAllViewports()
		return tea.Batch(cmds...)
	}

	hasActiveInner := v.hasActiveInner()
	for _, vp := range v.viewports.Records() {
		if vp.active {
			if updated, cmd := vp.model.Update(msg); cmd != nil {
				vp.model = updated
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
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\nViewports: %d (%dx%d)\n", v.viewports.Len(), v.dimensions.Width, v.dimensions.Height))
	for xpath, vp := range v.viewports.Records() {
		sb.WriteString(fmt.Sprintf("  '%s': %dx%d c:%d", xpath, vp.model.Width, vp.model.Height, lipgloss.Height(vp.content)))
		if vp.active {
			sb.WriteString(" [A]")
		}
		if vp.model.ScrollPercent() == 1 {
			sb.WriteString(" @btm")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// resizeAllViewports updates all viewport dimensions when terminal resizes
func (v *Viewports) resizeAllViewports() {
	for xpath, vp := range v.viewports.Records() {
		w := max(1, v.dimensions.Width-scrollbarWidth)
		h := max(1, v.dimensions.Height-v.footerHeight)

		if xpath == v.mainXpath {
			// Main viewport gets full height minus footer
			vp.model.Width, vp.model.Height = w, h
		} else {
			// Inner viewports: update width, keep existing height or use max height
			vp.model.Width = w
			if v.commandOutputMaxHeight > 0 && vp.model.Height > v.commandOutputMaxHeight {
				vp.model.Height = v.commandOutputMaxHeight
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

func (v *Viewports) createViewport(xpath config_attributes.Xpath, content string, indent int, opts viewportOptions) string {
	w := opts.availableWidth
	if w == 0 {
		w = max(10, v.dimensions.Width-indent-scrollbarWidth)
	}
	h := max(1, opts.height)

	vp, exists := v.viewports.Get(xpath)
	if !exists {
		vp = &Viewport{
			model:         viewport.New(w, h),
			scrollbarZone: xpath.NewXpathWithAppend("scrollbar"),
			content:       content,
			active:        xpath == v.mainXpath,
		}
		vp.model.GotoBottom()
		v.viewports.Set(xpath, vp)
	} else {
		vp.content = content
	}

	proc := v.processContent(content, w, opts.wrapContent, opts.noPadding)
	contentHeight := lipgloss.Height(proc)

	if contentHeight > opts.maxHeight && opts.maxHeight > 0 && !opts.full {
		w = max(1, w-scrollbarWidth)
		proc = v.processContent(content, w, opts.wrapContent, opts.noPadding)
		contentHeight = lipgloss.Height(proc)
	}

	finalHeight := contentHeight
	if !opts.full && opts.maxHeight > 0 && contentHeight > opts.maxHeight {
		finalHeight = opts.maxHeight
	} else if opts.full {
		finalHeight = h
	}

	vp.model.Width, vp.model.Height = w, finalHeight

	yOffset, pct := vp.model.YOffset, vp.model.ScrollPercent()
	vp.model.SetContent(proc)

	if pct == 1 {
		vp.model.GotoBottom()
	} else {
		maxOffset := max(0, lipgloss.Height(proc)-finalHeight)
		vp.model.YOffset = min(yOffset, maxOffset)
	}

	return zone.Mark(xpath.String(), v.renderViewport(vp, finalHeight, opts.useBorder, opts.noPadding))
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

func (v *Viewports) renderViewport(vp *Viewport, height int, useBorder bool, noPadding bool) string {
	contentHeight := lipgloss.Height(vp.content)
	scrollbar, _ := v.renderScrollbar(vp.model.ScrollPercent(), contentHeight, height)
	combined := v.combineWithScrollbar(vp.model.View(), scrollbar, vp.scrollbarZone)

	if noPadding {
		if vp.active {
			combined = v.colors.SelectionHighlightBackground.Render(combined)
		}
		return combined
	}

	if !useBorder {
		return combined
	}

	borderColor := v.colors.TableBorder.GetForeground()
	if vp.active {
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

func (v *Viewports) combineWithScrollbar(view, bar string, barZone config_attributes.Xpath) string {
	if bar == "" {
		return view
	}

	vLines := strings.Split(view, "\n")
	bLines := strings.Split(bar, "\n")
	result := make([]string, len(vLines))
	barLen := len(bLines)

	for i, line := range vLines {
		if i < barLen {
			result[i] = line + " " + zone.Mark(barZone.NewXpathWithAppend(fmt.Sprintf("%d", i)).String(), bLines[i])
		} else {
			result[i] = line
		}
	}

	return strings.Join(result, "\n")
}

func (v *Viewports) handleMouse(m tea.MouseMsg) {
	// Handle mouse wheel scrolling first (doesn't require release action)
	if m.Button == tea.MouseButtonWheelUp || m.Button == tea.MouseButtonWheelDown {
		if vp := v.getActiveViewport(); vp != nil {
			if m.Button == tea.MouseButtonWheelUp {
				vp.model.ScrollUp(3)
			} else {
				vp.model.ScrollDown(3)
			}
		}
		return
	}

	// Handle click-to-activate (only on release)
	if m.Action != tea.MouseActionRelease {
		return
	}

	clicked := v.mostSpecific(v.underMouse(m))
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

func (v *Viewports) handleKey(m tea.KeyMsg) {
	if vp := v.getActiveViewport(); vp != nil {
		switch m.String() {
		case "up", "k":
			vp.model.ScrollUp(1)
		case "down", "j":
			vp.model.ScrollDown(1)
		case "pgup":
			vp.model.HalfPageUp()
		case "pgdown", " ":
			vp.model.HalfPageDown()
		case "home", "g":
			vp.model.GotoTop()
		case "end", "G":
			vp.model.GotoBottom()
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

func (v *Viewports) underMouse(m tea.MouseMsg) []config_attributes.Xpath {
	var result []config_attributes.Xpath
	for xpath := range v.viewports.Records() {
		if zone.Get(xpath.String()).InBounds(m) {
			result = append(result, xpath)
		}
	}
	return result
}

func (v *Viewports) mostSpecific(xpaths []config_attributes.Xpath) config_attributes.Xpath {
	if len(xpaths) == 0 {
		return config_attributes.Xpath{}
	}
	sort.Slice(xpaths, func(i, j int) bool { return xpaths[i].Depth() > xpaths[j].Depth() })
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
func truncateToRuneWidth(s string, maxWidth int) string {
	width := lipgloss.Width(s)
	if width <= maxWidth {
		return s
	}

	// Binary search for the correct truncation point
	low, high := 0, len(s)
	for low < high {
		mid := (low + high) / 2
		// Find the previous valid UTF-8 boundary
		for mid > low && !utf8.ValidString(s[:mid]) {
			mid--
		}
		if mid <= low {
			break
		}

		w := lipgloss.Width(s[:mid])
		if w > maxWidth {
			high = mid
		} else if w < maxWidth {
			low = mid + 1
		} else {
			return s[:mid]
		}
	}

	// Final adjustment to ensure valid UTF-8
	for low > 0 && !utf8.ValidString(s[:low]) {
		low--
	}
	return s[:low]
}

func clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
