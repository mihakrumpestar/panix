package colorscheme

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/lucasb-eyer/go-colorful"
)

type ColorSchemeLogEntity struct {
	Color lipgloss.Style
	Icon  rune
}

type ColorSchemeFooter struct {
	HelpKey         lipgloss.Style
	DebugBackground lipgloss.Style
}

type ColorSchemeHeader struct {
	Title  lipgloss.Style
	Border lipgloss.Style
	Time   lipgloss.Style
}

type ColorSchemeStatus struct {
	OK      lipgloss.Style
	Warning lipgloss.Style
	Failed  lipgloss.Style
	Running lipgloss.Style
}

type ColorSchemeTableAndLogs struct {
	Header                       lipgloss.Style
	Border                       lipgloss.Style
	Row                          lipgloss.Style
	RowAlt                       lipgloss.Style
	SelectionHighlightBackground lipgloss.Style
	SelectionHighlightBorder     lipgloss.Style
}

type ColorSchemeTree struct {
	Enumerator lipgloss.Style
}

type ColorScheme struct {
	Header      ColorSchemeHeader
	Status      ColorSchemeStatus
	Table       ColorSchemeTableAndLogs
	PhaseStatus ColorSchemePhaseStatus

	Tree    ColorSchemeTree
	Spinner lipgloss.Style
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
	Pill    lipgloss.Style
}

type ColorPair [2]colorful.Color

func DefaultColorScheme() *ColorScheme {
	borderStyle := makeForegroundStyle("#6272A4", false)

	return &ColorScheme{
		Header: ColorSchemeHeader{
			Title:  makeForegroundStyle("#00ADD8", true),
			Border: borderStyle,
			Time:   makeForegroundStyle("#6EE7B7", false),
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
			Pill:    lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Padding(0, 1),
		},
		Tree: ColorSchemeTree{
			Enumerator: borderStyle,
		},
		Footer: ColorSchemeFooter{
			HelpKey:         lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")),
			DebugBackground: lipgloss.NewStyle().Background(lipgloss.Color("#FFC800")),
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

func makeForegroundStyle(color string, bold bool) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	if bold {
		style = style.Bold(true)
	}

	return style
}

func makeBoldForegroundStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color))
}

func makeBackgroundStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.Color(color))
}

func makeLogEntity(color string, icon rune, bold bool) ColorSchemeLogEntity {
	style := makeForegroundStyle(color, bold)

	return ColorSchemeLogEntity{Color: style, Icon: icon}
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
