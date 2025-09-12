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
	viewport  viewport.Model
	active    bool
	pty       *os.File
	minHeight int
}

func NewViewports(dimensions *Dimensions, debug *strings.Builder, colors ColorScheme) *Viewports {
	return &Viewports{
		viewports:  orderedmap.NewOrderedMap[string, *Viewport](),
		dimensions: dimensions,
		debug:      debug,
		colors:     colors,
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
			viewport:  viewport,
			pty:       pty,
			minHeight: 0, // Initialize to 0, will be set on first content update
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

	vprStr := fmt.Sprint(vpr.viewport.ScrollPercent(), " ", vpr.viewport.TotalLineCount(), " ") + vpr.viewport.View()

	borderColor := v.colors.TableBorder.GetForeground()
	if vpr.active {
		borderColor = v.colors.Error.GetBackground()
	}

	final := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Render(vprStr)

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
			switch msg.String() {
			case "left press":
				vpr.active = zone.Get(xpath).InBounds(msg)
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
