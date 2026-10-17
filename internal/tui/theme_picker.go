package tui

import (
	"bytes"
	"path/filepath"
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/pkg/tui/overlay"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
)

// Theme picker layout constants (see overlays.go for the shared ones).
const (
	themePickerCursorWidth = 2
	themePickerCursorGlyph = "▸"
	themePickerCellGap     = 2
	themePickerChromeRows  = 4
	themeActiveMarker      = "(active)"
	themePickerTitle       = "Themes"
)

// themePickerState is the modal state of the theme picker overlay (ctrl+t). The
// zero value means closed and is behavior-neutral.
type themePickerState struct {
	open     bool
	index    int    // selected row in Theme.Registry.Names()
	offset   int    // first visible row when the list scrolls
	prevName string // theme active before opening; esc reverts to it
}

// handleThemePickerToggle is the ctrl+t handler: opens the picker or, when
// already open, cancels it (revert + close) like esc.
func (m *model) handleThemePickerToggle() zeroterm.Cmd {
	if m.themePicker.open {
		m.revertThemePicker()

		return nil
	}

	m.openThemePicker()

	return nil
}

func (m *model) openThemePicker() {
	names := m.themeNames()
	if len(names) == 0 {
		return
	}

	m.themePicker = themePickerState{
		open:     true,
		index:    max(slices.Index(names, m.conf.Theme.Name), 0),
		prevName: m.conf.Theme.Name,
	}

	m.themePickerEnsureVisible(names)
}

// handleThemePickerKey handles one key while the picker is open. Quitting keys
// run the esc teardown first (close + revert the live preview), then the global
// handler, so a deferred quit cannot leave the live preview behind.
func (m *model) handleThemePickerKey(key string) zeroterm.Cmd {
	switch key {
	case "up", "k":
		m.moveThemePickerSelection(-1)
	case "down", "j":
		m.moveThemePickerSelection(1)
	case "enter":
		m.applyThemePickerChoice()
	case "esc", "ctrl+t", quitKey, forceQuitKey:
		m.revertThemePicker()

		return m.handleQuittingKey(key)
	}

	return nil
}

// handleQuittingKey dispatches a quitting key to its global handler, nil for any
// other key.
func (m *model) handleQuittingKey(key string) zeroterm.Cmd {
	switch key {
	case quitKey:
		return m.handleQuit()
	case forceQuitKey:
		return m.handleForceQuit()
	default:
		return nil
	}
}

// moveThemePickerSelection moves the selection by delta and live-previews the new
// theme. The selection clamps at the list edges.
func (m *model) moveThemePickerSelection(delta int) {
	names := m.themeNames()
	if len(names) == 0 {
		return
	}

	m.themePicker.index = min(max(m.themePicker.index+delta, 0), len(names)-1)
	m.themePickerEnsureVisible(names)
	m.previewTheme(names[m.themePicker.index])
}

// previewTheme swaps the active theme and re-themes every widget so the whole
// TUI previews live as the selection moves. It updates the model's runtime theme
// and writes the config theme name so the picker and session stay coherent.
// Unknown names are ignored.
func (m *model) previewTheme(name string) {
	if name == m.conf.Theme.Name && m.theme != nil {
		return
	}

	scheme, ok := m.conf.Theme.Registry.Build(name)
	if !ok {
		return
	}

	m.theme = scheme
	m.conf.Theme.Name = name
	m.applyTheme()
}

// revertThemePicker closes the picker and restores the theme active before it
// opened.
func (m *model) revertThemePicker() {
	m.themePicker.open = false
	m.previewTheme(m.themePicker.prevName)
}

// applyThemePickerChoice keeps the previewed theme for the session and closes the
// picker. Nothing is written to the config file; the in-memory config flag is
// updated so the session stays coherent.
func (m *model) applyThemePickerChoice() {
	m.themePicker.open = false

	m.conf.Flags.Tui.Theme = m.conf.Theme.Name
}

// themeNames returns the picker theme names, empty when no registry is resolved
// (e.g. hand built models in tests).
func (m *model) themeNames() []string {
	if m.conf.Theme.Registry == nil {
		return nil
	}

	return m.conf.Theme.Registry.Names()
}

