package colorscheme

import (
	"fmt"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
)

type ColorSchemeLogEntity struct {
	Color style.Style
	Icon  rune
}

type ColorSchemeFooter struct {
	HelpKey          style.Style
	HelpDesc         style.Style
	HelpSeparator    style.Style
	HelpSelectedKey  style.Style
	HelpSelectedDesc style.Style
	DebugBackground  style.Style
}

type ColorSchemeHeader struct {
	Title  style.Style
	Border style.Style
}

type ColorSchemeStatus struct {
	OK      style.Style
	Warning style.Style
	Failed  style.Style
	Running style.Style
}

type ColorSchemeTableAndLogs struct {
	Header                       style.Style
	Border                       style.Style
	Row                          style.Style
	RowAlt                       style.Style
	SelectionHighlightBackground style.Style
	SelectionHighlightBorder     style.Style
}

type ColorSchemeTree struct {
	Enumerator style.Style
}

type ColorScheme struct {
	Header      ColorSchemeHeader
	Status      ColorSchemeStatus
	Table       ColorSchemeTableAndLogs
	PhaseStatus ColorSchemePhaseStatus

	Tree    ColorSchemeTree
	Spinner style.Style
	Footer  ColorSchemeFooter

	Flake         ColorSchemeLogEntity
	Configuration ColorSchemeLogEntity
	Machine       ColorSchemeLogEntity
	Phase         ColorSchemeLogEntity
	Command       ColorSchemeLogEntity
	Error         ColorSchemeLogEntity
}

type ColorSchemePhaseStatus struct {
	Running ColorPair
	Failed  ColorPair
	Done    ColorPair
	Default ColorPair
	Pill    style.Style
}

type ColorPair [2]colorful.Color

func DefaultColorScheme() *ColorScheme {
	borderStyle := makeForegroundStyle("#6272A4", false)

	return &ColorScheme{
		Header: ColorSchemeHeader{
			Title:  makeForegroundStyle("#00ADD8", true),
			Border: borderStyle,
		},
		Status: ColorSchemeStatus{
			OK:      makeForegroundStyle("#50FA7B", false),
			Warning: makeForegroundStyle("#FFB86C", false),
			Failed:  makeForegroundStyle("#FF5555", false),
			Running: makeForegroundStyle("#00BFFF", false),
		},
		Table: ColorSchemeTableAndLogs{
			Header:                       makeBoldForegroundStyle("#F8F8F2"),
			Border:                       borderStyle,
			Row:                          makeForegroundStyle("#F8F8F2", false),
			RowAlt:                       makeForegroundStyle("#BFBFBF", false),
			SelectionHighlightBackground: makeBackgroundStyle("#3B3258"),
			SelectionHighlightBorder:     makeBackgroundStyle("#BD93F9"),
		},
		PhaseStatus: ColorSchemePhaseStatus{
			Running: mustColorfulHexPair("#01536e", "#007da7"),
			Failed:  mustColorfulHexPair("#5f1414", "#DC2626"),
			Done:    mustColorfulHexPair("#14532D", "#11883d"),
			Default: mustColorfulHexPair("#535862", "#6B7280"),
			Pill:    style.NewStyle().Foreground(style.Color("#FFFFFF")).Bold(true).Padding(0, 1),
		},
		Tree: ColorSchemeTree{
			Enumerator: borderStyle,
		},
		Footer: ColorSchemeFooter{
			HelpKey:          style.NewStyle().Foreground(style.Color("#FFFFFF")),
			HelpDesc:         style.NewStyle().Foreground(style.Color("#8e8e8e")),
			HelpSeparator:    style.NewStyle().Foreground(style.Color("#6272A4")),
			HelpSelectedKey:  style.NewStyle().Foreground(style.Color("#8BE9FD")).Bold(true),
			HelpSelectedDesc: style.NewStyle().Foreground(style.Color("#568CAF")),
			DebugBackground:  style.NewStyle().Background(style.Color("#FFC800")),
		},
		Flake:         makeLogEntity("#F1FA8C", '📁', true),
		Configuration: makeLogEntity("#FFB86C", '📦', false),
		Machine:       makeLogEntity("#8BE9FD", '💻', false),
		Phase:         makeLogEntity("#FF79C6", '📋', false),
		Command:       makeLogEntity("#BD93F9", '⚙', false),
		Error:         makeLogEntity("#FF5555", '✗', false),
		Spinner:       makeForegroundStyle("#8BE9FD", false),
	}
}

// Helpers

func makeForegroundStyle(color string, bold bool) style.Style {
	s := style.NewStyle().Foreground(style.Color(color))
	if bold {
		s = s.Bold(true)
	}

	return s
}

func makeBoldForegroundStyle(color string) style.Style {
	return style.NewStyle().Bold(true).Foreground(style.Color(color))
}

func makeBackgroundStyle(color string) style.Style {
	return style.NewStyle().Background(style.Color(color))
}

func makeLogEntity(color string, icon rune, bold bool) ColorSchemeLogEntity {
	sty := makeForegroundStyle(color, bold)

	return ColorSchemeLogEntity{Color: sty, Icon: icon}
}

func mustColorfulHex(hex string) colorful.Color {
	c, err := colorful.Hex(hex)
	if err != nil {
		panic(fmt.Sprintf("Invalid color hex %s: %v", hex, err))
	}

	return c
}

func mustColorfulHexPair(hex1, hex2 string) ColorPair {
	return ColorPair{mustColorfulHex(hex1), mustColorfulHex(hex2)}
}
