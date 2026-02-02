package tui_viewports

import (
	"fmt"
	"math"
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
	viewports  *omap.Omap[config_attributes.Xpath, *Viewport]
	dimensions *Dimensions
	colors     *config.ColorScheme
	debug      *strings.Builder
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
	return &Viewports{viewports: v, dimensions: d, colors: c, debug: dbg}
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

func (v *Viewports) needsScrollbar(allowedHeight, contentHeight int) bool {
	return contentHeight > allowedHeight
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
		}
		v.viewports.Set(cfg.xpath, vpr)
	}

	width := cfg.availableWidth

	proc := cfg.content
	if cfg.wrapContent {
		proc = lipgloss.NewStyle().Width(width).Render(cfg.content)
	}
	contentHeight := lipgloss.Height(proc)

	allowedHeight := cfg.viewportHeight
	if allowedHeight <= 0 {
		allowedHeight = commandOutputMaxHeight
	}

	// If we need scrollbar
	if contentHeight > allowedHeight {
		width -= 2 // Adjust for scrollbar width
		if cfg.wrapContent {
			proc = lipgloss.NewStyle().Width(width).Render(cfg.content)
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
	vpr.viewport.SetContent(proc)
	if pct == 1 {
		vpr.viewport.GotoBottom()
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

	return v.getOrCreateViewport(ViewportConfig{
		xpath:          xpath,
		content:        content,
		availableWidth: v.dimensions.Width - indent,
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
		}
		if m.Y != 0 {
			if vp, ok := v.viewports.Get(v.mostSpecific(v.underMouse(m))); ok {
				v.debug.WriteString(fmt.Sprintf("%s: mouseY %d", v.mostSpecific(v.underMouse(m)), m.Y))
				n := int(math.Abs(float64(m.Y) / 3))
				if m.Y > 0 {
					vp.viewport.ScrollDown(n)
				} else {
					vp.viewport.ScrollUp(n)
				}
			}
		}
	}

	hasActiveInner := v.hasActiveInner(mainXpath)

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
