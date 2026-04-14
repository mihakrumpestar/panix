package viewports

import (
	"fmt"
	"hash/fnv"
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
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
)

const (
	scrollThumb       = "█"
	scrollTrack       = "│"
	scrollbarWidth    = 2
	borderHeight      = 2
	borderWidth       = 2
	mouseScrollAmount = 3
)

// Dimensions represents terminal size.
type Dimensions struct {
	Width, Height int
}

// Viewports manages all viewport instances.
type Viewports struct {
	viewports       *omap.Omap[xpath.Xpath, *Viewport]
	dimensions      *Dimensions
	conf            *config.Config
	fullscreenXpath xpath.Xpath
	mainXpath       xpath.Xpath
}

// Viewport wraps a bubbletea viewport with additional state.
type Viewport struct {
	model         viewport.Model
	active        bool
	content       string
	scrollbarZone xpath.Xpath
	cache         viewportCache
}

// viewportCache stores rendered output to avoid redundant rendering.
type viewportCache struct {
	width       int
	height      int
	contentHash uint64
	scrollPct   float64
	active      bool
	render      string
}

// hashContent generates a fast hash for content comparison.
func hashContent(content string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(content))

	return h.Sum64()
}

// NewViewports creates a new viewport manager.
func NewViewports(dimensions *Dimensions, conf *config.Config) *Viewports {
	viewportsMap, _ := omap.New[xpath.Xpath, *Viewport]()

	return &Viewports{
		viewports:  viewportsMap,
		dimensions: dimensions,
		conf:       conf,
		mainXpath:  xpath.New("main"),
	}
}

// Fullscreen management

func (v *Viewports) IsFullscreen() bool              { return v.fullscreenXpath.Depth() > 0 }
func (v *Viewports) GetFullscreenXpath() xpath.Xpath { return v.fullscreenXpath }
func (v *Viewports) SetFullscreen(xpath xpath.Xpath) { v.fullscreenXpath = xpath }
func (v *Viewports) ExitFullscreen()                 { v.fullscreenXpath = xpath.Xpath{} }

// ContentWidth returns available width accounting for scrollbar.
func (v *Viewports) ContentWidth() int { return v.dimensions.Width - scrollbarWidth }

// Viewport factory methods

func (v *Viewports) GetOrCreateViewport(xpath xpath.Xpath, content string, indent int) string {
	return v.createViewport(xpath, content, indent, viewportOptions{
		maxHeight:   v.conf.Flags.Tui.CommandOutputMaxHeight,
		wrapContent: true,
		useBorder:   true,
	})
}

func (v *Viewports) GetOrCreateLabelViewport(xpath xpath.Xpath, content string, indent int) string {
	return v.createViewport(xpath, content, indent, viewportOptions{wrapContent: true, noPadding: true})
}

func (v *Viewports) GetOrCreateMainViewport(content string, footerHeaderHeight int) string {
	h := v.dimensions.Height - footerHeaderHeight

	return v.createViewport(v.mainXpath, content, 0, viewportOptions{
		height:    h,
		maxHeight: h,
		full:      true,
	}) + "\n"
}

func (v *Viewports) RenderFullscreenViewport(xpath xpath.Xpath, content string, footerHeaderHeight int) string {
	height := max(1, v.dimensions.Height-footerHeaderHeight-borderHeight)
	width := max(1, v.dimensions.Width-scrollbarWidth-borderWidth)

	viewport, exists := v.viewports.Get(xpath)
	if !exists {
		return v.createViewport(xpath, content, 0, viewportOptions{
			availableWidth: width, height: height, maxHeight: height,
			wrapContent: true, useBorder: true, full: true,
		})
	}

	if v.isCacheValid(viewport, width, height, content) {
		return viewport.cache.render
	}

	contentHash := hashContent(content)

	yOffset := viewport.model.YOffset()
	viewport.model.SetWidth(width)
	viewport.model.SetHeight(height)
	viewport.content = content
	proc := lipgloss.NewStyle().Width(width).Render(content)
	viewport.model.SetContent(proc)
	viewport.model.SetYOffset(min(yOffset, max(0, lipgloss.Height(proc)-height)))

	rendered := v.renderViewport(viewport, lipgloss.Height(proc), height, true, false)
	rendered = zone.Mark(xpath.String(), rendered)
	viewport.cache = viewportCache{
		width:       width,
		height:      height,
		contentHash: contentHash,
		scrollPct:   viewport.model.ScrollPercent(),
		active:      viewport.active,
		render:      rendered,
	}

	return rendered + "\n"
}

func (v *Viewports) RemoveIfExistsViewport(xpath xpath.Xpath) { v.viewports.Del(xpath) }

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

func (v *Viewports) GetActiveInnerViewportXpath() xpath.Xpath {
	for xpath, vp := range v.viewports.Records() {
		if vp.active && xpath != v.mainXpath {
			return xpath
		}
	}

	return xpath.Xpath{}
}

