package notification

import (
	"strings"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/stretchr/testify/assert"
)

// testStyles returns distinct per-level colors; Warning carries the marker and
// row-text contrast pattern.
func testStyles() Styles {
	return Styles{
		DefaultFg: style.Color("#B4B4B4"),
		Base:      style.NewStyle().Bold(true),
		Border:    style.RoundedBorder(),
		Info:      LevelStyle{FrameColor: style.Color("#B4B4B4")},
		Success:   LevelStyle{FrameColor: style.Color("#50FA7B")},
		Warning: LevelStyle{
			Marker:      "! ",
			MarkerStyle: style.NewStyle().Foreground(style.Color("#FFB86C")),
			TextStyle:   style.NewStyle().Foreground(style.Color("#F8F8F2")),
			FrameColor:  style.Color("#FFB86C"),
		},
		Error: LevelStyle{FrameColor: style.Color("#FF5555")},
	}
}

func viewString(n *Notification) string {
	v := n.Render()
	if v == nil {
		return ""
	}

	return v.String()
}

func TestViewRendersBorderedBox(t *testing.T) {
	t.Parallel()

	n := New(testStyles())
	_ = n.Set(LevelSuccess, "hello")

	result := viewString(n)
	assert.NotEmpty(t, result, "expected non-empty output")

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	assert.GreaterOrEqual(t, len(lines), 3, "expected at least 3 lines")

	top := lines[0]
	content := lines[1]
	bottom := lines[2]

	assert.Contains(t, top, "╭", "top border missing ╭")
	assert.Contains(t, top, "╮", "top border missing ╮")
	assert.Contains(t, top, "─", "top border missing horizontal fill")
	assert.Contains(t, content, "│", "content line missing vertical borders")
	assert.Contains(t, bottom, "╰", "bottom border missing ╰")
	assert.Contains(t, bottom, "╯", "bottom border missing ╯")
	assert.Contains(t, bottom, "─", "bottom border missing horizontal fill")
}

func TestViewEmptyWhenExpired(t *testing.T) {
	t.Parallel()

	n := New(testStyles())

	assert.Empty(t, viewString(n), "expected empty output for expired notification")
}

// TestUpdateEmptyNotificationIsNoop proves an empty notification never ticks.
func TestUpdateEmptyNotificationIsNoop(t *testing.T) {
	t.Parallel()

	notif := New(testStyles())

	assert.Nil(t, notif.Update(notificationTickMsg{}), "an empty notification must not tick")
}

func TestViewBorderWidthMatchesContent(t *testing.T) {
	t.Parallel()

	n := New(testStyles())
	_ = n.Set(LevelSuccess, "short")

	result := viewString(n)
	topWidth := style.CellWidth([]byte(strings.Split(strings.TrimRight(result, "\n"), "\n")[0]))

	n2 := New(testStyles())
	_ = n2.Set(LevelSuccess, "a much longer message text")

	result2 := viewString(n2)
	topWidth2 := style.CellWidth([]byte(strings.Split(strings.TrimRight(result2, "\n"), "\n")[0]))

	assert.Greater(t, topWidth2, topWidth, "longer content should produce wider border")
}

func TestViewAllLinesEqualWidth(t *testing.T) {
	t.Parallel()

	n := New(testStyles())
	_ = n.Set(LevelSuccess, "Here is a notification")

	result := viewString(n)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")

	widths := make([]int, len(lines))
	for i, line := range lines {
		widths[i] = style.CellWidth([]byte(line))
	}

	for i := 1; i < len(widths); i++ {
		assert.Equal(t, widths[0], widths[i], "lines have mismatched widths: %v", widths)
	}
}

func TestViewContentHasHorizontalPadding(t *testing.T) {
	t.Parallel()

	n := New(testStyles())
	_ = n.Set(LevelSuccess, "hello")

	result := viewString(n)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	content := lines[1]

	clean := string(style.StripANSI([]byte(content)))
	assert.True(t, strings.HasPrefix(clean, "│ "), "content line should have padding: %q", clean)
	assert.True(t, strings.HasSuffix(clean, " │"), "content line should have padding: %q", clean)
}

