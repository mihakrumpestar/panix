package theme

import "github.com/mihakrumpestar/panix/pkg/tui/style"

// Default border drawing preset names per widget region: the fallback the derived
// ThemeBorder *Glyphs fields use for unset or unknown region names, matching the
// built-in widget defaults. These are derivation behavior, not theme data: the
// default theme itself lives in the embedded builtin/panix.yml.
const (
	defaultBorderTable        = "sharp"
	defaultBorderOverlay      = "sharp"
	defaultBorderViewport     = "rounded"
	defaultBorderNotification = "rounded"
)

// derive fills every render-ready field of the theme from its serializable fields
// and returns it. It is the single derivation pass: applied after every YAML
// decode, at the end of Merge and at the end of clone, so derived values are never
// stale.
func (t Theme) derive() Theme {
	t.Header.Title = t.Header.Title.derive()

	t.Status.OK = t.Status.OK.derive()
	t.Status.Warning = t.Status.Warning.derive()
	t.Status.Failed = t.Status.Failed.derive()
	t.Status.Running = t.Status.Running.derive()
	t.Status.Icons = t.Status.Icons.derive()

	t.Table.Border = t.Table.Border.derive()
	t.Table.Row = t.Table.Row.derive()
	t.Table.SelectionHighlightBackground = t.Table.SelectionHighlightBackground.derive()
	t.Table.SelectionHighlightBorder = t.Table.SelectionHighlightBorder.derive()

	t.PhaseStatus = t.PhaseStatus.derive()

	t.Chars = t.Chars.derive()
	t.Border = t.Border.derive()
	t.Tree = t.Tree.derive()
	t.Spinner = t.Spinner.derive()

	t.Footer.HelpKey = t.Footer.HelpKey.derive()
	t.Footer.HelpDesc = t.Footer.HelpDesc.derive()
	t.Footer.HelpSeparator = t.Footer.HelpSeparator.derive()
	t.Footer.HelpSelectedKey = t.Footer.HelpSelectedKey.derive()
	t.Footer.HelpSelectedDesc = t.Footer.HelpSelectedDesc.derive()
	t.Footer.NotificationBaseStyle = t.Footer.NotificationBaseStyle.derive()

	t.Flake = t.Flake.derive()
	t.Installable = t.Installable.derive()
	t.OutputType = t.OutputType.derive()
	t.Machine = t.Machine.derive()
	t.Phase = t.Phase.derive()
	t.Command = t.Command.derive()
	t.Error = t.Error.derive()

	return t
}

// derive renders the plain style values into Style. Setter order does not matter:
// style.Style computes its render prefix from the final state. Colors go through
// the shared parser: unset or invalid colors render uncolored (render tolerance;
// config input is validated up front).
func (s ThemeStyle) derive() ThemeStyle {
	built := style.NewStyle()
	if fg := style.ParseColorOrEmpty(string(s.Fg)); fg != "" {
		built = built.Foreground(fg)
	}

	if bg := style.ParseColorOrEmpty(string(s.Bg)); bg != "" {
		built = built.Background(bg)
	}

	if s.Bold != nil {
		built = built.Bold(*s.Bold)
	}

	if s.PadY != nil || s.PadX != nil {
		built = built.Padding(derefInt(s.PadY), derefInt(s.PadX))
	}

	s.Style = built

	return s
}

// derive fills the icon byte copies and returns the icons.
func (i ThemeStatusIcons) derive() ThemeStatusIcons {
	i.OKBytes = []byte(i.OK)
	i.FailedBytes = []byte(i.Failed)
	i.RunningBytes = []byte(i.Running)

	return i
}

// derive fills the glyph byte copies and returns the chars.
func (c ThemeChars) derive() ThemeChars {
	c.HeaderSeparatorBytes = []byte(c.HeaderSeparator)
	c.SnapshotIconBytes = []byte(c.SnapshotIcon)
	c.ErrorIconBytes = []byte(c.ErrorIcon)
	c.RowSpanMarkerBytes = []byte(c.RowSpanMarker)
	c.HeaderTitleSepBytes = []byte(c.HeaderTitleSep)
	c.PhaseArrowBytes = []byte(c.PhaseArrow)

	return c
}

// derive resolves the border presets and returns the border. Each region falls
// back to its default preset and finally to the built-in sharp preset, so a
// derived border is never empty.
func (b ThemeBorder) derive() ThemeBorder {
	b.TableGlyphs = buildBorder(b.Table, defaultBorderTable)
	b.OverlayGlyphs = buildBorder(b.Overlay, defaultBorderOverlay)
	b.ViewportGlyphs = buildBorder(b.Viewport, defaultBorderViewport)
	b.NotificationGlyphs = buildBorder(b.Notification, defaultBorderNotification)

	return b
}

// derive fills the connector byte copies, derives the enumerator style and
// returns the tree.
func (t ThemeTree) derive() ThemeTree {
	t.Enumerator = t.Enumerator.derive()
	t.BranchBytes = []byte(t.Branch)
	t.BranchLastBytes = []byte(t.BranchLast)
	t.VerticalBytes = []byte(t.Vertical)
	t.HorizontalBytes = []byte(t.Horizontal)

	return t
}

// derive fills the spinner frame byte copies and returns the spinner.
func (s ThemeSpinner) derive() ThemeSpinner {
	s.FrameBytes = buildFrames(s.Frames)

	return s
}

// derive derives the pill style and returns the phase status.
func (p ThemePhaseStatus) derive() ThemePhaseStatus {
	p.Pill = p.Pill.derive()

	return p
}

// derive fills the derived entity style and icon byte copy and returns the
// entity.
func (e ThemeLogEntity) derive() ThemeLogEntity {
	derived := ThemeStyle{Fg: e.Color, Bold: e.Bold}.derive()
	e.Style = derived.Style
	e.IconBytes = []byte(e.Icon)

	return e
}

// buildBorder resolves name to border glyphs, falling back to the region default
// preset and finally to sharp, so a derived border is never empty or panics.
func buildBorder(name, fallback string) style.Border {
	if border, ok := style.BorderByName(name); ok {
		return border
	}

	if border, ok := style.BorderByName(fallback); ok {
		return border
	}

	return style.NormalBorder()
}

func buildFrames(frames []string) [][]byte {
	built := make([][]byte, 0, len(frames))
	for frame := range frames {
		built = append(built, []byte(frames[frame]))
	}

	return built
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}

	return *v
}
