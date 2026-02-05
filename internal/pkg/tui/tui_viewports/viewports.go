package tui_viewports

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kirill-scherba/omap"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
)

const (
	commandOutputMaxHeight, footerHeight = 8, 3
	scrollThumb, scrollTrack             = "█", "│"
)

type Viewports struct {
	viewports       *omap.Omap[config_attributes.Xpath, *Viewport]
	dimensions      *Dimensions
	colors          *config.ColorScheme
	debug           *strings.Builder
	fullscreenXpath config_attributes.Xpath // Tracks which viewport is in fullscreen mode
}

type Viewport struct {
	viewport      viewport.Model
	active        bool
	minHeight     int
	scrollbarZone config_attributes.Xpath
	content       string
}

type Dimensions struct{ Width, Height int }

type ViewportConfig struct {
	xpath          config_attributes.Xpath
	content        string
	availableWidth int
	viewportHeight int
	maxHeight      int
	wrapContent    bool
	useBorder      bool
	isFullscreen   bool
}

func NewViewports(d *Dimensions, c *config.ColorScheme, dbg *strings.Builder) *Viewports {
	v, _ := omap.New[config_attributes.Xpath, *Viewport]()
	return &Viewports{viewports: v, dimensions: d, colors: c, debug: dbg, fullscreenXpath: config_attributes.Xpath{}}
}

// IsFullscreen returns true if any viewport is currently in fullscreen mode
func (v *Viewports) IsFullscreen() bool {
	return v.fullscreenXpath.Depth() > 0
}

// GetFullscreenXpath returns the xpath of the viewport currently in fullscreen mode
func (v *Viewports) GetFullscreenXpath() config_attributes.Xpath {
	return v.fullscreenXpath
}

// SetFullscreen sets the fullscreen mode for a viewport
// Pass an empty xpath to exit fullscreen
func (v *Viewports) SetFullscreen(xpath config_attributes.Xpath) {
	v.fullscreenXpath = xpath
}

// ExitFullscreen exits fullscreen mode
func (v *Viewports) ExitFullscreen() {
	v.fullscreenXpath = config_attributes.Xpath{}
}

// ContentWidth returns the width available for content inside the main viewport,
// accounting for the scrollbar (2 characters: space + scrollbar)
func (v *Viewports) ContentWidth() int {
	return v.dimensions.Width - 2
}

// truncateLines truncates each line of content to maxWidth characters
func truncateLines(content string, maxWidth int) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth > maxWidth {
			// Find the byte position to truncate at
			truncated := line
			for lipgloss.Width(truncated) > maxWidth {
				truncated = truncated[:len(truncated)-1]
			}
			lines[i] = truncated
		}
	}
	return strings.Join(lines, "\n")
}

func (v *Viewports) renderScrollbar(pct float64, total, visible int) (string, int) {
	if total <= visible {
		return "", 0
	}
	thumb := int(float64(visible) * float64(visible) / float64(total))
	if thumb < 1 {
		thumb = 1
	}
	maxPos := visible - thumb
	if pct < 0 {
		pct = 0
	} else if pct > 1 {
		pct = 1
	}
	pos := int(float64(maxPos) * pct)
	lines := make([]string, visible)
	for i := range lines {
		if i >= pos && i < pos+thumb {
			lines[i] = scrollThumb
		} else {
			lines[i] = scrollTrack
		}
	}
	return lipgloss.NewStyle().Foreground(v.colors.TableBorder.GetForeground()).
		Render(strings.Join(lines, "\n")), 1
}

func (v *Viewports) combineWithScrollbar(view, bar string, barZone config_attributes.Xpath) string {
	if bar == "" {
		return view
	}
	vLines, bLines := strings.Split(view, "\n"), strings.Split(bar, "\n")
	cb := make([]string, len(vLines))
	for i, l := range vLines {
		if i < len(bLines) {
			cb[i] = l + " " + zone.Mark(barZone.NewXpathWithAppend(fmt.Sprintf("%d", i)).String(), bLines[i])
		} else {
			cb[i] = l
		}
	}
	return strings.Join(cb, "\n")
}