func TestSetPersistent_NeverExpiresAndDoesNotKeepTicking(t *testing.T) {
	t.Parallel()

	notif := New(testStyles())
	_ = notif.SetPersistent(LevelSuccess, "keep me")

	assert.NotEmpty(t, viewString(notif), "persistent notification should render")

	// Force the start time far in the past: a persistent notification must
	// still be visible and unfaded.
	notif.started = time.Now().Add(-time.Hour)

	assert.NotEmpty(t, viewString(notif), "persistent notification should not expire")
	assert.Contains(t, viewString(notif), "keep me", "persistent notification content should stay visible")

	// After the initial tick no further ticking is needed.
	assert.Nil(t, notif.Update(notificationTickMsg{}))
}

// TestMarkerColumnWidth_DerivedFromWidestMarker pins that the marker column is
// sized from the widest marker so levels align and no marker is truncated.
func TestMarkerColumnWidth_DerivedFromWidestMarker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		info    string
		warning string
		want    int
	}{
		{name: "one cell marker set", info: "!", warning: "!", want: 1},
		{name: "mixed markers align to widest", info: "!", warning: "!!!", want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			styles := testStyles()
			styles.Info.Marker = tt.info
			styles.Info.MarkerStyle = style.NewStyle()
			styles.Warning.Marker = tt.warning

			notif := New(styles)
			assert.Equal(t, tt.want, notif.markerColumnWidth,
				"the marker column must be the widest level marker")

			_ = notif.Set(LevelWarning, "message")

			assert.Contains(t, viewString(notif), tt.warning,
				"the marker must render without truncation")
		})
	}
}

// TestWarningRendersMarkerAndReadableText guards the warning contrast contract:
// the marker carries the warning accent while the message uses the level's row
// text, readable on both dark and light themes.
func TestWarningRendersMarkerAndReadableText(t *testing.T) {
	t.Parallel()

	styles := testStyles()
	notif := New(styles)
	_ = notif.Set(LevelWarning, "warning message")

	out := viewString(notif)

	marker := string(styles.Warning.MarkerStyle.Width(notif.markerColumnWidth).RenderLine([]byte(styles.Warning.Marker)))
	message := string(layerTextStyle(styles.Base.Foreground(styles.Warning.FrameColor), styles.Warning.TextStyle).
		RenderLine([]byte("warning message")))

	assert.Contains(t, out, marker, "the marker must carry the warning color")
	assert.Contains(t, out, message, "the message must use readable row text")
	assert.NotContains(t, out, string(styles.Warning.MarkerStyle.RenderLine([]byte("warning message"))),
		"the message must not be warning colored: too low contrast on light themes")
}

// TestSuccessKeepsSingleColorLook pins the single-color look for a zero
// TextStyle (no marker, no text override).
func TestSuccessKeepsSingleColorLook(t *testing.T) {
	t.Parallel()

	styles := testStyles()
	notif := New(styles)
	_ = notif.Set(LevelSuccess, "saved")

	out := viewString(notif)

	message := string(styles.Base.Foreground(styles.Success.FrameColor).RenderLine([]byte("saved")))
	assert.Contains(t, out, message, "the message must be the frame-colored base text")
	assert.NotContains(t, out, "! ", "a level without a marker must not indent the message")
}

// TestTransientNotificationFades pins fading of transients only.
func TestTransientNotificationFades(t *testing.T) {
	t.Parallel()

	notif := New(testStyles())
	_ = notif.Set(LevelSuccess, "fading")

	full := viewString(notif)

	// Jump past the fade start: the frame color must dim.
	notif.started = time.Now().Add(-(defaultFadeStart + defaultDuration) / 2)
	notif.render()

	assert.NotEqual(t, full, viewString(notif), "a transient notification must fade over time")

	persistent := New(testStyles())
	_ = persistent.SetPersistent(LevelSuccess, "steady")

	before := viewString(persistent)
	persistent.started = time.Now().Add(-time.Hour)
	persistent.render()

	assert.Equal(t, before, viewString(persistent), "a persistent notification must not fade")
}

