package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/elliotchance/orderedmap/v3"
	zone "github.com/lrstanley/bubblezone"
)

type Viewports struct {
	viewports  *orderedmap.OrderedMap[string, *Viewport]
	dimensions *Dimensions
}

type Viewport struct {
	viewport viewport.Model
	active   bool
}

func NewViewports(dimensions *Dimensions) *Viewports {
	return &Viewports{
		viewports:  orderedmap.NewOrderedMap[string, *Viewport](),
		dimensions: dimensions,
	}
}

func (v *Viewports) GetOrCreateViewport(xpath string, content string) string {
	availableWidth := 140
	maxHeight := 4

	vprHeight := min(lipgloss.Height(content), maxHeight)

	vpr, ok := v.viewports.Get(xpath)
	if !ok {
		viewport := viewport.New(0, 0)
		viewport.GotoBottom()

		vpr = &Viewport{viewport: viewport}

		v.viewports.Set(xpath, vpr)
	}

	//truncatedContent := lipgloss.NewStyle().Width(availableWidth).Render(content)

	oldScrollPercent := vpr.viewport.ScrollPercent()
	vpr.viewport.SetContent(content)
	vpr.viewport.Height = vprHeight
	vpr.viewport.Width = availableWidth

	// Follow bottom if we are stuck at the buttom, and not if we have scrolled higher
	if oldScrollPercent == 1 {
		vpr.viewport.GotoBottom()
	}

	vprStr := fmt.Sprint(vpr.viewport.ScrollPercent(), " ", vpr.viewport.TotalLineCount(), " ") + vpr.viewport.View()

	final := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(DefaultColorScheme().TableBorder).
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
			switch msg.String() {
			case "press":
				vpr.active = zone.Get(xpath).InBounds(msg)
			}
		}

		if vpr.active {
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