func (v *Viewports) getOrCreateViewport(cfg ViewportConfig) string {
	vpr, ok := v.viewports.Get(cfg.xpath)
	if !ok {
		nv := viewport.New(cfg.availableWidth, cfg.viewportHeight)
		nv.GotoBottom()
		vpr = &Viewport{
			viewport:      nv,
			minHeight:     cfg.viewportHeight,
			scrollbarZone: cfg.xpath.NewXpathWithAppend("scrollbar"),
			content:       cfg.content,
			// Main viewport is active by default
			active: cfg.xpath.String() == "main",
		}
		v.viewports.Set(cfg.xpath, vpr)
	} else {
		// Update content for existing viewport
		vpr.content = cfg.content
	}

	width := cfg.availableWidth

	allowedHeight := cfg.viewportHeight
	if allowedHeight <= 0 {
		allowedHeight = commandOutputMaxHeight
	}

	// Check if scrollbar needed and render content
	// First check at reduced width (accounting for potential scrollbar)
	procAtReducedWidth := lipgloss.NewStyle().Width(width - 2).Render(cfg.content)
	contentHeight := lipgloss.Height(procAtReducedWidth)
	needsBar := contentHeight > allowedHeight

	var proc string
	// Apply width constraint to prevent overflow
	if cfg.wrapContent {
		proc = lipgloss.NewStyle().Width(width).Render(cfg.content)
	} else {
		// Truncate long lines instead of wrapping
		proc = truncateLines(cfg.content, width)
	}
	contentHeight = lipgloss.Height(proc)

	if needsBar {
		// Reduce width for scrollbar and re-process content
		width -= 2
		if cfg.wrapContent {
			proc = lipgloss.NewStyle().Width(width).Render(cfg.content)
		} else {
			proc = truncateLines(cfg.content, width)
		}
		contentHeight = lipgloss.Height(proc)
	}

	height := contentHeight
	if !cfg.isFullscreen && cfg.maxHeight > 0 && contentHeight > cfg.maxHeight {
		height = cfg.maxHeight
	} else if cfg.isFullscreen {
		height = cfg.viewportHeight
	}

	vpr.viewport.Width = width
	vpr.viewport.Height = height

	pct := vpr.viewport.ScrollPercent()
	yOffset := vpr.viewport.YOffset
	vpr.viewport.SetContent(proc)
	// Restore scroll position (stay at bottom if we were there, to show new content)
	if pct == 1 {
		vpr.viewport.GotoBottom()
	} else {
		vpr.viewport.YOffset = yOffset
	}

	scrollbar, _ := v.renderScrollbar(vpr.viewport.ScrollPercent(), contentHeight, height)
	combined := v.combineWithScrollbar(vpr.viewport.View(), scrollbar, vpr.scrollbarZone)

	style := lipgloss.NewStyle()
	borderColor := v.colors.TableBorder.GetForeground()
	if cfg.useBorder {
		if vpr.active {
			borderColor = v.colors.TableBorder.GetBackground()
		}
		style = style.Border(lipgloss.RoundedBorder()).BorderForeground(borderColor)
	}

	return zone.Mark(cfg.xpath.String(), style.Render(combined))
}

func (v *Viewports) createViewport(xpath config_attributes.Xpath, content string, indent int,
	h, maxHeight int, wrap, border, full bool) string {

	width := v.dimensions.Width - indent
	// Inner viewports need width-2 to account for main viewport's scrollbar
	// (1 for scrollbar + 1 for space before it)
	if xpath.String() != "main" {
		width -= 2
	}

	// Ensure minimum dimensions to prevent panics
	if width < 1 {
		width = 1
	}
	if h < 1 {
		h = 1
	}

	return v.getOrCreateViewport(ViewportConfig{
		xpath:          xpath,
		content:        content,
		availableWidth: width,
		viewportHeight: h,
		maxHeight:      maxHeight,
		wrapContent:    wrap,
		useBorder:      border,
		isFullscreen:   full,
	})
}

func (v *Viewports) GetOrCreateViewport(xpath config_attributes.Xpath, content string, indent int) string {
	return v.createViewport(xpath, content, indent, 0, commandOutputMaxHeight, true, true, false)
}

func (v *Viewports) GetOrCreateLabelViewport(xpath config_attributes.Xpath, content string, indent int) string {
	return v.createViewport(xpath, content, indent, 0, 0, true, false, false)
}

func (v *Viewports) GetOrCreateMainViewport(content string) string {
	h := v.dimensions.Height - footerHeight
	return v.createViewport(config_attributes.NewXpath("main"), content, 0, h, h, false, false, true)
}

