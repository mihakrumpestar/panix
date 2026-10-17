package overlay

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

// composeCase describes one composition expectation. A nil region composes
// over the whole frame with Compose, otherwise over the region with
// ComposeRegion.
type composeCase struct {
	name      string
	frameRows []string
	region    *Rect
	boxLines  []string
	want      []string
	wantPlace Placement
}

func testFrame(rows ...string) *buffer.LinesBufDiff {
	frame := buffer.NewLinesBufDiff()

	for _, row := range rows {
		frame.WriteString(row)
	}

	return frame
}

func testBox(lines ...string) *Box {
	return NewBox(toBytes(lines), WithStyle(style.NewStyle()))
}

func frameStrings(frame Frame) []string {
	out := make([]string, frame.Len())

	for i := range frame.Len() {
		out[i] = string(frame.Line(i))
	}

	return out
}

func runComposeCases(t *testing.T, cases []composeCase) {
	t.Helper()

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			frame := testFrame(tt.frameRows...)
			box := testBox(tt.boxLines...)

			var place Placement

			if tt.region == nil {
				place = Compose(frame, box)
			} else {
				place = ComposeRegion(frame, box, *tt.region)
			}

			assert.Equal(t, tt.wantPlace, place, "placement")
			assert.Equal(t, tt.want, frameStrings(frame), "frame rows")
		})
	}
}

func TestCompose(t *testing.T) {
	t.Parallel()

	runComposeCases(t, []composeCase{
		{
			name:      "centering even frame even box",
			frameRows: slices.Repeat([]string{"ffffffffff"}, 8),
			boxLines:  []string{"bbbbbb", "bbbbbb", "bbbbbb", "bbbbbb"},
			want: []string{
				"ffffffffff", "ffffffffff",
				"ffbbbbbbff", "ffbbbbbbff", "ffbbbbbbff", "ffbbbbbbff",
				"ffffffffff", "ffffffffff",
			},
			wantPlace: Placement{Row: 2, Col: 2, Height: 4, Width: 6},
		},
		{
			name:      "centering odd frame odd box",
			frameRows: slices.Repeat([]string{"fffffffff"}, 7),
			boxLines:  []string{"bbbbb", "bbbbb", "bbbbb"},
			want: []string{
				"fffffffff", "fffffffff",
				"ffbbbbbff", "ffbbbbbff", "ffbbbbbff",
				"fffffffff", "fffffffff",
			},
			wantPlace: Placement{Row: 2, Col: 2, Height: 3, Width: 5},
		},
		{
			name:      "centering even frame odd box floor bias",
			frameRows: slices.Repeat([]string{"ffffffffff"}, 8),
			boxLines:  []string{"bbbbb", "bbbbb", "bbbbb"},
			want: []string{
				"ffffffffff", "ffffffffff",
				"ffbbbbbfff", "ffbbbbbfff", "ffbbbbbfff",
				"ffffffffff", "ffffffffff", "ffffffffff",
			},
			wantPlace: Placement{Row: 2, Col: 2, Height: 3, Width: 5},
		},
		{
			name:      "centering odd frame even box floor bias",
			frameRows: slices.Repeat([]string{"fffffffff"}, 7),
			boxLines:  []string{"bbbb", "bbbb"},
			want: []string{
				"fffffffff", "fffffffff",
				"ffbbbbfff", "ffbbbbfff",
				"fffffffff", "fffffffff", "fffffffff",
			},
			wantPlace: Placement{Row: 2, Col: 2, Height: 2, Width: 4},
		},
		{
			name:      "styled frame keeps its style around the box",
			frameRows: slices.Repeat([]string{"\x1b[31mffffffff\x1b[m"}, 3),
			boxLines:  []string{"bb"},
			want: []string{
				"\x1b[31mffffffff\x1b[m",
				"\x1b[31mfff\x1b[m" + "bb" + "\x1b[m\x1b[31mfff\x1b[m",
				"\x1b[31mffffffff\x1b[m",
			},
			wantPlace: Placement{Row: 1, Col: 3, Height: 1, Width: 2},
		},
	})
}

