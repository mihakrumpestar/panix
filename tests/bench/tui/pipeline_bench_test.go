// Full-pipeline TUI benchmarks comparing zeroterm, Bubbletea, and cview.
//
// Each benchmark measures the steady-state per-frame cost of rendering a
// composite TUI layout through the complete production pipeline:
//
//	Zeroterm:  Render(into LinesBufDiff) → Diff → RenderLines → terminal bytes
//	Bubbletea: View() → ultraviolet StyledString → ScreenBuffer → TerminalRenderer → Flush
//	Cview:     widget state update → Draw(tcell.Screen) → Show() → VT emulator write
//
// Layout (top to bottom within a 200×50 terminal):
//
//	Large viewport  (80-100 ANSI content lines, 15 visible)
//	Styled separator
//	Small viewport  (20 ANSI content lines, 6 visible)
//	Styled separator
//	Tree  (depth 3, breadth 4)
//	Styled separator
//	Table  (4 cols × 10 rows, styled headers)
//
// Three update scenarios:
//   - NoChange:          no updates between frames (cache/diff hit)
//   - EveryFrameUpdate:  every frame re-randomizes all viewport content
//     (80-100 lines, random line count and content via
//     seeded math/rand) + toggles one table cell status
//   - QuarterFrameUpdate: every 4th frame appends 3 viewport lines (until
//     the viewport reaches 100 lines, then stops) +
//     toggles one table cell status
//

package main

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"

	bubbles "charm.land/bubbles/v2/viewport"
	lipgloss "charm.land/lipgloss/v2"
	lipglosstable "charm.land/lipgloss/v2/table"
	lipglosstree "charm.land/lipgloss/v2/tree"
	"codeberg.org/tslocum/cview"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/gdamore/tcell/v3/vt"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/table"
	"github.com/mihakrumpestar/panix/pkg/tui/tree"
	"github.com/mihakrumpestar/panix/pkg/tui/viewport"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

// ────────────────────────────────────────────────────────────────────────────
// Constants
// ────────────────────────────────────────────────────────────────────────────

const (
	benchWidth  = 200
	benchHeight = 50

	largeVPContentLines = 80
	largeVPVisibleLines = 15
	smallVPContentLines = 20
	smallVPVisibleLines = 6

	treeDepth   = 3
	treeBreadth = 4
	tableRows   = 10
	tableWidth  = 80

	separatorText   = "─── output ───"
	maxLargeVPLines = 100
	linesPerUpdate  = 3
	benchRandSeed   = 42
	randomPoolSize  = 64 // pre-generated random line sets to cycle through
)

var (
	tcellColorCyan   = color.XTerm6
	tcellColorGreen  = color.XTerm2
	tcellColorYellow = color.XTerm11

	cviewColorNames = []string{"black", "red", "green", "yellow", "blue", "magenta"}

	// Pre-allocated byte slices to avoid string→[]byte conversion per frame.
	separatorTextBytes = []byte(separatorText)
	statusDoneBytes    = []byte("\x1b[32mdone\x1b[0m")
	statusRunningBytes = []byte("\x1b[33mrunning\x1b[0m")

	// Pre-allocated line parts for pushIntf (avoids string→[]byte per frame).
	linePrefix = []byte("\x1b[32mnew line ")
	lineSuffix = []byte("\x1b[0m appended content here")
)

// Pre-generated random line pools (lazy-init on first use).
// Same random seed ensures identical random values across all pools.
// Each pool is in the framework's native format for fair comparison.
var (
	randomZerotermPool  [][][]byte
	randomBubbleteaPool [][]string
	randomCviewPool     [][][]byte
	randomCviewJoined   [][]byte
	randomPoolOnce      sync.Once
)

