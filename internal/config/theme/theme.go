package theme

import (
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
)

// Theme is the single, render-ready theme type: the serializable surface
// (colors, glyphs, border preset names, spinner settings) plus derived render
// values (style.Style, style.Border, byte glyphs) tagged yaml:"-". It may be
// partial: unset fields inherit from a base theme via Merge.
//
// Every construction path ends with derive, so derived fields are never stale:
// after YAML decode, in Merge and in clone.
//
// Colors are parsed by style.ParseColor: hex "#RRGGBB" or a decimal 0-255 palette
// index. Pairs are all-or-nothing: a pair must be fully unset (both sides empty)
// or fully set; a half pair is invalid and fails validation. Invalid values fail
// theme validation at eval. Pointer fields (bold, pad_y, pad_x) distinguish unset
// (nil, inherit from the base theme) from an explicit zero.
type Theme struct {
	Mode         string            `yaml:"mode,omitempty" json:"mode,omitempty" desc:"Terminal background the theme targets" validate:"omitempty,oneof=dark light"` //nolint:lll // struct tag is one literal; keys, desc and enum are public contract
	Header       ThemeHeader       `yaml:"header,omitempty" json:"header" desc:"Header title styling"`
	Status       ThemeStatus       `yaml:"status,omitempty" json:"status" desc:"Status colors and icons"`
	Table        ThemeTableAndLogs `yaml:"table,omitempty" json:"table" desc:"Stats table and log list styling"`
	PhaseStatus  ThemePhaseStatus  `yaml:"phase_status,omitempty" json:"phase_status" desc:"Phase status pill colors"`
	Chars        ThemeChars        `yaml:"chars,omitempty" json:"chars" desc:"UI glyphs"`
	Border       ThemeBorder       `yaml:"border,omitempty" json:"border" desc:"Border drawing presets per widget region"`
	Tree         ThemeTree         `yaml:"tree,omitempty" json:"tree" desc:"Tree enumerator styling"`
	Spinner      ThemeSpinner      `yaml:"spinner,omitempty" json:"spinner" desc:"Spinner frames and interval"`
	Footer       ThemeFooter       `yaml:"footer,omitempty" json:"footer" desc:"Footer help line styling"`
	Notification ThemeNotification `yaml:"notification,omitempty" json:"notification" desc:"Notification styling"`

	Flake       ThemeLogEntity `yaml:"flake,omitempty" json:"flake" desc:"Log entity: flake"`
	Installable ThemeLogEntity `yaml:"installable,omitempty" json:"installable" desc:"Log entity: installable"`
	OutputType  ThemeLogEntity `yaml:"output_type,omitempty" json:"output_type" desc:"Log entity: output type"`
	Machine     ThemeLogEntity `yaml:"machine,omitempty" json:"machine" desc:"Log entity: machine"`
	Phase       ThemeLogEntity `yaml:"phase,omitempty" json:"phase" desc:"Log entity: phase"`
	Command     ThemeLogEntity `yaml:"command,omitempty" json:"command" desc:"Log entity: command"`
	Error       ThemeLogEntity `yaml:"error,omitempty" json:"error" desc:"Log entity: error"`
}

// ThemeStyle is the plain form of a style.Style role: optional foreground and
// background colors, bold and padding. Style is the derived render value.
type ThemeStyle struct {
	Fg    style.Color `yaml:"fg,omitempty" json:"fg,omitempty" desc:"Foreground color: hex #RRGGBB or 0-255 palette index"`
	Bg    style.Color `yaml:"bg,omitempty" json:"bg,omitempty" desc:"Background color: hex #RRGGBB or 0-255 palette index"`
	Bold  *bool       `yaml:"bold,omitempty" json:"bold,omitempty" desc:"Bold text; unset inherits from the base theme"`
	PadY  *int        `yaml:"pad_y,omitempty" json:"pad_y,omitempty" desc:"Vertical padding; unset inherits from the base theme"`
	PadX  *int        `yaml:"pad_x,omitempty" json:"pad_x,omitempty" desc:"Horizontal padding; unset inherits from the base theme"`
	Style style.Style `yaml:"-" json:"-"`
}

// ThemePair is a two-color gradient pair: [from, to]. The named array preserves
// the YAML and schema shape (a fixed two-item sequence). A pair is either fully
// set or fully empty; a half pair is invalid.
type ThemePair [2]style.Color

// ThemeHeader mirrors the header section.
type ThemeHeader struct {
	Title ThemeStyle `yaml:"title,omitempty" json:"title" desc:"Title styling"`
}

// ThemeStatusIcons holds the status glyphs as strings. The *Bytes fields are
// the derived render values.
type ThemeStatusIcons struct {
	OK           string `yaml:"ok,omitempty" json:"ok,omitempty" desc:"Icon for OK status" glyph:""`
	OKBytes      []byte `yaml:"-" json:"-"`
	Failed       string `yaml:"failed,omitempty" json:"failed,omitempty" desc:"Icon for failed status" glyph:""`
	FailedBytes  []byte `yaml:"-" json:"-"`
	Running      string `yaml:"running,omitempty" json:"running,omitempty" desc:"Icon for running status" glyph:""`
	RunningBytes []byte `yaml:"-" json:"-"`
}

