package tui_viewports

import (
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kirill-scherba/omap"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_raw_key_reader"
)

type Viewports struct {
	viewports  *omap.Omap[config_attributes.Xpath, *Viewport]
	dimensions *Dimensions
	colors     *config.ColorScheme
	debug      *strings.Builder
}

type Viewport struct {
	viewport      viewport.Model
	active        bool
	pty           *os.File
	minHeight     int
	scrollbarZone config_attributes.Xpath // Zone ID for scrollbar area
	content       string                  // Store the content for double-click copying
}

type Dimensions struct {
	Width  int
	Height int
}

func NewViewports(dimensions *Dimensions, colors *config.ColorScheme, debug *strings.Builder) *Viewports {
	viewports, err := omap.New[config_attributes.Xpath, *Viewport]()
	if err != nil {
		panic(err)
	}

	return &Viewports{
		viewports:  viewports,
		dimensions: dimensions,
		colors:     colors,
		debug:      debug,
	}
}

// renderScrollbar creates a visual scrollbar representation
func (v *Viewports) renderScrollbar(scrollPercent float64, totalLines, visibleLines int) (string, int) {
	if totalLines <= visibleLines {
		return "", 0 // No scrollbar needed if all content is visible
	}

	// Calculate thumb size proportional to visible content
	// Like in browsers: thumb_size = viewport_height * (viewport_height / content_height)
	thumbRatio := float64(visibleLines) / float64(totalLines)
	thumbSize := int(float64(visibleLines) * thumbRatio)

	if thumbSize < 1 { // Keep minimum
		thumbSize = 1
	}

	// Calculate the position of the scrollbar thumb
	// The thumb should move within the available space (visibleLines - thumbSize)
	maxThumbPosition := visibleLines - thumbSize
	thumbPosition := int(float64(maxThumbPosition) * scrollPercent)
	if thumbPosition < 0 {
		thumbPosition = 0
	}
	if thumbPosition > maxThumbPosition {
		thumbPosition = maxThumbPosition
	}

	// Build the scrollbar
	scrollbar := make([]string, visibleLines)
	for i := range scrollbar {
		if i >= thumbPosition && i < thumbPosition+thumbSize {
			scrollbar[i] = "█" // Thumb
		} else {
			scrollbar[i] = "│" // Track
		}
	}

	// Style the scrollbar
	scrollbarStyle := lipgloss.NewStyle().
		Foreground(v.colors.TableBorder.GetForeground())

	// Join all lines with newlines
	return scrollbarStyle.Render(strings.Join(scrollbar, "\n")), 2
}

// ViewportConfig holds configuration options for creating or updating a viewport
type ViewportConfig struct {
	xpath          config_attributes.Xpath
	content        string
	pty            *os.File
	availableWidth int
	viewportHeight int
	maxHeight      int
	wrapContent    bool
	useBorder      bool
	isFullScrean   bool
}

// calculateAvailableWidth determines the final width available for content,
// accounting for scrollbar space if needed.
func (v *Viewports) calculateAvailableWidth(vpr *Viewport, config ViewportConfig, totalLines int) int {
	// Calculate what width will be available after accounting for scrollbar
	scrollbarWidth := 0

	// For fullscreen viewport, the scrollbar is added to the right without taking content space
	// So the content width remains the same
	if config.isFullScrean {
		return config.availableWidth
	}

	// For normal viewports, we need to check if scrollbar will be present
	visibleLines := config.viewportHeight
	if visibleLines <= 0 {
		// Calculate based on content height if viewportHeight is 0
		if config.wrapContent {
			wrappedHeight := lipgloss.Height(lipgloss.NewStyle().Width(config.availableWidth).Render(config.content))
			visibleLines = wrappedHeight
		} else {
			visibleLines = lipgloss.Height(config.content)
		}
	}

	// Check if scrollbar is needed (content exceeds visible lines)
	if totalLines > visibleLines {
		scrollbarWidth = 2 // 1 space + 1 scrollbar column
	}

	return config.availableWidth - scrollbarWidth
}

// combineViewportWithScrollbar combines viewport content with scrollbar
func (v *Viewports) combineViewportWithScrollbar(viewportView, scrollbar string, scrollbarZone config_attributes.Xpath) string {
	if scrollbar == "" {
		return viewportView
	}

	// Split viewport view into lines and combine with scrollbar
	viewportLines := strings.Split(viewportView, "\n")
	scrollbarLines := strings.Split(scrollbar, "\n")

	combinedLines := make([]string, len(viewportLines))
	for i, line := range viewportLines {
		if i < len(scrollbarLines) {
			// Add spacing and zone the scrollbar
			scrollbarLine := zone.Mark(scrollbarZone.NewXpathWithAppend(fmt.Sprintf("%d", i)).String(), scrollbarLines[i])
			combinedLines[i] = line + " " + scrollbarLine
		} else {
			combinedLines[i] = line
		}
	}
	return strings.Join(combinedLines, "\n")
}

