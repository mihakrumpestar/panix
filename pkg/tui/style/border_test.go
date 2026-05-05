package style

import (
	"testing"
)

func TestNormalBorder(t *testing.T) {
	t.Parallel()

	b := NormalBorder()

	if b.TopLeft != "┌" || b.TopRight != "┐" {
		t.Errorf("NormalBorder corners = (%q, %q), want (┌, ┐)", b.TopLeft, b.TopRight)
	}

	if b.BottomLeft != "└" || b.BottomRight != "┘" {
		t.Errorf("NormalBorder bottom corners = (%q, %q), want (└, ┘)", b.BottomLeft, b.BottomRight)
	}

	if b.Horizontal != "─" || b.Vertical != "│" {
		t.Errorf("NormalBorder lines = (%q, %q), want (─, │)", b.Horizontal, b.Vertical)
	}

	if b.TopMid != "┬" || b.BottomMid != "┴" || b.LeftMid != "├" || b.RightMid != "┤" {
		t.Errorf("NormalBorder mids = (%q, %q, %q, %q), want (┬, ┴, ├, ┤)",
			b.TopMid, b.BottomMid, b.LeftMid, b.RightMid)
	}

	if b.MidMid != "┼" {
		t.Errorf("NormalBorder MidMid = %q, want ┼", b.MidMid)
	}
}

func TestRoundedBorder(t *testing.T) {
	t.Parallel()

	b := RoundedBorder()

	if b.TopLeft != "╭" || b.TopRight != "╮" {
		t.Errorf("RoundedBorder top corners = (%q, %q), want (╭, ╮)", b.TopLeft, b.TopRight)
	}

	if b.BottomLeft != "╰" || b.BottomRight != "╯" {
		t.Errorf("RoundedBorder bottom corners = (%q, %q), want (╰, ╯)", b.BottomLeft, b.BottomRight)
	}

	if b.Horizontal != "─" || b.Vertical != "│" {
		t.Errorf("RoundedBorder lines = (%q, %q), want (─, │)", b.Horizontal, b.Vertical)
	}
}

func TestHiddenBorder(t *testing.T) {
	t.Parallel()

	b := HiddenBorder()

	if b.TopLeft != "" || b.Horizontal != "" || b.Vertical != "" {
		t.Errorf("HiddenBorder should have all empty strings, got TopLeft=%q Horizontal=%q Vertical=%q",
			b.TopLeft, b.Horizontal, b.Vertical)
	}
}

func TestMarkdownBorder(t *testing.T) {
	t.Parallel()

	b := MarkdownBorder()

	if b.TopLeft != "|" || b.TopRight != "|" || b.BottomLeft != "|" || b.BottomRight != "|" {
		t.Errorf("MarkdownBorder corners should all be |")
	}

	if b.Horizontal != "-" || b.Vertical != "|" {
		t.Errorf("MarkdownBorder lines = (%q, %q), want (-, |)", b.Horizontal, b.Vertical)
	}
}

func TestBorder_NoPerSideColorByDefault(t *testing.T) {
	t.Parallel()

	b := NormalBorder()

	if b.topFg != "" || b.rightFg != "" || b.bottomFg != "" || b.leftFg != "" {
		t.Error("New border should have empty per-side color prefixes")
	}
}