func (m *model) themePickerVisibleRows(total int) int {
	contentH := m.dimensions.Height - m.header.Len() - m.footer.Len() - themePickerChromeRows

	return min(total, max(contentH, 1))
}

// themePickerEnsureVisible scrolls the list window so the selected row is
// visible and the offset stays within bounds.
func (m *model) themePickerEnsureVisible(names []string) {
	visible := m.themePickerVisibleRows(len(names))

	if m.themePicker.index < m.themePicker.offset {
		m.themePicker.offset = m.themePicker.index
	}

	if m.themePicker.index >= m.themePicker.offset+visible {
		m.themePicker.offset = m.themePicker.index - visible + 1
	}

	m.themePicker.offset = min(max(m.themePicker.offset, 0), max(len(names)-visible, 0))
}

// themePickerBox renders the picker overlay: the theme list with the selection
// highlighted. It reads the current (previewed) scheme every frame, so browsing
// restyles the picker together with the rest of the TUI. Key hints live in the
// footer while the picker is open.
func (m *model) themePickerBox() *overlay.Box {
	scheme := m.theme

	names := m.themeNames()
	if len(names) == 0 {
		return nil
	}

	provenances := make([]string, len(names))
	nameW, provW := 0, 0

	for idx, name := range names {
		provenances[idx] = themeProvenance(m.conf.Theme.Registry.Source(name))
		nameW = max(nameW, style.CellWidth([]byte(name)))
		provW = max(provW, style.CellWidth([]byte(provenances[idx])))
	}

	markerW := 0
	if slices.Contains(names, m.conf.Theme.Name) {
		markerW = style.CellWidth([]byte(themeActiveMarker))
	}

	rowW := themePickerCursorWidth + nameW + themePickerCellGap + provW
	if markerW > 0 {
		rowW += themePickerCellGap + markerW
	}

	selBg := scheme.Table.SelectionHighlightBackground.Style.GetBackground()
	lines := make([][]byte, 0, themePickerChromeRows+len(names))
	lines = append(lines, []byte{}) // spacer above the list

	end := min(m.themePicker.offset+m.themePickerVisibleRows(len(names)), len(names))

	for idx := m.themePicker.offset; idx < end; idx++ {
		marker := ""
		if names[idx] == m.conf.Theme.Name {
			marker = themeActiveMarker
		}

		lines = append(lines, themePickerRow(scheme, selBg, rowW, nameW, provW,
			names[idx], provenances[idx], marker, idx == m.themePicker.index))
	}

	lines = append(lines, []byte{}) // spacer below the list

	return overlay.NewBox(lines,
		overlay.WithStyle(overlayBoxStyle(scheme)),
		overlay.WithTitle(themePickerTitle),
		overlay.WithTitleStyle(scheme.Header.Title.Style),
		overlay.WithMaxWidth(m.overlayMaxWidth()),
	)
}

// themePickerRow renders one theme list row: cursor, name, provenance and the
// optional active marker, with columns padded so rows stay aligned. Selected rows
// carry the theme's selection background across the full row width.
func themePickerRow(
	scheme *theme.Theme,
	selBg style.Color,
	rowW, nameW, provW int,
	name, provenance, marker string,
	selected bool,
) []byte {
	cursor := bytes.Repeat([]byte{' '}, themePickerCursorWidth)

	if selected {
		cursor = styledCell(scheme.Footer.HelpSelectedKey.Style, themePickerCursorWidth, themePickerCursorGlyph+" ")
	}

	cells := [][]byte{
		cursor,
		styledCell(scheme.Table.Row.Style, nameW, name),
		bytes.Repeat([]byte{' '}, themePickerCellGap),
		styledCell(scheme.Footer.HelpDesc.Style, provW, provenance),
	}

	if marker != "" {
		cells = append(cells,
			bytes.Repeat([]byte{' '}, themePickerCellGap),
			styledCell(scheme.Footer.HelpSelectedKey.Style, 0, marker),
		)
	}

	row := style.NewStyle()
	if selected {
		row = row.Background(selBg)
	}

	return row.Width(rowW).RenderLine(bytes.Join(cells, nil))
}

// themeProvenance shortens a theme source for display: "builtin" for built-in
// themes, the base file name for user themes.
func themeProvenance(source string) string {
	if source == theme.SourceBuiltin {
		return theme.SourceBuiltin
	}

	return filepath.Base(source)
}