// getOrCreateViewportShared is the shared implementation for both GetOrCreateViewport and GetOrCreateMainViewport
func (v *Viewports) getOrCreateViewportShared(config ViewportConfig) string {
	vpr, ok := v.viewports.Get(config.xpath)
	if !ok {
		// Create new viewport with appropriate dimensions
		viewport := viewport.New(config.availableWidth, config.viewportHeight)
		viewport.GotoBottom()

		vpr = &Viewport{
			viewport:      viewport,
			pty:           config.pty,
			minHeight:     config.viewportHeight,
			scrollbarZone: config.xpath.NewXpathWithAppend("scrollbar"),
			content:       config.content,
		}

		_ = v.viewports.Set(config.xpath, vpr)
	}

	// Calculate total lines in the current content to determine if scrollbar is needed
	totalLines := lipgloss.Height(config.content)

	// Calculate the final width available for content, accounting for scrollbar
	availableWidthForContent := v.calculateAvailableWidth(vpr, config, totalLines)

	// Update viewport dimensions BEFORE processing content
	vpr.viewport.Width = availableWidthForContent

	// Process content based on configuration
	var processedContent string
	if config.wrapContent {
		processedContent = lipgloss.NewStyle().Width(availableWidthForContent).Render(config.content)
	} else {
		processedContent = config.content
	}

	// Recalculate total lines based on processed content
	totalLines = lipgloss.Height(processedContent)

	// Calculate height if needed
	var finalHeight int
	if config.isFullScrean {
		finalHeight = config.viewportHeight
	} else {
		// Calculate height based on the wrapped content
		if config.maxHeight > 0 {
			finalHeight = min(totalLines, config.maxHeight)
		} else {
			// No height limit specified, use full content height
			finalHeight = totalLines
		}
	}

	// Update viewport height
	vpr.viewport.Height = finalHeight

	// Preserve scroll position before updating content
	oldScrollPercent := vpr.viewport.ScrollPercent()

	// Set the processed content
	vpr.viewport.SetContent(processedContent)

	// Follow bottom if we are stuck at the bottom, and not if we have scrolled higher
	if oldScrollPercent == 1 {
		vpr.viewport.GotoBottom()
	}

	// Get the viewport view AFTER content is updated
	viewportView := vpr.viewport.View()

	// Get scroll percentage and line counts for scrollbar calculation
	scrollPercent := vpr.viewport.ScrollPercent()
	visibleLines := vpr.viewport.Height

	// Create scrollbar
	scrollbar, _ := v.renderScrollbar(scrollPercent, totalLines, visibleLines)

	// Combine viewport content with scrollbar
	combinedView := v.combineViewportWithScrollbar(viewportView, scrollbar, vpr.scrollbarZone)

	style := lipgloss.NewStyle()

	// Apply border if needed
	if config.useBorder {
		borderColor := v.colors.TableBorder.GetForeground()
		if vpr.active {
			// Create a lighter version of the border color for active viewports
			borderColor = v.colors.TableBorder.GetBackground()
		}
		style = style.Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor)
	} else {
		style = style.UnsetBorderStyle()
	}

	return zone.Mark(config.xpath.String(), style.Render(combinedView))
}

func (v *Viewports) GetOrCreateViewport(xpath config_attributes.Xpath, content string, pty *os.File, indentation int) string {
	availableWidth := v.dimensions.Width - indentation

	maxHeight := 8

	config := ViewportConfig{
		xpath:          xpath,
		content:        content,
		pty:            pty,
		availableWidth: availableWidth,
		viewportHeight: 0, // Will be calculated based on content
		maxHeight:      maxHeight,
		wrapContent:    true,
		useBorder:      true,
		isFullScrean:   false,
	}

	return v.getOrCreateViewportShared(config)
}

// GetOrCreateLabelViewport creates or updates a viewport specifically for labels
func (v *Viewports) GetOrCreateLabelViewport(xpath config_attributes.Xpath, content string, indentation int) string {
	availableWidth := v.dimensions.Width - indentation

	// For labels, we want to show all lines without a height limit
	// The height will be calculated based on the actual content
	config := ViewportConfig{
		xpath:          xpath,
		content:        content,
		pty:            nil,
		availableWidth: availableWidth,
		viewportHeight: 0, // Will be calculated based on content
		maxHeight:      0, // No height limit for labels
		wrapContent:    true,
		useBorder:      false, // No border for labels
		isFullScrean:   false,
	}

	return v.getOrCreateViewportShared(config)
}