func TestComposeClamping(t *testing.T) {
	t.Parallel()

	runComposeCases(t, []composeCase{
		{
			name:      "box taller than frame is clipped",
			frameRows: []string{"ffff", "ffff"},
			boxLines:  []string{"bbbb", "cccc", "dddd"},
			want:      []string{"bbbb", "cccc"},
			wantPlace: Placement{Row: 0, Col: 0, Height: 2, Width: 4},
		},
		{
			name:      "box wider than frame is clipped",
			frameRows: []string{"fff", "fff", "fff"},
			boxLines:  []string{"bbbbb"},
			want:      []string{"fff", "bbb", "fff"},
			wantPlace: Placement{Row: 1, Col: 0, Height: 1, Width: 3},
		},
		{
			name:      "box filling the frame exactly",
			frameRows: []string{"ffff", "ffff"},
			boxLines:  []string{"bbbb", "bbbb"},
			want:      []string{"bbbb", "bbbb"},
			wantPlace: Placement{Row: 0, Col: 0, Height: 2, Width: 4},
		},
	})
}

func TestComposeNoBleedOutsideBoxRows(t *testing.T) {
	t.Parallel()

	frame := testFrame("row0", "row1", "row2", "row3", "row4", "row5")

	place := Compose(frame, testBox("bb", "cc"))

	assert.Equal(t, Placement{Row: 2, Col: 1, Height: 2, Width: 2}, place)
	assert.Equal(t, "row0", string(frame.Line(0)), "row above the box must be untouched")
	assert.Equal(t, "row1", string(frame.Line(1)), "row above the box must be untouched")
	assert.Equal(t, "rbb2", string(frame.Line(2)), "first box row spliced, row sides preserved")
	assert.Equal(t, "rcc3", string(frame.Line(3)), "second box row spliced, row sides preserved")
	assert.Equal(t, "row4", string(frame.Line(4)), "row below the box must be untouched")
	assert.Equal(t, "row5", string(frame.Line(5)), "row below the box must be untouched")
}

func TestComposePreservesContentAroundBox(t *testing.T) {
	t.Parallel()

	frame := testFrame("0123456789abcdef")

	place := Compose(frame, testBox("xyz"))

	assert.Equal(t, Placement{Row: 0, Col: 6, Height: 1, Width: 3}, place)
	assert.Equal(t, "012345xyz9abcdef", string(frame.Line(0)),
		"content left and right of the box must survive composition")
}

func TestComposeTwoBoxes(t *testing.T) {
	t.Parallel()

	frame := testFrame(slices.Repeat([]string{"ffffffff"}, 8)...)

	placeA := Compose(frame, testBox("aaaaaa", "aaaaaa", "aaaaaa", "aaaaaa"))
	placeB := Compose(frame, testBox("bbbb", "bbbb"))

	assert.Equal(t, Placement{Row: 2, Col: 1, Height: 4, Width: 6}, placeA)
	assert.Equal(t, Placement{Row: 3, Col: 2, Height: 2, Width: 4}, placeB)
	assert.Equal(t, []string{
		"ffffffff", "ffffffff",
		"faaaaaaf",
		"fabbbbaf", "fabbbbaf",
		"faaaaaaf",
		"ffffffff", "ffffffff",
	}, frameStrings(frame), "second box splices over the first within its own rows")
}

func TestComposeIdempotent(t *testing.T) {
	t.Parallel()

	frame := testFrame(slices.Repeat([]string{"ffff"}, 6)...)
	box := testBox("bb", "cc")

	first := Compose(frame, box)
	afterFirst := frameStrings(frame)

	second := Compose(frame, box)

	assert.Equal(t, first, second, "placement must be stable")
	assert.Equal(t, afterFirst, frameStrings(frame), "composing the same box twice must not change the frame")

	fresh := testFrame(slices.Repeat([]string{"ffff"}, 6)...)
	Compose(fresh, box)

	assert.Equal(t, afterFirst, frameStrings(fresh), "composition must be deterministic")
}

