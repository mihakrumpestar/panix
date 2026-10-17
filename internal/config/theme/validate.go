package theme

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/pkg/errors"

	"github.com/mihakrumpestar/panix/pkg/tui/spinner"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

const (
	// maxThemePadding is the largest padding a theme may set per axis. The
	// style renderer clamps defensively as well, but config input must fail at
	// eval instead of relying on that clamp.
	maxThemePadding = 16
)

// validator is implemented by the theme value types with field level rules,
// reached by reflection from Theme.Validate.
type validator interface {
	validate(path string) error
}

// Validate checks every field of a parsed theme against the documented value
// grammar: colors through style.ParseColor, border preset names through
// style.BorderByName, padding and spinner interval bounds, glyph control
// characters and spinner frame renderer constraints. Zero/empty values are valid
// and mean "inherit from the base theme". The error names the YAML field path and
// the offending value.
//
// It runs on every theme parsed from a themes file, before merging, so garbage
// anywhere fails at eval even when another file overrides the field.
func (t Theme) Validate() error {
	err := validateMode(t.Mode)
	if err != nil {
		return err
	}

	return validateThemeValue("", reflect.ValueOf(t))
}

// validateMode checks the theme mode against the documented enum. An empty mode
// is valid (unset).
func validateMode(mode string) error {
	switch mode {
	case "", "dark", "light":
		return nil
	default:
		return errors.Errorf("mode: must be \"dark\" or \"light\", got %q", mode)
	}
}

// validateGlyphField rejects glyph strings carrying control characters:
// newlines and escape sequences would corrupt the header layout or, once
// enumerated by GlyphsOf, the terminal width probe script.
func validateGlyphField(path, value string) error {
	if value == "" {
		return nil
	}

	if strings.ContainsFunc(value, unicode.IsControl) {
		return errors.Errorf("%s: glyph must not contain control characters", path)
	}

	return nil
}

// validateThemeValue validates one theme value at the YAML field path: types
// implementing validator validate their fields, plain structs recurse field by
// field deriving paths from the yaml tags.
func validateThemeValue(path string, value reflect.Value) error {
	if value.CanInterface() {
		if validatable, ok := reflect.TypeAssert[validator](value); ok {
			return validatable.validate(path)
		}
	}

	if value.Kind() != reflect.Struct {
		return nil
	}

	typ := value.Type()

	for index := range value.NumField() {
		field := typ.Field(index)
		if isDerived(field) {
			continue // derived render values are never user-supplied data
		}

		err := validateThemeValue(yamlFieldPath(path, field), value.Field(index))
		if err != nil {
			return err
		}
	}

	return nil
}

// yamlFieldPath extends path with the field's yaml key ("section.field").
func yamlFieldPath(path string, field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
	if name == "" {
		name = field.Name
	}

	if path == "" {
		return name
	}

	return path + "." + name
}

// validateColorField checks one color field; "" (unset) is valid.
func validateColorField(path string, value style.Color) error {
	if value == "" {
		return nil
	}

	_, err := style.ParseColor(string(value))

	return errors.Wrapf(err, "%s", path)
}

// validate checks the style's color and padding fields.
func (s ThemeStyle) validate(path string) error {
	err := validateColorField(path+".fg", s.Fg)
	if err != nil {
		return err
	}

	err = validateColorField(path+".bg", s.Bg)
	if err != nil {
		return err
	}

	err = validatePaddingField(path+".pad_y", s.PadY)
	if err != nil {
		return err
	}

	return validatePaddingField(path+".pad_x", s.PadX)
}

// validatePaddingField checks one padding pointer against the theme bounds; nil
// (unset) is valid.
func validatePaddingField(path string, value *int) error {
	if value == nil {
		return nil
	}

	if *value < 0 || *value > maxThemePadding {
		return errors.Errorf("%s: padding must be between 0 and %d cells, got %d", path, maxThemePadding, *value)
	}

	return nil
}

// validate checks both gradient pair sides. Pairs are all-or-nothing: a pair
// with one empty side is invalid, because it is not zero and merge would let it
// replace the base pair, rendering the empty side black.
func (p ThemePair) validate(path string) error {
	for side, color := range p {
		other := p[1-side]
		if color == "" && other != "" {
			return errors.Errorf("%s[%d]: gradient pair is incomplete; set both sides or neither", path, side)
		}
	}

	for side, color := range p {
		err := validateColorField(fmt.Sprintf("%s[%d]", path, side), color)
		if err != nil {
			return err
		}
	}

	return nil
}

