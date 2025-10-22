package tui_viewports

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kirill-scherba/omap"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_raw_key_reader"
)

type Viewports struct {
	viewports  *omap.Omap[string, *Viewport]
	dimensions *Dimensions
	colors     *config.ColorScheme
	debug      *strings.Builder
}

type Viewport struct {
	viewport      viewport.Model
	active        bool
	pty           *os.File
	minHeight     int
	scrollbarZone string // Zone ID for scrollbar area
	content       string // Store the content for double-click copying
}

type Dimensions struct {
	Width  int
	Height int
}

func NewViewports(dimensions *Dimensions, colors *config.ColorScheme, debug *strings.Builder) *Viewports {
	viewports, err := omap.New[string, *Viewport]()
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
func (v *Viewports) renderScrollbar(scrollPercent float64, totalLines, visibleLines int) string {
	if totalLines <= visibleLines {
		return "" // No scrollbar needed if all content is visible
	}

	scrollbarHeight := visibleLines
	if scrollbarHeight <= 0 {
		return ""
	}

	// Calculate the position of the scrollbar thumb
	thumbPosition := int(float64(scrollbarHeight-1) * scrollPercent)
	if thumbPosition < 0 {
		thumbPosition = 0
	}
	if thumbPosition >= scrollbarHeight {
		thumbPosition = scrollbarHeight - 1
	}

	// Build the scrollbar
	scrollbar := make([]string, scrollbarHeight)
	for i := range scrollbar {
		if i == thumbPosition {
			scrollbar[i] = "█" // Thumb
		} else {
			scrollbar[i] = "│" // Track
		}
	}

	// Style the scrollbar
	scrollbarStyle := lipgloss.NewStyle().
		Foreground(v.colors.TableBorder.GetForeground())

	// Join all lines with newlines
	return scrollbarStyle.Render(strings.Join(scrollbar, "\n"))
}

// ViewportConfig holds configuration options for creating or updating a viewport
type ViewportConfig struct {
	xpath          string
	content        string
	pty            *os.File
	availableWidth int
	viewportHeight int
	maxHeight      int
	wrapContent    bool
	useBorder      bool
	isMain         bool
}

// combineViewportWithScrollbar combines viewport content with scrollbar
func (v *Viewports) combineViewportWithScrollbar(viewportView, scrollbar string, scrollbarZone string) string {
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
			scrollbarLine := zone.Mark(scrollbarZone+fmt.Sprintf("-%d", i), scrollbarLines[i])
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
			scrollbarZone: config.xpath + "-scrollbar",
			active:        false, // Will be set appropriately based on context
			content:       config.content,
		}

		v.viewports.Set(config.xpath, vpr)
	}

	// Process content based on configuration
	var processedContent string
	if config.wrapContent {
		processedContent = lipgloss.NewStyle().Width(config.availableWidth).MaxWidth(config.availableWidth).Render(config.content)
	} else {
		processedContent = config.content
	}

	// Calculate height if needed
	var finalHeight int
	if config.isMain {
		finalHeight = config.viewportHeight
	} else {
		// Calculate height based on the wrapped content
		if config.maxHeight > 0 {
			finalHeight = min(lipgloss.Height(processedContent), config.maxHeight)
		} else {
			// No height limit specified, use full content height
			finalHeight = lipgloss.Height(processedContent)
		}
	}

	// Update viewport
	oldScrollPercent := vpr.viewport.ScrollPercent()
	vpr.viewport.SetContent(processedContent)
	vpr.viewport.Height = finalHeight
	vpr.viewport.Width = config.availableWidth

	// Follow bottom if we are stuck at the bottom, and not if we have scrolled higher
	if oldScrollPercent == 1 {
		vpr.viewport.GotoBottom()
	}

	viewportView := vpr.viewport.View()
	scrollPercent := vpr.viewport.ScrollPercent()
	totalLines := vpr.viewport.TotalLineCount()
	visibleLines := vpr.viewport.Height

	// Create scrollbar
	scrollbar := v.renderScrollbar(scrollPercent, totalLines, visibleLines)

	// Combine viewport content with scrollbar
	combinedView := v.combineViewportWithScrollbar(viewportView, scrollbar, vpr.scrollbarZone)

	// Apply border if needed
	if config.useBorder {
		borderColor := v.colors.TableBorder.GetForeground()
		if vpr.active {
			borderColor = v.colors.Error.GetBackground()
		}

		final := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Render(combinedView)

		return zone.Mark(config.xpath, final)
	} else {
		return zone.Mark(config.xpath, combinedView)
	}
}