func TestComposeRegion(t *testing.T) {
	t.Parallel()

	runComposeCases(t, []composeCase{
		{
			name:      "centered within region",
			frameRows: slices.Repeat([]string{"ffffffffffff"}, 6),
			region:    &Rect{Row: 1, Col: 2, Height: 4, Width: 8},
			boxLines:  []string{"bbbb", "bbbb", "bbbb"},
			want: []string{
				"ffffffffffff",
				"ffffbbbbffff", "ffffbbbbffff", "ffffbbbbffff",
				"ffffffffffff", "ffffffffffff",
			},
			wantPlace: Placement{Row: 1, Col: 4, Height: 3, Width: 4},
		},
		{
			name:      "region clamped to frame bottom",
			frameRows: slices.Repeat([]string{"ffffffffffff"}, 6),
			region:    &Rect{Row: 5, Col: 0, Height: 4, Width: 12},
			boxLines:  []string{"bbbb"},
			want: []string{
				"ffffffffffff", "ffffffffffff", "ffffffffffff",
				"ffffffffffff", "ffffffffffff",
				"ffffbbbbffff",
			},
			wantPlace: Placement{Row: 5, Col: 4, Height: 1, Width: 4},
		},
		{
			name:      "negative region origin is clamped",
			frameRows: slices.Repeat([]string{"ffffffffffff"}, 6),
			region:    &Rect{Row: -3, Col: -2, Height: 5, Width: 10},
			boxLines:  []string{"bb"},
			want: []string{
				"ffffffffffff", "ffffffffffff",
				"ffffbbffffff",
				"ffffffffffff", "ffffffffffff", "ffffffffffff",
			},
			wantPlace: Placement{Row: 2, Col: 4, Height: 1, Width: 2},
		},
	})
}

func TestComposeRegionClamping(t *testing.T) {
	t.Parallel()

	runComposeCases(t, []composeCase{
		{
			name:      "box larger than region is clipped",
			frameRows: slices.Repeat([]string{"ffffff"}, 4),
			region:    &Rect{Row: 0, Col: 0, Height: 2, Width: 3},
			boxLines:  []string{"bbbbb", "ccccc", "ddddd"},
			want:      []string{"bbbfff", "cccfff", "ffffff", "ffffff"},
			wantPlace: Placement{Row: 0, Col: 0, Height: 2, Width: 3},
		},
		{
			name:      "empty region is a no-op",
			frameRows: []string{"ffff"},
			region:    &Rect{},
			boxLines:  []string{"bb"},
			want:      []string{"ffff"},
			wantPlace: Placement{},
		},
	})
}

func TestRectCenter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rect    Rect
		boxW    int
		boxH    int
		wantRow int
		wantCol int
	}{
		{
			name: "even gaps",
			rect: Rect{Height: 8, Width: 10},
			boxW: 6, boxH: 4,
			wantRow: 2, wantCol: 2,
		},
		{
			name: "odd gaps floor bias",
			rect: Rect{Height: 7, Width: 7},
			boxW: 2, boxH: 2,
			wantRow: 2, wantCol: 2,
		},
		{
			name: "offset region",
			rect: Rect{Row: 2, Col: 3, Height: 5, Width: 5},
			boxW: 2, boxH: 2,
			wantRow: 3, wantCol: 4,
		},
		{
			name: "box larger than region anchors top left",
			rect: Rect{Row: 1, Col: 1, Height: 3, Width: 3},
			boxW: 4, boxH: 4,
			wantRow: 1, wantCol: 1,
		},
		{
			name: "negative region origin is clamped",
			rect: Rect{Row: -2, Col: -2, Height: 4, Width: 4},
			boxW: 2, boxH: 2,
			wantRow: 1, wantCol: 1,
		},
		{
			name: "exact fit",
			rect: Rect{Row: 4, Col: 5, Height: 3, Width: 3},
			boxW: 3, boxH: 3,
			wantRow: 4, wantCol: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			row, col := tt.rect.Center(tt.boxW, tt.boxH)

			assert.Equal(t, tt.wantRow, row, "row")
			assert.Equal(t, tt.wantCol, col, "col")
		})
	}
}
