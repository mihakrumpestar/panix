package style

import (
	"testing"
)

//nolint:cyclop
func TestNormalBorder(t *testing.T) {
	t.Parallel()

	brd :=NormalBorder()

	if brd.TopLeft != "┌" || brd.TopRight != "┐" {
		t.Errorf("NormalBorder corners = (%q, %q), want (┌, ┐)", brd.TopLeft, brd.TopRight)
	}

	if brd.BottomLeft != "└" || brd.BottomRight != "┘" {
		t.Errorf("NormalBorder bottom corners = (%q, %q), want (└, ┘)", brd.BottomLeft, brd.BottomRight)
	}

	if brd.Horizontal != "─" || brd.Vertical != "│" {
		t.Errorf("NormalBorder lines = (%q, %q), want (─, │)", brd.Horizontal, brd.Vertical)
	}

	if brd.TopMid != "┬" || brd.BottomMid != "┴" || brd.LeftMid != "├" || brd.RightMid != "┤" {
		t.Errorf("NormalBorder mids = (%q, %q, %q, %q), want (┬, ┴, ├, ┤)",
			brd.TopMid, brd.BottomMid, brd.LeftMid, brd.RightMid)
	}

	if brd.MidMid != "┼" {
		t.Errorf("NormalBorder MidMid = %q, want ┼", brd.MidMid)
	}
}

func TestRoundedBorder(t *testing.T) {
	t.Parallel()

	brd :=RoundedBorder()

	if brd.TopLeft != "╭" || brd.TopRight != "╮" {
		t.Errorf("RoundedBorder top corners = (%q, %q), want (╭, ╮)", brd.TopLeft, brd.TopRight)
	}

	if brd.BottomLeft != "╰" || brd.BottomRight != "╯" {
		t.Errorf("RoundedBorder bottom corners = (%q, %q), want (╰, ╯)", brd.BottomLeft, brd.BottomRight)
	}

	if brd.Horizontal != "─" || brd.Vertical != "│" {
		t.Errorf("RoundedBorder lines = (%q, %q), want (─, │)", brd.Horizontal, brd.Vertical)
	}
}

func TestHiddenBorder(t *testing.T) {
	t.Parallel()

	brd :=HiddenBorder()

	if brd.TopLeft != "" || brd.Horizontal != "" || brd.Vertical != "" {
		t.Errorf("HiddenBorder should have all empty strings, got TopLeft=%q Horizontal=%q Vertical=%q",
			brd.TopLeft, brd.Horizontal, brd.Vertical)
	}
}

func TestMarkdownBorder(t *testing.T) {
	t.Parallel()

	brd :=MarkdownBorder()

	if brd.TopLeft != "|" || brd.TopRight != "|" || brd.BottomLeft != "|" || brd.BottomRight != "|" {
		t.Errorf("MarkdownBorder corners should all be |")
	}

	if brd.Horizontal != "-" || brd.Vertical != "|" {
		t.Errorf("MarkdownBorder lines = (%q, %q), want (-, |)", brd.Horizontal, brd.Vertical)
	}
}

func TestBorder_NoPerSideColorByDefault(t *testing.T) {
	t.Parallel()

	brd :=NormalBorder()

	if brd.topFg != "" || brd.rightFg != "" || brd.bottomFg != "" || brd.leftFg != "" {
		t.Error("New border should have empty per-side color prefixes")
	}
}
