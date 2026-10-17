package theme

import (
	"fmt"
	"reflect"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

// GlyphsOf returns every glyph string a theme ships, keyed by the dotted path of
// the field that holds it (for example "Chars.PhaseArrow" or
// "Spinner.Frames[2]"). It collects the glyph-tagged string leaves by reflection,
// then expands the border regions and spinner frames, so the list cannot drift
// from the theme type. A nil theme yields an empty map.
func GlyphsOf(theme *Theme) map[string]string {
	glyphs := map[string]string{}
	if theme == nil {
		return glyphs
	}

	collectGlyphs(reflect.ValueOf(theme), "", glyphs)

	addBorderGlyphs(glyphs, "Border.Table", theme.Border.TableGlyphs)
	addBorderGlyphs(glyphs, "Border.Overlay", theme.Border.OverlayGlyphs)
	addBorderGlyphs(glyphs, "Border.Viewport", theme.Border.ViewportGlyphs)
	addBorderGlyphs(glyphs, "Border.Notification", theme.Border.NotificationGlyphs)

	for index, frame := range theme.Spinner.Frames {
		if frame != "" {
			glyphs[fmt.Sprintf("Spinner.Frames[%d]", index)] = frame
		}
	}

	return glyphs
}

// collectGlyphs walks value, recording every non-empty glyph-tagged string
// leaf under path.
func collectGlyphs(value reflect.Value, path string, out map[string]string) {
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		if value.IsNil() {
			return
		}

		collectGlyphs(value.Elem(), path, out)
	case reflect.Struct:
		collectStructGlyphs(value, path, out)
	default:
	}
}

// collectStructGlyphs recurses into the exported fields of a struct value,
// recording glyph-tagged string leaves and descending into everything else.
func collectStructGlyphs(value reflect.Value, path string, out map[string]string) {
	typ := value.Type()

	for index := range typ.NumField() {
		field := typ.Field(index)
		if field.PkgPath != "" {
			continue // unexported: not part of the theme surface
		}

		if isDerived(field) {
			continue // derived render field: not a serializable glyph
		}

		child := field.Name
		if path != "" {
			child = path + "." + field.Name
		}

		if _, tagged := field.Tag.Lookup("glyph"); tagged && value.Field(index).Kind() == reflect.String {
			if glyph := value.Field(index).String(); glyph != "" {
				out[child] = glyph
			}

			continue
		}

		collectGlyphs(value.Field(index), child, out)
	}
}

// addBorderGlyphs records the resolved glyph bytes of one border region.
func addBorderGlyphs(out map[string]string, prefix string, border style.Border) {
	out[prefix+".TopLeft"] = string(border.TopLeft)
	out[prefix+".TopRight"] = string(border.TopRight)
	out[prefix+".BottomLeft"] = string(border.BottomLeft)
	out[prefix+".BottomRight"] = string(border.BottomRight)
	out[prefix+".Horizontal"] = string(border.Horizontal)
	out[prefix+".Vertical"] = string(border.Vertical)
	out[prefix+".TopMid"] = string(border.TopMid)
	out[prefix+".BottomMid"] = string(border.BottomMid)
	out[prefix+".LeftMid"] = string(border.LeftMid)
	out[prefix+".RightMid"] = string(border.RightMid)
	out[prefix+".MidMid"] = string(border.MidMid)
}