func ensureRandomPools() {
	randomPoolOnce.Do(func() {
		rng := newBenchRNG()

		randomZerotermPool = make([][][]byte, randomPoolSize)
		randomBubbleteaPool = make([][]string, randomPoolSize)
		randomCviewPool = make([][][]byte, randomPoolSize)
		randomCviewJoined = make([][]byte, randomPoolSize)

		for idx := range randomPoolSize {
			ansiLines := makeRandomBenchANSILines(rng, largeVPContentLines)
			randomZerotermPool[idx] = ansiLines

			strs := make([]string, len(ansiLines))
			for j, b := range ansiLines {
				strs[j] = string(b)
			}

			randomBubbleteaPool[idx] = strs

			cviewLines := makeRandomCviewColorLines(rng, largeVPContentLines)
			randomCviewPool[idx] = cviewLines
			randomCviewJoined[idx] = joinLines(cviewLines)
		}
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Shared data generators
// ────────────────────────────────────────────────────────────────────────────

func makeBenchANSILines(lineCount int) [][]byte {
	lines := make([][]byte, lineCount)
	for idx := range lineCount {
		switch {
		case idx%10 == 0:
			lines[idx] = fmt.Appendf(nil, "\x1b[1;34msrc/pkg%-4d\x1b[0m \x1b[32mOK\x1b[0m package with a longer description that fills the line", idx)
		case idx%3 == 0:
			lines[idx] = fmt.Appendf(nil, "\x1b[3%dmline %d: colored text with escape sequences\x1b[0m and plain suffix to fill width", idx%6, idx)
		default:
			lines[idx] = fmt.Appendf(nil, "line %d: plain text with some content that is reasonably long for testing purposes here  ", idx)
		}
	}

	return lines
}

func makeBenchANSIContentString(lineCount int) string {
	var builder strings.Builder

	for idx := range lineCount {
		switch {
		case idx%10 == 0:
			fmt.Fprintf(&builder, "\x1b[1;34msrc/pkg%-4d\x1b[0m \x1b[32mOK\x1b[0m package with a longer description that fills the line", idx)
		case idx%3 == 0:
			fmt.Fprintf(&builder, "\x1b[3%dmline %d: colored text with escape sequences\x1b[0m and plain suffix to fill width", idx%6, idx)
		default:
			fmt.Fprintf(&builder, "line %d: plain text with some content that is reasonably long for testing purposes here  ", idx)
		}

		if idx < lineCount-1 {
			builder.WriteByte('\n')
		}
	}

	return builder.String()
}

func makeCviewColorLines(lineCount int) [][]byte {
	lines := make([][]byte, lineCount)
	for idx := range lineCount {
		switch {
		case idx%10 == 0:
			lines[idx] = fmt.Appendf(nil, "[blue::b]src/pkg%-4d[-] [green]OK[-] package with a longer description that fills the line", idx)
		case idx%3 == 0:
			lines[idx] = fmt.Appendf(nil, "[%s]line %d: colored text with escape sequences[-] and plain suffix to fill width", cviewColorNames[idx%6], idx)
		default:
			lines[idx] = fmt.Appendf(nil, "line %d: plain text with some content that is reasonably long for testing purposes here  ", idx)
		}
	}

	return lines
}

func makeBenchTableRows() [][][]byte {
	rows := make([][][]byte, tableRows)
	for idx := range tableRows {
		rows[idx] = [][]byte{
			fmt.Appendf(nil, "item-%d", idx),
			statusDoneBytes,
			[]byte("1.23s"),
			[]byte("x86_64"),
		}
	}

	return rows
}

func joinLines(lines [][]byte) []byte {
	if len(lines) == 0 {
		return nil
	}

	totalLen := 0
	for _, line := range lines {
		totalLen += len(line) + 1
	}

	result := make([]byte, 0, totalLen)
	for i, line := range lines {
		result = append(result, line...)
		if i < len(lines)-1 {
			result = append(result, '\n')
		}
	}

	return result
}

// ────────────────────────────────────────────────────────────────────────────
// Update modes
// ────────────────────────────────────────────────────────────────────────────

const (
	updateNone         = 0
	updateEveryFrame   = 1
	updateQuarterFrame = 2
)

// ────────────────────────────────────────────────────────────────────────────
// Random line generators (for EveryFrameUpdate: full content randomization)
// ────────────────────────────────────────────────────────────────────────────

func makeRandomBenchANSILines(rng *rand.Rand, lineCount int) [][]byte {
	lines := make([][]byte, lineCount)

	for idx := range lineCount {
		val := rng.IntN(10000)
		switch {
		case val%10 == 0:
			lines[idx] = fmt.Appendf(nil, "\x1b[1;34msrc/pkg%-4d\x1b[0m \x1b[32mOK\x1b[0m package with a longer description that fills the line", val)
		case val%3 == 0:
			lines[idx] = fmt.Appendf(nil, "\x1b[3%dmline %d: colored text with escape sequences\x1b[0m and plain suffix to fill width", val%6, val)
		default:
			lines[idx] = fmt.Appendf(nil, "line %d: plain text with some content that is reasonably long for testing purposes here  ", val)
		}
	}

	return lines
}

func makeRandomCviewColorLines(rng *rand.Rand, lineCount int) [][]byte {
	lines := make([][]byte, lineCount)

	for idx := range lineCount {
		val := rng.IntN(10000)
		switch {
		case val%10 == 0:
			lines[idx] = fmt.Appendf(nil, "[blue::b]src/pkg%-4d[-] [green]OK[-] package with a longer description that fills the line", val)
		case val%3 == 0:
			lines[idx] = fmt.Appendf(nil, "[%s]line %d: colored text with escape sequences[-] and plain suffix to fill width", cviewColorNames[val%6], val)
		default:
			lines[idx] = fmt.Appendf(nil, "line %d: plain text with some content that is reasonably long for testing purposes here  ", val)
		}
	}

	return lines
}

//nolint:gosec // G404: intentional seeded PRNG for deterministic benchmark results
func newBenchRNG() *rand.Rand {
	return rand.New(rand.NewPCG(benchRandSeed, benchRandSeed))
}

// ────────────────────────────────────────────────────────────────────────────
// Zeroterm (Ours) pipeline
// ────────────────────────────────────────────────────────────────────────────

type zerotermPipeline struct {
	largeVP  viewport.Viewport
	smallVP  viewport.Viewport
	treeRoot *tree.Node
	tbl      *table.Table
	sepStyle style.Style

	largeLines [][]byte
	smallLines [][]byte
	tblRows    [][][]byte

	randomPoolIdx int
}

func newZerotermPipeline() *zerotermPipeline {
	ensureRandomPools()

	sepStyle := style.NewStyle().Foreground(style.Color("#6272A4")).Bold(true)

	largeVP := viewport.New(viewport.WithWidth(benchWidth), viewport.WithHeight(largeVPVisibleLines))
	smallVP := viewport.New(viewport.WithWidth(benchWidth), viewport.WithHeight(smallVPVisibleLines))

	largeLines := makeBenchANSILines(largeVPContentLines)
	smallLines := makeBenchANSILines(smallVPContentLines)

	largeVP.SetContentLines(largeLines)
	largeVP.GotoBottom()
	smallVP.SetContentLines(smallLines)

	tbl := table.New(table.Config{
		Width:  tableWidth,
		Border: style.NormalBorder(),
		Headers: [][]byte{
			[]byte("Name"), []byte("Status"), []byte("Time"), []byte("Arch"),
		},
		ColumnStyles: []style.Style{
			style.NewStyle().Foreground(style.Color("#8BE9FD")),
			style.NewStyle().Foreground(style.Color("#50FA7B")),
			{},
			{},
		},
	})
	tbl.SetRows(makeBenchTableRows())

	return &zerotermPipeline{
		largeVP:    largeVP,
		smallVP:    smallVP,
		treeRoot:   buildZerotermTree(),
		tbl:        tbl,
		sepStyle:   sepStyle,
		largeLines: largeLines,
		smallLines: smallLines,
		tblRows:    makeBenchTableRows(),
	}
}

func buildZerotermTree() *tree.Node {
	treeStyle := style.NewStyle().Foreground(style.Color("#6272A4"))
	root := tree.NewTree(treeStyle, 3)

	var build func(depth int, parent *tree.Node, parentXp string) *tree.Node

	build = func(depth int, parent *tree.Node, parentXp string) *tree.Node {
		nodeXp := parentXp + "/node"

		child := parent.Child(xpath.New(nodeXp), 1, func(_ int) *buffer.LinesBuf {
			nodeBuf := buffer.NewLinesBuf()
			if depth%2 == 0 {
				nodeBuf.WriteString(fmt.Sprintf("\x1b[1;36mnode-d%d\x1b[0m", treeDepth-depth))
			} else {
				nodeBuf.WriteString(fmt.Sprintf("node-d%d", treeDepth-depth))
			}

			return nodeBuf
		})

		if depth > 0 {
			for i := range treeBreadth {
				build(depth-1, child, fmt.Sprintf("%s/%d", nodeXp, i))
			}
		}

		return child
	}

	for i := range treeBreadth {
		build(treeDepth-1, root, fmt.Sprintf("root/%d", i))
	}

	return root
}

func (pipe *zerotermPipeline) renderInto(buf *buffer.LinesBufDiff) {
	buf.AppendFrom(pipe.largeVP.Render())
	pipe.sepStyle.RenderLineInto(buf.LinesBuf, separatorTextBytes)

	buf.AppendFrom(pipe.smallVP.Render())
	pipe.sepStyle.RenderLineInto(buf.LinesBuf, separatorTextBytes)

	pipe.treeRoot.WriteRenderTo(buf.LinesBuf)
	pipe.sepStyle.RenderLineInto(buf.LinesBuf, separatorTextBytes)

	buf.AppendFrom(pipe.tbl.Render())
}

func (pipe *zerotermPipeline) randomizeLargeViewport() {
	pipe.largeLines = randomZerotermPool[pipe.randomPoolIdx%randomPoolSize]
	pipe.randomPoolIdx++
	pipe.largeVP.SetContentLines(pipe.largeLines)
	pipe.largeVP.GotoBottom()
}

func (pipe *zerotermPipeline) appendLargeViewport(frameCount int) {
	if len(pipe.largeLines) >= maxLargeVPLines {
		return
	}

	base := frameCount * 3

	for i := range linesPerUpdate {
		var line []byte

		line = append(line, linePrefix...)
		line = buffer.AppendInt(line, base+i)
		line = append(line, lineSuffix...)
		pipe.largeLines = append(pipe.largeLines, line)
	}

	pipe.largeVP.SetContentLines(pipe.largeLines)
	pipe.largeVP.GotoBottom()
}

func (pipe *zerotermPipeline) toggleTableStatus(frameCount int) {
	rowIdx := frameCount % tableRows

	if frameCount%2 == 0 {
		pipe.tblRows[rowIdx][1] = statusDoneBytes
	} else {
		pipe.tblRows[rowIdx][1] = statusRunningBytes
	}

	pipe.tbl.SetRows(pipe.tblRows)
}

func benchZerotermPipeline(b *testing.B, updateMode int) {
	b.Helper()
	b.ReportAllocs()

	pipe := newZerotermPipeline()

	frames := [2]*buffer.LinesBufDiff{
		buffer.NewLinesBufDiff(),
		buffer.NewLinesBufDiff(),
	}
	curFrame := 0

	// Pre-fill both buffers for steady-state start.
	for i := range 2 {
		frames[i].Reset()
		pipe.renderInto(frames[i])
	}

	outBuf := make([]byte, 0, 32768)
	frameCount := 0

	b.ResetTimer()

	for b.Loop() {
		switch updateMode {
		case updateNone:
		case updateEveryFrame:
			pipe.randomizeLargeViewport()
			pipe.toggleTableStatus(frameCount)
		case updateQuarterFrame:
			if frameCount%4 == 0 {
				pipe.appendLargeViewport(frameCount)
				pipe.toggleTableStatus(frameCount)
			}
		}

		cur := frames[curFrame]
		cur.Reset()
		pipe.renderInto(cur)

		prev := frames[1-curFrame]

		prevCount := prev.Len()
		diffs := cur.Diff(prev)

		if len(diffs) == 0 && cur.Len() >= prevCount {
			curFrame = 1 - curFrame
			frameCount++

			continue
		}

		outBuf = outBuf[:0]
		outBuf = zeroterm.RenderLines(outBuf, diffs, cur, prevCount, benchHeight)

		curFrame = 1 - curFrame
		frameCount++
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Bubbletea pipeline
// ────────────────────────────────────────────────────────────────────────────

type bubbleteaPipeline struct {
	largeVP         bubbles.Model
	smallVP         bubbles.Model
	tbl             *lipglosstable.Table
	largeLines      []string
	smallContent    string
	sepStyle        lipgloss.Style
	treeStr         string
	lastViewContent string
	randomPoolIdx   int
}

func newBubbleteaPipeline() *bubbleteaPipeline {
	ensureRandomPools()
	zone.NewGlobal()

	largeVP := bubbles.New(bubbles.WithWidth(benchWidth), bubbles.WithHeight(largeVPVisibleLines))
	smallVP := bubbles.New(bubbles.WithWidth(benchWidth), bubbles.WithHeight(smallVPContentLines))

	largeContent := makeBenchANSIContentString(largeVPContentLines)
	smallContent := makeBenchANSIContentString(smallVPContentLines)

	largeLines := strings.Split(largeContent, "\n")

	largeVP.SetContentLines(largeLines)
	largeVP.GotoBottom()
	smallVP.SetContentLines(strings.Split(smallContent, "\n"))

	pipe := &bubbleteaPipeline{
		largeVP:      largeVP,
		smallVP:      smallVP,
		largeLines:   largeLines,
		smallContent: smallContent,
		sepStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4")).Bold(true),
		treeStr:      buildLipglossTree(),
	}

	pipe.tbl = pipe.buildLipglossTable()

	return pipe
}

func (pipe *bubbleteaPipeline) buildLipglossTable() *lipglosstable.Table {
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	doneStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))

	tbl := lipglosstable.New().
		Width(tableWidth).
		Border(lipgloss.NormalBorder()).
		Headers("Name", "Status", "Time", "Arch").
		StyleFunc(func(_, col int) lipgloss.Style {
			switch col {
			case 0:
				return nameStyle
			case 1:
				return doneStyle
			default:
				return lipgloss.NewStyle()
			}
		})

	for rowIdx := range tableRows {
		tbl.Row(fmt.Sprintf("item-%d", rowIdx), "done", "1.23s", "x86_64")
	}

	return tbl
}

func (pipe *bubbleteaPipeline) view() string {
	var builder strings.Builder

	builder.WriteString(pipe.largeVP.View())
	builder.WriteByte('\n')

	builder.WriteString(pipe.sepStyle.Render(separatorText))
	builder.WriteByte('\n')

	builder.WriteString(pipe.smallVP.View())
	builder.WriteByte('\n')

	builder.WriteString(pipe.sepStyle.Render(separatorText))
	builder.WriteByte('\n')

	builder.WriteString(pipe.treeStr)
	builder.WriteByte('\n')

	builder.WriteString(pipe.sepStyle.Render(separatorText))
	builder.WriteByte('\n')

	builder.WriteString(pipe.tbl.String())

	return builder.String()
}

func buildLipglossTree() string {
	connStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))

	var build func(depth int) *lipglosstree.Tree

	build = func(depth int) *lipglosstree.Tree {
		label := fmt.Sprintf("node-d%d", treeDepth-depth)
		if depth%2 == 0 {
			label = lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD")).Bold(true).Render(label)
		}

		node := lipglosstree.Root(label).
			EnumeratorStyle(connStyle).
			IndenterStyle(connStyle)

		if depth > 0 {
			for range treeBreadth {
				node.Child(build(depth - 1))
			}
		}

		return node
	}

	root := lipglosstree.Root("").
		EnumeratorStyle(connStyle).
		IndenterStyle(connStyle)

	for range treeBreadth {
		root.Child(build(treeDepth - 1))
	}

	return root.String()
}

func (pipe *bubbleteaPipeline) randomizeLargeViewport() {
	pipe.largeLines = randomBubbleteaPool[pipe.randomPoolIdx%randomPoolSize]
	pipe.randomPoolIdx++
	pipe.largeVP.SetContentLines(pipe.largeLines)
	pipe.largeVP.GotoBottom()
}

func (pipe *bubbleteaPipeline) appendLargeViewport(frameCount int) {
	if len(pipe.largeLines) >= maxLargeVPLines {
		return
	}

	base := frameCount * 3
	pipe.largeLines = append(pipe.largeLines,
		fmt.Sprintf("\x1b[32mnew line %d\x1b[0m appended content here", base),
		fmt.Sprintf("\x1b[32mnew line %d\x1b[0m appended content here", base+1),
		fmt.Sprintf("\x1b[32mnew line %d\x1b[0m appended content here", base+2),
	)

	pipe.largeVP.SetContentLines(pipe.largeLines)
	pipe.largeVP.GotoBottom()
}

func (pipe *bubbleteaPipeline) toggleTableStatus(frameCount int) {
	rowIdx := frameCount % tableRows

	newStatus := "running"
	if frameCount%2 == 0 {
		newStatus = "done"
	}

	pipe.tbl = pipe.buildLipglossTableWithUpdate(rowIdx, newStatus)
}

func (pipe *bubbleteaPipeline) buildLipglossTableWithUpdate(changedRow int, status string) *lipglosstable.Table {
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	doneStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))
	runningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F1FA8C"))

	// Store row statuses for StyleFunc dispatch
	rowStatuses := make([]string, tableRows)
	for rowIdx := range tableRows {
		rowStatuses[rowIdx] = "done"
	}

	rowStatuses[changedRow] = status

	tbl := lipglosstable.New().
		Width(tableWidth).
		Border(lipgloss.NormalBorder()).
		Headers("Name", "Status", "Time", "Arch").
		StyleFunc(func(row, col int) lipgloss.Style {
			switch col {
			case 0:
				return nameStyle
			case 1:
				if row >= 0 && row < len(rowStatuses) && rowStatuses[row] == "running" {
					return runningStyle
				}

				return doneStyle
			default:
				return lipgloss.NewStyle()
			}
		})

	for rowIdx := range tableRows {
		tbl.Row(fmt.Sprintf("item-%d", rowIdx), rowStatuses[rowIdx], "1.23s", "x86_64")
	}

	return tbl
}