// ThemeStatus mirrors the status section.
type ThemeStatus struct {
	OK      ThemeStyle       `yaml:"ok,omitempty" json:"ok" desc:"OK/success styling"`
	Warning ThemeStyle       `yaml:"warning,omitempty" json:"warning" desc:"Warning styling"`
	Failed  ThemeStyle       `yaml:"failed,omitempty" json:"failed" desc:"Failed/error styling"`
	Running ThemeStyle       `yaml:"running,omitempty" json:"running" desc:"Running/info styling"`
	Icons   ThemeStatusIcons `yaml:"icons,omitempty" json:"icons" desc:"Status icons"`
}

// ThemeChars holds the UI glyphs as strings. The *Bytes fields are the derived
// render values.
type ThemeChars struct {
	HeaderSeparator      string `yaml:"header_separator,omitempty" json:"header_separator,omitempty" desc:"Separator between header columns" glyph:""`
	HeaderSeparatorBytes []byte `yaml:"-" json:"-"`
	SnapshotIcon         string `yaml:"snapshot_icon,omitempty" json:"snapshot_icon,omitempty" desc:"Snapshot icon" glyph:""`
	SnapshotIconBytes    []byte `yaml:"-" json:"-"`
	ErrorIcon            string `yaml:"error_icon,omitempty" json:"error_icon,omitempty" desc:"Error icon" glyph:""`
	ErrorIconBytes       []byte `yaml:"-" json:"-"`
	RowSpanMarker        string `yaml:"row_span_marker,omitempty" json:"row_span_marker,omitempty" desc:"Marker for rows spanning multiple lines" glyph:""` //nolint:lll // struct tag is one literal
	RowSpanMarkerBytes   []byte `yaml:"-" json:"-"`
	HeaderTitleSep       string `yaml:"header_title_sep,omitempty" json:"header_title_sep,omitempty" desc:"Separator between header title and value" glyph:""` //nolint:lll // struct tag is one literal
	HeaderTitleSepBytes  []byte `yaml:"-" json:"-"`
	PhaseArrow           string `yaml:"phase_arrow,omitempty" json:"phase_arrow,omitempty" desc:"Arrow shown before phase names" glyph:""`
	PhaseArrowBytes      []byte `yaml:"-" json:"-"`
}

// ThemeBorder names the border preset per widget region (style.BorderByName:
// sharp, rounded, double, heavy, ascii, dashed, blank, markdown). An empty name
// inherits from the base theme; an unknown name fails validation at eval. The
// *Glyphs fields are the derived borders: named preset, else region default,
// else sharp, so a border is never empty.
type ThemeBorder struct {
	Table              string       `yaml:"table,omitempty" json:"table,omitempty" desc:"Border preset" validate:"omitempty,oneof=ascii blank dashed double heavy markdown rounded sharp"` //nolint:lll // struct tag is one literal; keys, desc and enum are public contract
	TableGlyphs        style.Border `yaml:"-" json:"-"`
	Overlay            string       `yaml:"overlay,omitempty" json:"overlay,omitempty" desc:"Border preset" validate:"omitempty,oneof=ascii blank dashed double heavy markdown rounded sharp"` //nolint:lll // struct tag is one literal; keys, desc and enum are public contract
	OverlayGlyphs      style.Border `yaml:"-" json:"-"`
	Viewport           string       `yaml:"viewport,omitempty" json:"viewport,omitempty" desc:"Border preset" validate:"omitempty,oneof=ascii blank dashed double heavy markdown rounded sharp"` //nolint:lll // struct tag is one literal; keys, desc and enum are public contract
	ViewportGlyphs     style.Border `yaml:"-" json:"-"`
	Notification       string       `yaml:"notification,omitempty" json:"notification,omitempty" desc:"Border preset" validate:"omitempty,oneof=ascii blank dashed double heavy markdown rounded sharp"` //nolint:lll // struct tag is one literal; keys, desc and enum are public contract
	NotificationGlyphs style.Border `yaml:"-" json:"-"`
}

// ThemeTableAndLogs mirrors the stats table and log list section.
type ThemeTableAndLogs struct {
	Border                       ThemeStyle `yaml:"border,omitempty" json:"border" desc:"Table border styling"`
	Row                          ThemeStyle `yaml:"row,omitempty" json:"row" desc:"Table row styling"`
	SelectionHighlightBackground ThemeStyle `yaml:"selection_highlight_background,omitempty" json:"selection_highlight_background" desc:"Selection highlight background"` //nolint:lll // struct tag is one literal; keys and desc are public contract
	SelectionHighlightBorder     ThemeStyle `yaml:"selection_highlight_border,omitempty" json:"selection_highlight_border" desc:"Selection highlight border"`             //nolint:lll // struct tag is one literal; keys and desc are public contract
}