func (v *Viewports) GetViewportContent(xpath xpath.Xpath) string {
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

// Update handles messages for all viewports.
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
		v.handleResize(msgVal)

		return tea.Batch(cmds...)
	}

	cmds = v.updateActiveViewports(msg, cmds)
	cmds = v.updateMainViewport(msg, cmds)

	return tea.Batch(cmds...)
}

// Debug returns debug info about all viewports.
func (v *Viewports) Debug() string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "\nViewports: %d (%dx%d)\n", v.viewports.Len(), v.dimensions.Width, v.dimensions.Height)

	for xpath, viewport := range v.viewports.Records() {
		fmt.Fprintf(&builder, "  '%s': %dx%d c:%d", xpath, viewport.model.Width(), viewport.model.Height(), lipgloss.Height(viewport.content))

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

func (v *Viewports) HasActiveInner() bool {
	for xpath, vp := range v.viewports.Records() {
		if xpath != v.mainXpath && vp.active {
			return true
		}
	}

	return false
}

// DeselectAll activates only the main viewport.
func (v *Viewports) DeselectAll() {
	for xpath, vp := range v.viewports.Records() {
		vp.active = xpath == v.mainXpath
	}
}

// isCacheValid checks if the cached render is still valid.
// Dimensions and state are checked first (O(1)) before content hash (O(n)) due to && short-circuit.
func (v *Viewports) isCacheValid(viewport *Viewport, width, height int, content string) bool {
	return viewport.cache.width == width &&
		viewport.cache.height == height &&
		viewport.cache.scrollPct == viewport.model.ScrollPercent() &&
		viewport.cache.active == viewport.active &&
		viewport.cache.contentHash == hashContent(content) &&
		viewport.cache.render != ""
}

// handleResize updates dimensions when terminal size changes.
func (v *Viewports) handleResize(msg tea.WindowSizeMsg) {
	v.dimensions.Width = msg.Width
	v.dimensions.Height = msg.Height
}

// updateActiveViewports updates all active viewports with the given message.
func (v *Viewports) updateActiveViewports(msg tea.Msg, cmds []tea.Cmd) []tea.Cmd {
	for _, viewport := range v.viewports.Records() {
		if viewport.active {
			if updated, cmd := viewport.model.Update(msg); cmd != nil {
				viewport.model = updated

				cmds = append(cmds, cmd)
			}
		}
	}

	return cmds
}

// updateMainViewport updates the main viewport if no inner viewport is active.
func (v *Viewports) updateMainViewport(msg tea.Msg, cmds []tea.Cmd) []tea.Cmd {
	if v.HasActiveInner() {
		return cmds
	}

	mainVpr, ok := v.viewports.Get(v.mainXpath)
	if !ok {
		return cmds
	}

	if updated, cmd := mainVpr.model.Update(msg); cmd != nil {
		mainVpr.model = updated

		cmds = append(cmds, cmd)
	}

	return cmds
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

func (v *Viewports) createViewport(xpath xpath.Xpath, content string, indent int, opts viewportOptions) string {
	width := v.calculateViewportWidth(opts, indent)
	height := max(1, opts.height)

	viewportInstance := v.getOrCreateViewportInstance(xpath, content, width, height)
	if viewportInstance == nil {
		return ""
	}

	if v.isCacheValid(viewportInstance, width, height, content) {
		return viewportInstance.cache.render
	}

	contentHash := hashContent(content)

	proc, contentHeight, finalWidth := v.processViewportContent(content, width, opts)

	finalHeight := v.calculateFinalHeight(contentHeight, opts, height)
	v.configureViewportModel(xpath, viewportInstance, proc, finalWidth, finalHeight)

	rendered := v.renderViewport(viewportInstance, contentHeight, finalHeight, opts.useBorder, opts.noPadding)
	rendered = zone.Mark(xpath.String(), rendered)

	viewportInstance.cache = viewportCache{
		width:       width,
		height:      height,
		contentHash: contentHash,
		scrollPct:   viewportInstance.model.ScrollPercent(),
		active:      viewportInstance.active,
		render:      rendered,
	}

	return rendered
}

// calculateViewportWidth determines the available width for a viewport.
func (v *Viewports) calculateViewportWidth(opts viewportOptions, indent int) int {
	if opts.availableWidth != 0 {
		return opts.availableWidth
	}

	return v.dimensions.Width - indent - scrollbarWidth
}

// getOrCreateViewportInstance retrieves an existing viewport or creates a new one.
func (v *Viewports) getOrCreateViewportInstance(xpath xpath.Xpath, content string, width, height int) *Viewport {
	viewportInstance, exists := v.viewports.Get(xpath)
	if !exists {
		viewportInstance = &Viewport{
			model:         viewport.New(viewport.WithWidth(width), viewport.WithHeight(height)),
			scrollbarZone: xpath.NewXpathWithAppend("scrollbar"),
			content:       content,
			active:        xpath == v.mainXpath,
		}
		viewportInstance.model.GotoBottom()

		err := v.viewports.Set(xpath, viewportInstance)
		if err != nil {
			return nil
		}
	} else {
		viewportInstance.content = content
	}

	return viewportInstance
}

// processViewportContent processes content and returns it with its calculated height and final width.
func (v *Viewports) processViewportContent(content string, width int, opts viewportOptions) (string, int, int) {
	proc := v.processContent(content, width, opts.wrapContent, opts.noPadding)
	contentHeight := lipgloss.Height(proc)

	finalWidth := width
	if contentHeight > opts.maxHeight && opts.maxHeight > 0 && !opts.full {
		finalWidth = max(1, width-scrollbarWidth)
		proc = v.processContent(content, finalWidth, opts.wrapContent, opts.noPadding)
		contentHeight = lipgloss.Height(proc)
	}

	return proc, contentHeight, finalWidth
}

// calculateFinalHeight determines the final height for a viewport.
func (v *Viewports) calculateFinalHeight(contentHeight int, opts viewportOptions, height int) int {
	if !opts.full && opts.maxHeight > 0 && contentHeight > opts.maxHeight {
		return opts.maxHeight
	}

	if opts.full {
		return height
	}

	return contentHeight
}

// configureViewportModel configures the viewport model with processed content.
func (v *Viewports) configureViewportModel(xpath xpath.Xpath, viewportInstance *Viewport, proc string, width, finalHeight int) {
	viewportInstance.model.SetWidth(width)
	viewportInstance.model.SetHeight(finalHeight)

	yOffset, pct := viewportInstance.model.YOffset(), viewportInstance.model.ScrollPercent()
	viewportInstance.model.SetContent(proc)

	if pct == 1 && xpath != v.mainXpath {
		viewportInstance.model.GotoBottom()
	} else {
		maxOffset := max(0, lipgloss.Height(proc)-finalHeight)
		viewportInstance.model.SetYOffset(min(yOffset, maxOffset))
	}
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

func (v *Viewports) renderViewport(viewport *Viewport, wrappedContentHeight int, height int, useBorder bool, noPadding bool) string {
	scrollbar, _ := v.renderScrollbar(viewport.model.ScrollPercent(), wrappedContentHeight, height)
	combined := v.combineWithScrollbar(viewport.model.View(), scrollbar, viewport.scrollbarZone)

	if noPadding {
		if viewport.active {
			combined = v.conf.ColorScheme.Table.SelectionHighlightBackground.Render(combined)
		}

		return combined
	}

	if !useBorder {
		return combined
	}

	borderColor := v.conf.ColorScheme.Table.Border.GetForeground()
	if viewport.active {
		borderColor = v.conf.ColorScheme.Table.Border.GetBackground()
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

	for i := range visible {
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
		Foreground(v.conf.ColorScheme.Table.Border.GetForeground()).
		Render(builder.String()), 1
}

func (v *Viewports) combineWithScrollbar(view, bar string, barZone xpath.Xpath) string {
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
				vp.model.ScrollUp(mouseScrollAmount)
			} else {
				vp.model.ScrollDown(mouseScrollAmount)
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
		var mainVp *Viewport

		mainVp, ok = v.viewports.Get(v.mainXpath)
		if ok {
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

func (v *Viewports) underMouse(m tea.MouseMsg) []xpath.Xpath {
	var result []xpath.Xpath

	for xpath := range v.viewports.Records() {
		if zone.Get(xpath.String()).InBounds(m) {
			result = append(result, xpath)
		}
	}

	return result
}

func (v *Viewports) mostSpecific(xpaths []xpath.Xpath) xpath.Xpath {
	if len(xpaths) == 0 {
		return xpath.Xpath{}
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

// truncateToRuneWidth truncates a string to fit within maxWidth runes
// using lipgloss.Width for accurate display width measurement.
func truncateToRuneWidth(str string, maxWidth int) string {
	width := lipgloss.Width(str)
	if width <= maxWidth {
		return str
	}

	// Binary search for the correct truncation point
	low, high := 0, len(str)
	for low < high {
		mid := (low + high) / 2 //nolint:mnd

		mid = adjustToUTF8Boundary(str, low, mid)
		if mid <= low {
			break
		}

		w := lipgloss.Width(str[:mid])

		switch {
		case w > maxWidth:
			high = mid
		case w < maxWidth:
			low = mid + 1
		default:
			return str[:mid]
		}
	}

	// Final adjustment to ensure valid UTF-8
	low = ensureValidUTF8(str, low)

	return str[:low]
}

// adjustToUTF8Boundary adjusts the midpoint to a valid UTF-8 boundary.
func adjustToUTF8Boundary(str string, low, mid int) int {
	for mid > low && !utf8.ValidString(str[:mid]) {
		mid--
	}

	return mid
}

// ensureValidUTF8 ensures the position is at a valid UTF-8 boundary.
func ensureValidUTF8(str string, pos int) int {
	for pos > 0 && !utf8.ValidString(str[:pos]) {
		pos--
	}

	return pos
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