func benchBubbleteaPipeline(b *testing.B, updateMode int) {
	b.Helper()
	b.ReportAllocs()

	pipe := newBubbleteaPipeline()

	var termBuf bytes.Buffer

	renderer := uv.NewTerminalRenderer(&termBuf, []string{"TERM=xterm-256color"})
	renderer.SetScrollOptim(true)

	screen := uv.NewScreenBuffer(benchWidth, benchHeight)

	// Pre-render first frame, matching cursedRenderer.flush() pipeline
	content := pipe.view()
	pipe.lastViewContent = content

	screen.Clear()

	styledStr := uv.NewStyledString(content)
	styledStr.Draw(screen, screen.Bounds())
	renderer.Render(screen.RenderBuffer)
	_ = renderer.Flush()

	frameCount := 0

	b.ResetTimer()

	for b.Loop() {
		switch updateMode {
		case updateNone:
		case updateEveryFrame:
			pipe.randomizeLargeViewport()
			pipe.toggleTableStatus(frameCount)
		case updateQuarterFrame:
			if frameCount%4 == 0 {
				pipe.appendLargeViewport(frameCount)
				pipe.toggleTableStatus(frameCount)
			}
		}

		content = pipe.view()

		// viewEquals optimization: cursedRenderer.flush() skips rendering
		// entirely when the view Content string is identical to last frame.
		if content == pipe.lastViewContent {
			frameCount++

			continue
		}

		pipe.lastViewContent = content

		// Clear screen buffer before Draw, matching cursedRenderer.flush():
		// "Clear our screen buffer before copying the new frame into it
		// to ensure we erase any old content."
		screen.Clear()

		styledStr = uv.NewStyledString(content)
		styledStr.Draw(screen, screen.Bounds())
		renderer.Render(screen.RenderBuffer)
		_ = renderer.Flush()

		termBuf.Reset()

		frameCount++
	}
}