// GetOrCreateMainViewport creates or updates the main viewport with full screen dimensions
func (v *Viewports) GetOrCreateMainViewport(content string) string {
	availableWidth := v.dimensions.Width

	viewportHeight := v.dimensions.Height - 3 // Reserve space for keybindings

	config := ViewportConfig{
		xpath:          config_attributes.NewXpath("main"),
		content:        content,
		pty:            nil,
		availableWidth: availableWidth,
		viewportHeight: viewportHeight,
		maxHeight:      viewportHeight,
		wrapContent:    false,
		useBorder:      false,
		isFullScrean:   true,
	}

	return v.getOrCreateViewportShared(config)
}

func (v *Viewports) RemoveIfExistsViewport(xpath config_attributes.Xpath) {
	v.viewports.Del(xpath)
}

// getMostSpecificViewport returns the most specific viewport (longest xpath) from a list of viewports
func (v *Viewports) getMostSpecificViewport(viewports []config_attributes.Xpath) config_attributes.Xpath {
	if len(viewports) == 0 {
		return config_attributes.Xpath{}
	}

	// Sort by length descending to get the most specific viewport
	for i := 0; i < len(viewports)-1; i++ {
		for j := i + 1; j < len(viewports); j++ {
			if viewports[i].Depth() < viewports[j].Depth() {
				viewports[i], viewports[j] = viewports[j], viewports[i]
			}
		}
	}

	return viewports[0]
}

// getViewportsUnderMouse returns all viewports that are under the mouse cursor
func (v *Viewports) getViewportsUnderMouse(mouseMsg tea.MouseMsg) []config_attributes.Xpath {
	var viewportsUnderMouse []config_attributes.Xpath
	for xpath := range v.viewports.Records() {
		if zone.Get(xpath.String()).InBounds(mouseMsg) {
			viewportsUnderMouse = append(viewportsUnderMouse, xpath)
		}
	}
	return viewportsUnderMouse
}

func (v *Viewports) Update(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	// Handle viewport selection only on mouse press, don't use MouseActionPress as it triggers also by just scrolling
	if mouseMsg, ok := msg.(tea.MouseMsg); ok && mouseMsg.Action == tea.MouseActionRelease {
		clickedViewports := v.getViewportsUnderMouse(mouseMsg)
		clickedViewport := v.getMostSpecificViewport(clickedViewports)
		anyViewportClicked := clickedViewport.Depth() > 0

		// Update viewport selection state
		for xpath, vpr := range v.viewports.Records() {
			if anyViewportClicked {
				// A viewport was clicked, activate it and deactivate others
				vpr.active = xpath == clickedViewport
			} else {
				// No viewport was clicked, deactivate all
				vpr.active = false
			}
		}
	}

	// Handle mouse wheel scrolling for wheel events - scroll viewport under cursor without activating it
	if mouseMsg, ok := msg.(tea.MouseMsg); ok && mouseMsg.Y != 0 {
		scrolledViewports := v.getViewportsUnderMouse(mouseMsg)
		mostSpecificViewport := v.getMostSpecificViewport(scrolledViewports)

		// Scroll the most specific viewport under the cursor without activating it
		vpr, ok := v.viewports.Get(mostSpecificViewport)
		if ok {
			mouseY := int(math.Abs(float64(mouseMsg.Y) / 3)) // Use larger steps for better precision

			v.debug.WriteString(fmt.Sprintf("%s: mouseY %d", mostSpecificViewport, mouseMsg.Y))

			if mouseMsg.Y > 0 {
				// Scroll down
				vpr.viewport.ScrollDown(mouseY)
			} else if mouseMsg.Y < 0 {
				// Scroll up
				vpr.viewport.ScrollUp(mouseY)
			}
		}
	}

	// Handle keyboard input and viewport updates for active viewports
	for _, vpr := range v.viewports.Records() {
		if vpr.active {
			if vpr.pty != nil {
				switch msg := msg.(type) {
				case tui_raw_key_reader.RawKeyReaderMsg:
					_, _ = vpr.pty.Write([]byte(msg)) // TODO: handle properly the DELETE keyboard events
				}
			}

			var cmd tea.Cmd
			vpr.viewport, cmd = vpr.viewport.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	// Special handling for main viewport - only allow keyboard scrolling when no other viewport is active
	hasActiveInnerViewport := false
	for xpath, vpr := range v.viewports.Records() {
		if xpath != config_attributes.NewXpath("main") && vpr.active {
			hasActiveInnerViewport = true
			break
		}
	}

	if mainVpr, ok := v.viewports.Get(config_attributes.NewXpath("main")); ok && !hasActiveInnerViewport {
		var cmd tea.Cmd
		mainVpr.viewport, cmd = mainVpr.viewport.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

func (v *Viewports) Debug() string {
	str := fmt.Sprintf("\nViewports: %d\n", v.viewports.Len())

	for pathx := range v.viewports.Records() {
		str += fmt.Sprintf("  '%s'\n", pathx)
	}

	return str
}
