package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/elliotchance/orderedmap/v3"
	zone "github.com/lrstanley/bubblezone"
)

type Viewports struct {
	viewports  *orderedmap.OrderedMap[string, *Viewport]
	dimensions *Dimensions
	debug      *strings.Builder
	colors     ColorScheme
}

type Viewport struct {
	viewport      viewport.Model
	active        bool
	pty           *os.File
	minHeight     int
	scrollbarZone string // Zone ID for scrollbar area
	isDragging    bool   // Track if scrollbar thumb is being dragged
}

func NewViewports(dimensions *Dimensions, debug *strings.Builder, colors ColorScheme) *Viewports {
	return &Viewports{
		viewports:  orderedmap.NewOrderedMap[string, *Viewport](),
		dimensions: dimensions,
		debug:      debug,
		colors:     colors,
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

// handleScrollbarClick handles mouse clicks on the scrollbar and updates the viewport position
func (v *Viewports) handleScrollbarClick(xpath string, clickPosition, viewportHeight int) {
	vpr, ok := v.viewports.Get(xpath)
	if !ok {
		return
	}

	totalLines := vpr.viewport.TotalLineCount()
	if totalLines <= viewportHeight {
		return // No scrolling needed
	}

	// Calculate scroll percentage based on click position
	scrollPercent := float64(clickPosition) / float64(viewportHeight-1)
	if scrollPercent < 0 {
		scrollPercent = 0
	}
	if scrollPercent > 1 {
		scrollPercent = 1
	}

	// Calculate line number based on scroll percentage
	targetLine := int(float64(totalLines-viewportHeight) * scrollPercent)
	if targetLine < 0 {
		targetLine = 0
	}

	// Set the viewport position
	vpr.viewport.GotoTop()
	for i := 0; i < targetLine; i++ {
		vpr.viewport.LineDown(1)
	}
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

	// Don't exceed reasonable maximum width
	if availableWidth > 400 {
		availableWidth = 400
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
			isDragging:    false,
		}

		v.viewports.Set(xpath, vpr)
	}

	// Wrap content to fit within the available width
	wrappedContent := lipgloss.NewStyle().Width(availableWidth).MaxWidth(availableWidth).Render(content)

	// Calculate height based on the wrapped content, not the original content
	wrappedHeight := lipgloss.Height(wrappedContent)
	calculatedHeight := min(wrappedHeight, maxHeight)

	// Update the maximum height if the calculated height is greater
	if calculatedHeight > vpr.minHeight {
		vpr.minHeight = calculatedHeight
	}

	// Use the maximum height reached so far, but never exceed maxHeight
	vprHeight := min(vpr.minHeight, maxHeight)

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

func (v *Viewports) RemoveIfExistsViewport(xpath string) {
	v.viewports.Delete(xpath)
}

func (v *Viewports) Update(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	for xpath, vpr := range v.viewports.AllFromFront() {
		switch msg := msg.(type) {
		case tea.MouseMsg:
			//v.debug.WriteString(msg.String() + "\n")
			switch msg.Type {
			case tea.MouseLeft:
				// Check if click is on main viewport area
				if zone.Get(xpath).InBounds(msg) {
					vpr.active = true
					vpr.isDragging = false
				}
				// Check if click is on scrollbar area
				for i := 0; i < vpr.viewport.Height; i++ {
					scrollbarZoneID := vpr.scrollbarZone + fmt.Sprintf("-%d", i)
					if zone.Get(scrollbarZoneID).InBounds(msg) {
						vpr.active = true
						vpr.isDragging = true
						// Calculate scroll position based on click
						v.handleScrollbarClick(xpath, i, vpr.viewport.Height)
						break
					}
				}
			case tea.MouseRelease:
				vpr.isDragging = false
			case tea.MouseMotion:
				if vpr.isDragging && vpr.active {
					// Find which scrollbar zone the mouse is over
					for i := 0; i < vpr.viewport.Height; i++ {
						scrollbarZoneID := vpr.scrollbarZone + fmt.Sprintf("-%d", i)
						if zone.Get(scrollbarZoneID).InBounds(msg) {
							v.handleScrollbarClick(xpath, i, vpr.viewport.Height)
							break
						}
					}
				}
			}
		}

		if vpr.active {
			if vpr.pty != nil {
				switch msg := msg.(type) {
				case RawKeyReaderMsg:
					vpr.pty.Write(msg)
				}
			}

			var cmd tea.Cmd
			vpr.viewport, cmd = vpr.viewport.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return tea.Batch(cmds...)
}

func (v *Viewports) Debug() string {
	str := fmt.Sprintf("\nViewports: %d\n", v.viewports.Len())

	for pathx, _ := range v.viewports.AllFromFront() {
		str += fmt.Sprintf("  '%s'\n", pathx)
	}

	return str
}