// validate checks the border preset names against the style.BorderByName
// registry; "" names are valid (unset).
func (b ThemeBorder) validate(path string) error {
	regions := []struct {
		name  string
		value string
	}{
		{"table", b.Table},
		{"overlay", b.Overlay},
		{"viewport", b.Viewport},
		{"notification", b.Notification},
	}

	for _, region := range regions {
		if region.value == "" {
			continue
		}

		if _, ok := style.BorderByName(region.value); !ok {
			return errors.Errorf("%s.%s: unknown border preset %q (available: %s)",
				path, region.name, region.value, strings.Join(style.BorderNames(), ", "))
		}
	}

	return nil
}

// validate checks the spinner frames and interval: every frame non-empty and all
// frames the same positive display width (the renderer cycles them in place), and
// the interval within the bounds the tick scheduler accepts. An empty list and a
// zero interval are valid (unset).
func (s ThemeSpinner) validate(path string) error {
	width := 0

	for frameIdx, frame := range s.Frames {
		framePath := fmt.Sprintf("%s.frames[%d]", path, frameIdx)

		err := validateGlyphField(framePath, frame)
		if err != nil {
			return err
		}

		if frame == "" {
			return errors.Errorf("%s: spinner frame must not be empty", framePath)
		}

		frameWidth := style.CellWidth([]byte(frame))
		if frameWidth == 0 {
			return errors.Errorf("%s: spinner frame must be at least one cell wide", framePath)
		}

		if frameIdx == 0 {
			width = frameWidth

			continue
		}

		if frameWidth != width {
			return errors.Errorf("%s: spinner frame width %d differs from %d of the first frame", framePath, frameWidth, width)
		}
	}

	if s.Interval != 0 {
		interval := s.Interval.Duration()
		if interval < spinner.MinInterval || interval > spinner.MaxInterval {
			return errors.Errorf("%s.interval: spinner interval must be between %s and %s, got %s",
				path, spinner.MinInterval, spinner.MaxInterval, interval)
		}
	}

	return nil
}

// validate checks the notification default foreground color.
func (n ThemeNotification) validate(path string) error {
	return validateColorField(path+".default_fg_color", n.DefaultFgColor)
}

// validate checks the log entity color and icon.
func (e ThemeLogEntity) validate(path string) error {
	err := validateColorField(path+".color", e.Color)
	if err != nil {
		return err
	}

	return validateGlyphField(path+".icon", e.Icon)
}

// validate checks every status icon glyph for control characters.
func (i ThemeStatusIcons) validate(path string) error {
	return validateGlyphFields(path, []glyphField{
		{"ok", i.OK},
		{"failed", i.Failed},
		{"running", i.Running},
	})
}

// validate checks every UI glyph for control characters.
func (c ThemeChars) validate(path string) error {
	return validateGlyphFields(path, []glyphField{
		{"header_separator", c.HeaderSeparator},
		{"snapshot_icon", c.SnapshotIcon},
		{"error_icon", c.ErrorIcon},
		{"row_span_marker", c.RowSpanMarker},
		{"header_title_sep", c.HeaderTitleSep},
		{"phase_arrow", c.PhaseArrow},
	})
}

// glyphField is one named glyph string at a theme field path.
type glyphField struct {
	name  string
	value string
}

// validateGlyphFields validates a named set of glyph fields in order.
func validateGlyphFields(path string, glyphs []glyphField) error {
	for _, glyph := range glyphs {
		err := validateGlyphField(path+"."+glyph.name, glyph.value)
		if err != nil {
			return err
		}
	}

	return nil
}

// validate checks the tree connector glyphs, then recurses into the enumerator
// style. The four glyphs are validated as a set, mirroring the spinner rule: a
// fully unset set is valid (a partial theme inherits all four from the base), but
// once any glyph is set all four must be present and exactly one display cell
// wide. The tree layout composes per-level prefixes from these glyphs and the
// indent step, so a missing or multi-cell glyph would break alignment.
func (t ThemeTree) validate(path string) error {
	glyphs := []glyphField{
		{"branch", t.Branch},
		{"branch_last", t.BranchLast},
		{"vertical", t.Vertical},
		{"horizontal", t.Horizontal},
	}

	set := 0

	for _, glyph := range glyphs {
		if glyph.value != "" {
			set++
		}
	}

	if set > 0 {
		for _, glyph := range glyphs {
			fieldPath := path + "." + glyph.name

			err := validateGlyphField(fieldPath, glyph.value)
			if err != nil {
				return err
			}

			if glyph.value == "" {
				return errors.Errorf("%s: tree glyph must not be empty (set all four tree glyphs or none)", fieldPath)
			}

			width := style.CellWidth([]byte(glyph.value))
			if width != 1 {
				return errors.Errorf("%s: tree glyph must be exactly one cell wide, got %d", fieldPath, width)
			}
		}
	}

	return validateThemeValue(path+".enumerator", reflect.ValueOf(t.Enumerator))
}