// ────────────────────────────────────────────────────────────────────────────
// cview pipeline
// ────────────────────────────────────────────────────────────────────────────

type cviewPipeline struct {
	flex          *cview.Flex
	largeTV       *cview.TextView
	smallTV       *cview.TextView
	treeView      *cview.TreeView
	tbl           *cview.Table
	largeLines    [][]byte
	screen        tcell.Screen
	randomPoolIdx int
}

func newCviewSepTV() *cview.TextView {
	tv := cview.NewTextView()
	tv.SetText("[blue::b]" + separatorText + "[-]")
	tv.SetRect(0, 0, benchWidth, 1)

	return tv
}

func newCviewPipeline() *cviewPipeline {
	ensureRandomPools()

	flex := cview.NewFlex()
	flex.SetDirection(cview.FlexRow)

	// Large viewport
	largeTV := cview.NewTextView()
	largeTV.SetScrollBarVisibility(cview.ScrollBarNever)
	largeTV.SetDynamicColors(true)

	largeLines := makeCviewColorLines(largeVPContentLines)

	largeTV.SetBytes(joinLines(largeLines))
	largeTV.ScrollToEnd()
	largeTV.SetRect(0, 0, benchWidth, largeVPVisibleLines)
	flex.AddItem(largeTV, largeVPVisibleLines, 0, false)

	flex.AddItem(newCviewSepTV(), 1, 0, false)

	// Small viewport
	smallTV := cview.NewTextView()
	smallTV.SetScrollBarVisibility(cview.ScrollBarNever)
	smallTV.SetDynamicColors(true)

	smallLines := makeCviewColorLines(smallVPContentLines)
	smallTV.SetBytes(joinLines(smallLines))
	smallTV.SetRect(0, 0, benchWidth, smallVPVisibleLines)
	flex.AddItem(smallTV, smallVPVisibleLines, 0, false)

	flex.AddItem(newCviewSepTV(), 1, 0, false)

	// Tree
	treeView := buildCviewTree()
	treeView.SetRect(0, 0, benchWidth, 12)
	flex.AddItem(treeView, 12, 0, false)

	flex.AddItem(newCviewSepTV(), 1, 0, false)

	// Table
	tbl := buildCviewTable()
	tbl.SetRect(0, 0, tableWidth, tableRows+2)
	flex.AddItem(tbl, tableRows+2, 0, false)

	screen := mustInitPipelineScreen(benchHeight)
	flex.SetRect(0, 0, benchWidth, benchHeight)

	pipe := &cviewPipeline{
		flex:       flex,
		largeTV:    largeTV,
		smallTV:    smallTV,
		treeView:   treeView,
		tbl:        tbl,
		largeLines: largeLines,
		screen:     screen,
	}

	return pipe
}