// TestApplyThemeRerendersActiveNotification pins immediate recoloring.
func TestApplyThemeRerendersActiveNotification(t *testing.T) {
	t.Parallel()

	notif := New(testStyles())
	_ = notif.SetPersistent(LevelSuccess, "saved")

	before := viewString(notif)

	updated := testStyles()
	updated.Success = LevelStyle{FrameColor: style.Color("#FF0000")}
	notif.ApplyTheme(updated)

	after := viewString(notif)
	assert.NotEqual(t, before, after, "ApplyTheme must recolor the active notification")
	assert.Contains(t, after, string(updated.Base.Foreground(updated.Success.FrameColor).RenderLine([]byte("saved"))))
}

// TestNewDefaultsRoundedBorder pins the rounded preset fallback for a zero Border.
func TestNewDefaultsRoundedBorder(t *testing.T) {
	t.Parallel()

	notif := New(Styles{DefaultFg: style.Color("#B4B4B4")})
	_ = notif.SetPersistent(LevelInfo, "framed")

	assert.Contains(t, viewString(notif), "╭", "a zero border must fall back to the rounded preset")
}

// TestLevelsRenderDistinctFrames pins per-level frame colors and the Info
// fallback.
func TestLevelsRenderDistinctFrames(t *testing.T) {
	t.Parallel()

	styles := testStyles()
	unknown := SeverityLevel("other")

	frames := map[SeverityLevel]string{}

	for _, level := range []SeverityLevel{LevelInfo, LevelSuccess, LevelWarning, LevelError, unknown} {
		notif := New(styles)
		_ = notif.SetPersistent(level, "message")
		frames[level] = viewString(notif)
	}

	assert.NotEqual(t, frames[LevelInfo], frames[LevelSuccess], "info and success must differ")
	assert.NotEqual(t, frames[LevelSuccess], frames[LevelError], "success and error must differ")
	assert.NotEqual(t, frames[LevelError], frames[LevelWarning], "error and warning must differ")
	assert.Equal(t, frames[LevelInfo], frames[unknown], "an unknown level must fall back to Info")
}

// TestEmptyFrameColorFallsBackToDefault pins the DefaultFg fallback.
func TestEmptyFrameColorFallsBackToDefault(t *testing.T) {
	t.Parallel()

	styles := testStyles()
	styles.Success = LevelStyle{}

	notif := New(styles)
	_ = notif.SetPersistent(LevelSuccess, "fallback")

	assert.Contains(t, viewString(notif),
		string(styles.Base.Foreground(styles.DefaultFg).RenderLine([]byte("fallback"))),
		"an empty frame color must fall back to DefaultFg")
}

// TestTextStyleBackgroundOverride pins background layering from TextStyle.
func TestTextStyleBackgroundOverride(t *testing.T) {
	t.Parallel()

	styles := testStyles()
	styles.Success = LevelStyle{
		FrameColor: style.Color("#50FA7B"),
		TextStyle:  style.NewStyle().Background(style.Color("#010203")),
	}

	notif := New(styles)
	_ = notif.SetPersistent(LevelSuccess, "bg")

	assert.Contains(t, viewString(notif),
		string(layerTextStyle(styles.Base.Foreground(styles.Success.FrameColor), styles.Success.TextStyle).
			RenderLine([]byte("bg"))),
		"the TextStyle background must override the base")
}

// TestTransientNotificationTicksAndExpires pins the tick lifecycle: ticks
// reschedule until the duration passes, then the next tick clears.
func TestTransientNotificationTicksAndExpires(t *testing.T) {
	t.Parallel()

	notif := New(testStyles())
	_ = notif.Set(LevelSuccess, "ticking")

	assert.NotZero(t, notif.Version(), "a rendered notification carries a version")
	assert.NotNil(t, notif.Update(notificationTickMsg{}), "a live transient must schedule the next tick")

	// Force the start time past the duration: the next tick must clear it.
	notif.started = time.Now().Add(-2 * defaultDuration)

	assert.Nil(t, notif.Update(notificationTickMsg{}), "an expired transient must not schedule a tick")
	assert.Empty(t, viewString(notif), "an expired notification must clear")
}