// ThemeTree mirrors the tree section. The *Bytes fields are derived.
type ThemeTree struct {
	Enumerator      ThemeStyle `yaml:"enumerator,omitempty" json:"enumerator" desc:"Tree enumerator (branch lines) styling"`
	Branch          string     `yaml:"branch,omitempty" json:"branch,omitempty" desc:"Connector glyph before a non-last child (single cell)" glyph:""`
	BranchBytes     []byte     `yaml:"-" json:"-"`
	BranchLast      string     `yaml:"branch_last,omitempty" json:"branch_last,omitempty" desc:"Connector glyph before the last child (single cell)" glyph:""` //nolint:lll // struct tag is one literal
	BranchLastBytes []byte     `yaml:"-" json:"-"`
	Vertical        string     `yaml:"vertical,omitempty" json:"vertical,omitempty" desc:"Continuation glyph for a non-last parent (single cell)" glyph:""`       //nolint:lll // struct tag is one literal
	VerticalBytes   []byte     `yaml:"-" json:"-"`                                                                                                                //nolint:lll // derived
	Horizontal      string     `yaml:"horizontal,omitempty" json:"horizontal,omitempty" desc:"Horizontal run glyph inside the connectors (single cell)" glyph:""` //nolint:lll // struct tag is one literal
	HorizontalBytes []byte     `yaml:"-" json:"-"`
}

// ThemePhaseStatus mirrors the phase status section: gradient pairs per phase
// state and the pill style drawn on top of them.
type ThemePhaseStatus struct {
	Running ThemePair  `yaml:"running,omitempty" json:"running,omitempty" desc:"Gradient pair [from, to] for running phases"`
	Failed  ThemePair  `yaml:"failed,omitempty" json:"failed,omitempty" desc:"Gradient pair [from, to] for failed phases"`
	Done    ThemePair  `yaml:"done,omitempty" json:"done,omitempty" desc:"Gradient pair [from, to] for done phases"`
	Default ThemePair  `yaml:"default,omitempty" json:"default,omitempty" desc:"Gradient pair [from, to] for other phases"`
	Pill    ThemeStyle `yaml:"pill,omitempty" json:"pill" desc:"Phase status pill label styling"`
}

// ThemeSpinner mirrors the spinner section. FrameBytes is the derived render
// value; Interval.Duration() is a free conversion, so it needs no field.
type ThemeSpinner struct {
	Frames     []string       `yaml:"frames,omitempty" json:"frames,omitempty" desc:"Spinner frames cycled while running" glyph:""`
	FrameBytes [][]byte       `yaml:"-" json:"-"`
	Interval   yamlx.Duration `yaml:"interval,omitempty" json:"interval,omitempty" desc:"Frame interval: duration string like 120ms or integer nanoseconds"` //nolint:lll // struct tag is one literal
}

// ThemeFooter mirrors the footer section.
type ThemeFooter struct {
	HelpKey               ThemeStyle `yaml:"help_key,omitempty" json:"help_key" desc:"Help key styling"`
	HelpDesc              ThemeStyle `yaml:"help_desc,omitempty" json:"help_desc" desc:"Help description styling"`
	HelpSeparator         ThemeStyle `yaml:"help_separator,omitempty" json:"help_separator" desc:"Help key/description separator styling"`
	HelpSelectedKey       ThemeStyle `yaml:"help_selected_key,omitempty" json:"help_selected_key" desc:"Selected help key styling"`
	HelpSelectedDesc      ThemeStyle `yaml:"help_selected_desc,omitempty" json:"help_selected_desc" desc:"Selected help description styling"`
	NotificationBaseStyle ThemeStyle `yaml:"notification_base_style,omitempty" json:"notification_base_style" desc:"Base styling for notifications"`
}

// ThemeNotification mirrors the notification section. DefaultFgColor is already
// render-ready.
type ThemeNotification struct {
	DefaultFgColor style.Color `yaml:"default_fg_color,omitempty" json:"default_fg_color,omitempty" desc:"Default notification foreground color: hex #RRGGBB or 0-255 palette index"` //nolint:lll // struct tag is one literal; keys and desc are public contract
}

// ThemeLogEntity mirrors one log entity. Style and IconBytes are derived.
type ThemeLogEntity struct {
	Color     style.Color `yaml:"color,omitempty" json:"color,omitempty" desc:"Foreground color: hex #RRGGBB or 0-255 palette index"`
	Icon      string      `yaml:"icon,omitempty" json:"icon,omitempty" desc:"Icon glyph" glyph:""`
	IconBytes []byte      `yaml:"-" json:"-"`
	Bold      *bool       `yaml:"bold,omitempty" json:"bold,omitempty" desc:"Bold text; unset inherits from the base theme"`
	Style     style.Style `yaml:"-" json:"-"`
}

// prepareTheme is the single validate-then-derive step shared by the embedded
// loader and ParseThemesFile, so callers never observe a stale derived value.
func prepareTheme(theme Theme) (Theme, error) {
	err := theme.Validate()
	if err != nil {
		return Theme{}, err
	}

	return theme.derive(), nil
}

// DefaultTheme returns the built-in "panix" theme: the parsed serializable value
// with its derived render fields, shared by the registry and the runtime.
func DefaultTheme() *Theme {
	theme := defaultTheme()

	return &theme
}