func buildCviewTree() *cview.TreeView {
	root := cview.NewTreeNode("")
	root.SetExpanded(true)

	var build func(depth int) *cview.TreeNode

	build = func(depth int) *cview.TreeNode {
		label := fmt.Sprintf("node-d%d", treeDepth-depth)
		node := cview.NewTreeNode(label)
		node.SetExpanded(true)

		if depth%2 == 0 {
			node.SetColor(tcellColorCyan)
		}

		if depth > 0 {
			for range treeBreadth {
				node.AddChild(build(depth - 1))
			}
		}

		return node
	}

	for range treeBreadth {
		root.AddChild(build(treeDepth - 1))
	}

	treeView := cview.NewTreeView()
	treeView.SetRoot(root)
	treeView.SetCurrentNode(root)
	treeView.SetGraphics(true)
	treeView.SetGraphicsColor(tcellColorCyan)
	treeView.SetTopLevel(1)

	return treeView
}

func buildCviewTable() *cview.Table {
	tbl := cview.NewTable()
	tbl.SetBorders(true)
	tbl.SetFixed(1, 0)

	headers := []string{"Name", "Status", "Time", "Arch"}
	for col, header := range headers {
		cell := cview.NewTableCell(header)
		cell.SetTextColor(tcellColorCyan)
		cell.SetExpansion(1)
		tbl.SetCell(0, col, cell)
	}

	for rowIdx := range tableRows {
		cells := []string{
			fmt.Sprintf("item-%d", rowIdx),
			"done",
			"1.23s",
			"x86_64",
		}

		for col, text := range cells {
			cell := cview.NewTableCell(text)
			cell.SetExpansion(1)

			if col == 1 {
				cell.SetTextColor(tcellColorGreen)
			}

			tbl.SetCell(rowIdx+1, col, cell)
		}
	}

	return tbl
}

