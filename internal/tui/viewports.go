package tui

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

func (v *Viewports) GetOrCreateViewport(xpath string, content string, pty *os.File, indentation int) string {
	// Calculate available width based on terminal width and indentation
	// Account for tree structure indentation, border, and padding
	baseIndentation := indentation * 2 // Each tree level typically takes 2 characters
	borderPadding := 6 + 8             // Border takes ~4 characters + some padding
	availableWidth := v.dimensions.width - baseIndentation - borderPadding

	// Ensure minimum width
	if availableWidth < 40 {
		availableWidth = 40
	}

	maxHeight := 8

	vpr, ok := v.viewports.Get(xpath)
	if !ok {
		viewport := viewport.New(0, 0)
		viewport.GotoBottom()

		vpr = &Viewport{
			viewport:      viewport,
			pty:           pty,
			minHeight:     0, // Initialize to 0, will be set on first content update
			scrollbarZone: xpath + "-scrollbar",
		}

		v.viewports.Set(xpath, vpr)
	}

	// Wrap content to fit within the available width
	wrappedContent := lipgloss.NewStyle().Width(availableWidth).MaxWidth(availableWidth).Render(content)

	// Calculate height based on the wrapped content, not the original content
	wrappedHeight := lipgloss.Height(wrappedContent)
	calculatedHeight := min(wrappedHeight, maxHeight)

	// Use the maximum height reached so far, but never exceed maxHeight
	vprHeight := min(calculatedHeight)

	oldScrollPercent := vpr.viewport.ScrollPercent()
	vpr.viewport.SetContent(wrappedContent)
	vpr.viewport.Height = vprHeight
	vpr.viewport.Width = availableWidth

	// Follow bottom if we are stuck at the buttom, and not if we have scrolled higher
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
	var combinedView string
	if scrollbar != "" {
		// Split viewport view into lines and combine with scrollbar
		viewportLines := strings.Split(viewportView, "\n")
		scrollbarLines := strings.Split(scrollbar, "\n")

		combinedLines := make([]string, len(viewportLines))
		for i, line := range viewportLines {
			if i < len(scrollbarLines) {
				// Add spacing and zone the scrollbar
				scrollbarLine := zone.Mark(vpr.scrollbarZone+fmt.Sprintf("-%d", i), scrollbarLines[i])
				combinedLines[i] = line + " " + scrollbarLine
			} else {
				combinedLines[i] = line
			}
		}
		combinedView = strings.Join(combinedLines, "\n")
	} else {
		combinedView = viewportView
	}

	borderColor := v.colors.TableBorder.GetForeground()
	if vpr.active {
		borderColor = v.colors.Error.GetBackground()
	}

	final := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Render(combinedView)

	finalZoneMarked := zone.Mark(xpath, final)

	return finalZoneMarked
}

// GetOrCreateMainViewport creates or updates the main viewport with full screen dimensions
func (v *Viewports) GetOrCreateMainViewport(content string) string {
	availableWidth := v.dimensions.width - 2 // Account for scrollbar

	// Ensure minimum width
	if availableWidth < 40 {
		availableWidth = 40
	}

	viewportHeight := v.dimensions.height - 3 // Reserve space for keybindings

	vpr, ok := v.viewports.Get("main")
	if !ok {
		viewport := viewport.New(availableWidth, viewportHeight)
		viewport.GotoBottom()

		vpr = &Viewport{
			viewport:      viewport,
			pty:           nil,
			minHeight:     viewportHeight,
			scrollbarZone: "main-scrollbar",
			active:        false, // Main viewport starts inactive, can be activated by click
		}

		v.viewports.Set("main", vpr)
	}

	// Set content directly without wrapping for better performance
	oldScrollPercent := vpr.viewport.ScrollPercent()
	vpr.viewport.SetContent(content)
	vpr.viewport.Height = viewportHeight
	vpr.viewport.Width = availableWidth

	// Follow bottom if we are stuck at the bottom, and not if we have scrolled higher
	if oldScrollPercent == 1 {
		vpr.viewport.GotoBottom()
	}

	viewportView := vpr.viewport.View()
	scrollPercent := vpr.viewport.ScrollPercent()
	totalLines := vpr.viewport.TotalLineCount()
	visibleLines := vpr.viewport.Height

	// Always create scrollbar for main viewport if there's content
	scrollbar := v.renderScrollbar(scrollPercent, totalLines, visibleLines)

	// Combine viewport content with scrollbar without border
	if scrollbar != "" {
		// Split viewport view into lines and combine with scrollbar
		viewportLines := strings.Split(viewportView, "\n")
		scrollbarLines := strings.Split(scrollbar, "\n")

		combinedLines := make([]string, len(viewportLines))
		for i, line := range viewportLines {
			if i < len(scrollbarLines) {
				// Add spacing and zone the scrollbar
				scrollbarLine := zone.Mark(vpr.scrollbarZone+fmt.Sprintf("-%d", i), scrollbarLines[i])
				combinedLines[i] = line + " " + scrollbarLine
			} else {
				combinedLines[i] = line
			}
		}
		return zone.Mark("main", strings.Join(combinedLines, "\n"))
	} else {
		return zone.Mark("main", viewportView)
	}
}

func (v *Viewports) RemoveIfExistsViewport(xpath string) {
	v.viewports.Del(xpath)
}

func (v *Viewports) Update(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	// Handle viewport selection only on mouse press, don't use MouseActionPress as it triggers also by just scrolling
	if mouseMsg, ok := msg.(tea.MouseMsg); ok && mouseMsg.Action == tea.MouseActionRelease {
		var clickedViewport string
		var anyViewportClicked bool

		// Check which viewport was clicked - prioritize inner viewports over main
		var clickedViewports []string
		for xpath := range v.viewports.Records() {
			if zone.Get(xpath).InBounds(mouseMsg) {
				clickedViewports = append(clickedViewports, xpath)
			}
		}

		// If multiple viewports were clicked (nested), prioritize the most specific one (longest xpath)
		if len(clickedViewports) > 0 {
			// Sort by length descending to get the most specific viewport
			for i := 0; i < len(clickedViewports)-1; i++ {
				for j := i + 1; j < len(clickedViewports); j++ {
					if len(clickedViewports[i]) < len(clickedViewports[j]) {
						clickedViewports[i], clickedViewports[j] = clickedViewports[j], clickedViewports[i]
					}
				}
			}
			clickedViewport = clickedViewports[0]
			anyViewportClicked = true
		}

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
		// Check if any viewport is under the mouse for wheel scrolling - prioritize inner viewports
		var scrolledViewports []string
		for xpath := range v.viewports.Records() {
			if zone.Get(xpath).InBounds(mouseMsg) {
				scrolledViewports = append(scrolledViewports, xpath)
			}
		}

		// If multiple viewports are under the mouse, scroll the most specific one
		if len(scrolledViewports) > 0 {
			// Sort by length descending to get the most specific viewport
			for i := 0; i < len(scrolledViewports)-1; i++ {
				for j := i + 1; j < len(scrolledViewports); j++ {
					if len(scrolledViewports[i]) < len(scrolledViewports[j]) {
						scrolledViewports[i], scrolledViewports[j] = scrolledViewports[j], scrolledViewports[i]
					}
				}
			}

			// Scroll the most specific viewport under the cursor without activating it
			if vpr, ok := v.viewports.Get(scrolledViewports[0]); ok {
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
				case RawKeyReaderMsg:
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

func (v *Viewports) ActivateViewport(xpathToActivate string) {
	for xpath, vpr := range v.viewports.Records() {
		vpr.active = xpath == xpathToActivate
	}
}