func (v *Viewports) GetOrCreateViewport(xpath string, content string, pty *os.File, indentation int) string {
	// Calculate available width based on terminal width and indentation
	// Account for tree structure indentation, border, and padding
	baseIndentation := indentation * 2 // Each tree level typically takes 2 characters
	borderPadding := 6 + 8             // Border takes ~4 characters + some padding
	availableWidth := v.dimensions.Width - baseIndentation - borderPadding

	// Ensure minimum width
	if availableWidth < 40 {
		availableWidth = 40
	}

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
		isMain:         false,
	}

	return v.getOrCreateViewportShared(config)
}

// GetOrCreateLabelViewport creates or updates a viewport specifically for labels
func (v *Viewports) GetOrCreateLabelViewport(xpath string, content string, indentation int) string {
	// Calculate available width based on terminal width and indentation
	// Account for tree structure indentation, border, and padding
	baseIndentation := indentation * 2 // Each tree level typically takes 2 characters
	borderPadding := 2                 // Minimal padding for labels
	availableWidth := v.dimensions.Width - baseIndentation - borderPadding

	// Ensure minimum width
	if availableWidth < 40 {
		availableWidth = 40
	}

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
		isMain:         false,
	}

	return v.getOrCreateViewportShared(config)
}

// GetOrCreateMainViewport creates or updates the main viewport with full screen dimensions
func (v *Viewports) GetOrCreateMainViewport(content string) string {
	availableWidth := v.dimensions.Width - 2 // Account for scrollbar

	// Ensure minimum width
	if availableWidth < 40 {
		availableWidth = 40
	}

	viewportHeight := v.dimensions.Height - 3 // Reserve space for keybindings

	config := ViewportConfig{
		xpath:          "main",
		content:        content,
		pty:            nil,
		availableWidth: availableWidth,
		viewportHeight: viewportHeight,
		maxHeight:      viewportHeight,
		wrapContent:    false,
		useBorder:      false,
		isMain:         true,
	}

	return v.getOrCreateViewportShared(config)
}

func (v *Viewports) RemoveIfExistsViewport(xpath string) {
	v.viewports.Del(xpath)
}

// getMostSpecificViewport returns the most specific viewport (longest xpath) from a list of viewports
func (v *Viewports) getMostSpecificViewport(viewports []string) string {
	if len(viewports) == 0 {
		return ""
	}

	// Sort by length descending to get the most specific viewport
	for i := 0; i < len(viewports)-1; i++ {
		for j := i + 1; j < len(viewports); j++ {
			if len(viewports[i]) < len(viewports[j]) {
				viewports[i], viewports[j] = viewports[j], viewports[i]
			}
		}
	}

	return viewports[0]
}

// getViewportsUnderMouse returns all viewports that are under the mouse cursor
func (v *Viewports) getViewportsUnderMouse(mouseMsg tea.MouseMsg) []string {
	var viewportsUnderMouse []string
	for xpath := range v.viewports.Records() {
		if zone.Get(xpath).InBounds(mouseMsg) {
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
		anyViewportClicked := clickedViewport != ""

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

		if mostSpecificViewport != "" {
			// Scroll the most specific viewport under the cursor without activating it
			if vpr, ok := v.viewports.Get(mostSpecificViewport); ok {
				if mouseMsg.Y > 0 {
					// Scroll down - use larger steps for better precision
					for i := 0; i < 3; i++ {
						vpr.viewport.ScrollDown(1)
					}
				} else if mouseMsg.Y < 0 {
					// Scroll up - use larger steps for better precision
					for i := 0; i < 3; i++ {
						vpr.viewport.ScrollUp(1)
					}
				}
			}
		}
	}

	// Handle keyboard input and viewport updates for active viewports
	for _, vpr := range v.viewports.Records() {
		if vpr.active {
			if vpr.pty != nil {
				switch msg := msg.(type) {
				case tui_raw_key_reader.RawKeyReaderMsg:
					vpr.pty.Write([]byte(msg))
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
		if xpath != "main" && vpr.active {
			hasActiveInnerViewport = true
			break
		}
	}

	if mainVpr, ok := v.viewports.Get("main"); ok && !hasActiveInnerViewport {
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