func (pipe *cviewPipeline) randomizeLargeViewport() {
	pipe.largeLines = randomCviewPool[pipe.randomPoolIdx%randomPoolSize]
	pipe.largeTV.SetBytes(randomCviewJoined[pipe.randomPoolIdx%randomPoolSize])
	pipe.randomPoolIdx++
	pipe.largeTV.ScrollToEnd()
}

func (pipe *cviewPipeline) appendLargeViewport(frameCount int) {
	if len(pipe.largeLines) >= maxLargeVPLines {
		return
	}

	base := frameCount * 3
	for i := range linesPerUpdate {
		line := fmt.Appendf(nil, "[green]new line %d[-] appended content here", base+i)
		pipe.largeLines = append(pipe.largeLines, line)
	}

	pipe.largeTV.SetBytes(joinLines(pipe.largeLines))
	pipe.largeTV.ScrollToEnd()
}

func (pipe *cviewPipeline) toggleTableStatus(frameCount int) {
	rowIdx := frameCount%tableRows + 1 // +1 for header row

	newStatus := "running"
	if frameCount%2 == 0 {
		newStatus = "done"
	}

	cell := pipe.tbl.GetCell(rowIdx, 1)
	cell.SetText(newStatus)

	if newStatus == "running" {
		cell.SetTextColor(tcellColorYellow)
	} else {
		cell.SetTextColor(tcellColorGreen)
	}

	pipe.tbl.SetCell(rowIdx, 1, cell)
}