// RenderFullscreenViewport renders the fullscreen viewport
// This takes the existing viewport and renders it in fullscreen mode
func (v *Viewports) RenderFullscreenViewport(xpath config_attributes.Xpath, content string) string {
	// Account for footer (3 lines) and border (2 lines: top + bottom)
	h := v.dimensions.Height - footerHeight - 2
	if h < 1 {
		h = 1
	}
	// Account for main viewport's scrollbar (2) and border (2: left + right)
	width := v.dimensions.Width - 4
	if width < 1 {
		width = 1
	}

	// Check if viewport exists
	if vp, ok := v.viewports.Get(xpath); ok {
		// Store scroll position before modifying
		yOffset := vp.viewport.YOffset

		// Update the viewport dimensions for fullscreen
		vp.viewport.Width = width
		vp.viewport.Height = h
		vp.content = content

		// Process content with proper wrapping for fullscreen
		proc := lipgloss.NewStyle().Width(width).Render(content)
		vp.viewport.SetContent(proc)

		// Restore scroll position (but ensure it's valid)
		maxOffset := max(0, lipgloss.Height(proc)-h)
		if yOffset > maxOffset {
			yOffset = maxOffset
		}
		vp.viewport.YOffset = yOffset

		// Render scrollbar
		contentHeight := lipgloss.Height(proc)
		scrollbar, _ := v.renderScrollbar(vp.viewport.ScrollPercent(), contentHeight, h)
		combined := v.combineWithScrollbar(vp.viewport.View(), scrollbar, vp.scrollbarZone)

		// Apply border style
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(v.colors.TableBorder.GetBackground())

		return zone.Mark(xpath.String(), style.Render(combined))
	}

	// If viewport doesn't exist, create it normally
	cfg := ViewportConfig{
		xpath:          xpath,
		content:        content,
		availableWidth: width,
		viewportHeight: h,
		maxHeight:      h,
		wrapContent:    true,
		useBorder:      true,
		isFullscreen:   true,
	}

	return v.getOrCreateViewport(cfg)
}

func (v *Viewports) RemoveIfExistsViewport(xpath config_attributes.Xpath) { v.viewports.Del(xpath) }

func (v *Viewports) mostSpecific(viewports []config_attributes.Xpath) config_attributes.Xpath {
	if len(viewports) == 0 {
		return config_attributes.Xpath{}
	}
	sort.Slice(viewports, func(i, j int) bool { return viewports[i].Depth() > viewports[j].Depth() })
	return viewports[0]
}

func (v *Viewports) underMouse(msg tea.MouseMsg) []config_attributes.Xpath {
	var r []config_attributes.Xpath
	for xpath := range v.viewports.Records() {
		if zone.Get(xpath.String()).InBounds(msg) {
			r = append(r, xpath)
		}
	}
	return r
}

func (v *Viewports) hasActiveInner(mainXpath config_attributes.Xpath) bool {
	for xpath, vp := range v.viewports.Records() {
		if xpath != mainXpath && vp.active {
			return true
		}
	}
	return false
}