func mustInitPipelineScreen(height int) tcell.Screen {
	mt := vt.NewMockTerm(vt.MockOptSize{X: vt.Col(benchWidth), Y: vt.Row(height)})

	scr, err := tcell.NewTerminfoScreenFromTty(mt)
	if err != nil {
		panic(fmt.Sprintf("create screen from mock tty: %v", err))
	}

	err = scr.Init()
	if err != nil {
		panic(fmt.Sprintf("init screen: %v", err))
	}

	return scr
}

func benchCviewPipeline(b *testing.B, updateMode int) {
	b.Helper()
	b.ReportAllocs()

	pipe := newCviewPipeline()
	defer pipe.screen.Fini()

	// Initial draw + Show to populate tcell's cell buffer and dirty flags
	pipe.flex.Draw(pipe.screen)
	pipe.screen.Show()

	frameCount := 0

	b.ResetTimer()

	for b.Loop() {
		switch updateMode {
		case updateNone:
		case updateEveryFrame:
			pipe.randomizeLargeViewport()
			pipe.toggleTableStatus(frameCount)
		case updateQuarterFrame:
			if frameCount%4 == 0 {
				pipe.appendLargeViewport(frameCount)
				pipe.toggleTableStatus(frameCount)
			}
		}

		// Draw writes cells into tcell's in-memory buffer.
		pipe.flex.Draw(pipe.screen)

		// Show triggers tcell's draw() which iterates all cells, diffs against
		// previous state (dirty flag per cell), and emits terminal bytes.
		// This is the cview equivalent of zeroterm's RenderLines and
		// bubbletea's TerminalRenderer.Render.
		pipe.screen.Show()

		frameCount++
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Benchmark entry points
// ────────────────────────────────────────────────────────────────────────────

// --- NoChange (no updates between frames, cache/diff hit) ---

func Benchmark__Pipeline_NoChange(b *testing.B) {
	benchZerotermPipeline(b, updateNone)
}

func Benchmark_Bubbletea__Pipeline_NoChange(b *testing.B) {
	benchBubbleteaPipeline(b, updateNone)
}

func Benchmark_Cview__Pipeline_NoChange(b *testing.B) {
	benchCviewPipeline(b, updateNone)
}

// --- EveryFrameUpdate (every frame: viewport re-randomized + table status toggle) ---

func Benchmark__Pipeline_EveryFrameUpdate(b *testing.B) {
	benchZerotermPipeline(b, updateEveryFrame)
}

func Benchmark_Bubbletea__Pipeline_EveryFrameUpdate(b *testing.B) {
	benchBubbleteaPipeline(b, updateEveryFrame)
}

func Benchmark_Cview__Pipeline_EveryFrameUpdate(b *testing.B) {
	benchCviewPipeline(b, updateEveryFrame)
}

// --- QuarterFrameUpdate (1/4 frames: viewport append + table status toggle) ---

func Benchmark__Pipeline_QuarterFrameUpdate(b *testing.B) {
	benchZerotermPipeline(b, updateQuarterFrame)
}

func Benchmark_Bubbletea__Pipeline_QuarterFrameUpdate(b *testing.B) {
	benchBubbleteaPipeline(b, updateQuarterFrame)
}

func Benchmark_Cview__Pipeline_QuarterFrameUpdate(b *testing.B) {
	benchCviewPipeline(b, updateQuarterFrame)
}