func (v *Viewports) Update(msg tea.Msg) tea.Cmd {
	cmds, mainXpath := make([]tea.Cmd, 0), config_attributes.NewXpath("main")

	if m, ok := msg.(tea.MouseMsg); ok {
		if m.Action == tea.MouseActionRelease {
			clicked := v.mostSpecific(v.underMouse(m))
			hasClick := clicked.Depth() > 0
			for xpath, vp := range v.viewports.Records() {
				vp.active = hasClick && xpath == clicked
			}
			// If clicked outside all viewports, reactivate main
			if !hasClick {
				if mainVpr, ok := v.viewports.Get(mainXpath); ok {
					mainVpr.active = true
				}
			}
		}
		// Handle mouse wheel scrolling - only scroll the active viewport
		if m.Button == tea.MouseButtonWheelUp || m.Button == tea.MouseButtonWheelDown {
			// Find the active viewport (inner first, fall back to main)
			var activeVp *Viewport
			for xpath, vp := range v.viewports.Records() {
				if vp.active && xpath != mainXpath {
					activeVp = vp
					break
				}
			}
			if activeVp == nil {
				if mainVpr, ok := v.viewports.Get(mainXpath); ok {
					activeVp = mainVpr
				}
			}

			if activeVp != nil {
				lines := 3
				if m.Button == tea.MouseButtonWheelUp {
					activeVp.viewport.ScrollUp(lines)
				} else {
					activeVp.viewport.ScrollDown(lines)
				}
			}
		}
	}

	hasActiveInner := v.hasActiveInner(mainXpath)

	// Handle keyboard scrolling for active viewport
	if m, ok := msg.(tea.KeyMsg); ok {
		// Find the active viewport (inner first, fall back to main)
		var activeVp *Viewport
		for xpath, vp := range v.viewports.Records() {
			if vp.active && xpath != mainXpath {
				activeVp = vp
				break
			}
		}
		if activeVp == nil {
			if mainVpr, ok := v.viewports.Get(mainXpath); ok {
				activeVp = mainVpr
			}
		}

		if activeVp != nil {
			switch m.String() {
			case "up", "k":
				activeVp.viewport.LineUp(1)
			case "down", "j":
				activeVp.viewport.LineDown(1)
			case "pgup":
				activeVp.viewport.HalfViewUp()
			case "pgdown", " ":
				activeVp.viewport.HalfViewDown()
			case "home", "g":
				activeVp.viewport.GotoTop()
			case "end", "G":
				activeVp.viewport.GotoBottom()
			}
		}
		// Don't pass KeyMsg to viewports - we handle scrolling manually
		return tea.Batch(cmds...)
	}

	for _, vp := range v.viewports.Records() {
		if !vp.active {
			continue
		}

		if updated, cmd := vp.viewport.Update(msg); cmd != nil {
			vp.viewport = updated
			cmds = append(cmds, cmd)
		}
	}

	if mainVpr, ok := v.viewports.Get(mainXpath); ok && !hasActiveInner {
		if updated, cmd := mainVpr.viewport.Update(msg); cmd != nil {
			mainVpr.viewport = updated
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

func (v *Viewports) Debug() string {
	s := fmt.Sprintf("\nViewports: %d (%dx%d)\n", v.viewports.Len(), v.dimensions.Width, v.dimensions.Height)
	for xpath, vp := range v.viewports.Records() {
		contentH := lipgloss.Height(vp.content)
		s += fmt.Sprintf("  '%s': %dx%d h:%d c:%d", xpath, vp.viewport.Width, vp.viewport.Height, vp.viewport.Height, contentH)
		if vp.active {
			s += " [A]"
		}
		if vp.viewport.ScrollPercent() == 1 {
			s += " @btm"
		}
		s += "\n"
	}
	return s
}

// GetActiveViewportContent returns the content of the currently active viewport
// Returns empty string if no viewport is active
func (v *Viewports) GetActiveViewportContent() string {
	mainXpath := config_attributes.NewXpath("main")

	// Find the active viewport (inner first, fall back to main)
	var activeVp *Viewport
	for xpath, vp := range v.viewports.Records() {
		if vp.active && xpath != mainXpath {
			activeVp = vp
			break
		}
	}
	if activeVp == nil {
		if mainVpr, ok := v.viewports.Get(mainXpath); ok {
			activeVp = mainVpr
		}
	}

	if activeVp != nil {
		return activeVp.content
	}
	return ""
}

// GetActiveInnerViewportContent returns the content of the active non-main viewport
// Returns "", false if main viewport is active or no viewport is active
// Returns content, true if an inner viewport is active
func (v *Viewports) GetActiveInnerViewportContent() (string, bool) {
	mainXpath := config_attributes.NewXpath("main")

	// Find active inner viewport only (not main)
	for xpath, vp := range v.viewports.Records() {
		if vp.active && xpath != mainXpath {
			// Get the actual rendered content from the viewport
			return vp.viewport.View(), true
		}
	}

	return "", false
}

// GetActiveInnerViewportXpath returns the xpath of the active non-main viewport
// Returns empty xpath if main viewport is active or no viewport is active
func (v *Viewports) GetActiveInnerViewportXpath() config_attributes.Xpath {
	mainXpath := config_attributes.NewXpath("main")

	for xpath, vp := range v.viewports.Records() {
		if vp.active && xpath != mainXpath {
			return xpath
		}
	}

	return config_attributes.Xpath{}
}

// GetViewportContent returns the raw content for a given xpath
// This is used to get the original content for fullscreen mode
func (v *Viewports) GetViewportContent(xpath config_attributes.Xpath) string {
	if vp, ok := v.viewports.Get(xpath); ok {
		return vp.content
	}
	return ""
}

// GetActiveViewportScrollPercent returns the scroll percent of the currently active viewport
// Returns 0.0 if no viewport is active
func (v *Viewports) GetActiveViewportScrollPercent() float64 {
	mainXpath := config_attributes.NewXpath("main")

	// First check for fullscreen viewport
	if v.IsFullscreen() {
		if vp, ok := v.viewports.Get(v.fullscreenXpath); ok {
			return vp.viewport.ScrollPercent()
		}
	}

	// Find the active viewport (inner first, fall back to main)
	var activeVp *Viewport
	for xpath, vp := range v.viewports.Records() {
		if vp.active && xpath != mainXpath {
			activeVp = vp
			break
		}
	}
	if activeVp == nil {
		if mainVpr, ok := v.viewports.Get(mainXpath); ok {
			activeVp = mainVpr
		}
	}

	if activeVp != nil {
		return activeVp.viewport.ScrollPercent()
	}
	return 0.0
}
