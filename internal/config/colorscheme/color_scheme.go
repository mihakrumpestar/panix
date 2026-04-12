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

type ColorSchemeHeader struct {
	Title  lipgloss.Style
	Border lipgloss.Style
}

type ColorSchemeStatus struct {
	OK      lipgloss.Style
	Warning lipgloss.Style
	Failed  lipgloss.Style
	Running lipgloss.Style
}

type ColorSchemeTable struct {
	Header                       lipgloss.Style
	Border                       lipgloss.Style
	Row                          lipgloss.Style
	RowAlt                       lipgloss.Style
	SelectionHighlightBackground lipgloss.Style
}

type ColorSchemeTree struct {
	Enumerator lipgloss.Style
}

type ColorScheme struct {
	Header      ColorSchemeHeader
	Status      ColorSchemeStatus
	Table       ColorSchemeTable
	PhaseStatus ColorSchemePhaseStatus

	Tree    ColorSchemeTree
	Spinner lipgloss.Style

	Flake         ColorSchemeLogEntity
	Configuration ColorSchemeLogEntity
	Machine       ColorSchemeLogEntity
	Phase         ColorSchemeLogEntity
	Command       ColorSchemeLogEntity
	Error         ColorSchemeLogEntity
}

type ColorSchemePhaseStatus struct {
	Running [2]colorful.Color
	Failed  [2]colorful.Color
	Done    [2]colorful.Color
	Default [2]colorful.Color
}

func DefaultColorScheme() *ColorScheme {
	borderStyle := makeBorderStyle()

	return &ColorScheme{
		Header: ColorSchemeHeader{
			Title:  makeHeaderTitleStyle(),
			Border: borderStyle,
		},
		Status: ColorSchemeStatus{
			OK:      makeForegroundStyle("#50FA7B"),
			Warning: makeForegroundStyle("#FFB86C"),
			Failed:  makeForegroundStyle("#FF5555"),
			Running: makeForegroundStyle("#00BFFF"),
		},
		Table: ColorSchemeTable{
			Header:                       makeBoldForegroundStyle("#F8F8F2"),
			Border:                       borderStyle,
			Row:                          makeForegroundStyle("#F8F8F2"),
			RowAlt:                       makeForegroundStyle("#BFBFBF"),
			SelectionHighlightBackground: makeBackgroundStyle("#444444"),
		},
		PhaseStatus: ColorSchemePhaseStatus{
			Running: [2]colorful.Color{mustColorfulHex("#01536e"), mustColorfulHex("#007da7")},
			Failed:  [2]colorful.Color{mustColorfulHex("#5f1414"), mustColorfulHex("#DC2626")},
			Done:    [2]colorful.Color{mustColorfulHex("#14532D"), mustColorfulHex("#11883d")},
			Default: [2]colorful.Color{mustColorfulHex("#535862"), mustColorfulHex("#6B7280")},
		},
		Tree: ColorSchemeTree{
			Enumerator: borderStyle,
		},
		Spinner:       makeForegroundStyle("#8BE9FD"),
		Flake:         makeLogEntity("#F1FA8C", '📁', true),
		Configuration: makeLogEntity("#FFB86C", '📦', false),
		Machine:       makeLogEntity("#8BE9FD", '💻', false),
		Phase:         makeLogEntity("#FF79C6", '📋', false),
		Command:       makeLogEntity("#BD93F9", '⚙', false),
		Error:         makeLogEntity("#FF5555", '✗', false),
	}
}

func makeForegroundStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
}

func makeBoldForegroundStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color))
}

func makeBackgroundStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.Color(color))
}

func makeBorderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
}

func makeHeaderTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00ADD8"))
}

func makeLogEntity(color string, icon rune, bold bool) ColorSchemeLogEntity {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	if bold {
		style = style.Bold(true)
	}

	return ColorSchemeLogEntity{Color: style, Icon: icon}
}

func mustColorfulHex(hex string) colorful.Color {
	c, err := colorful.Hex(hex)
	if err != nil {
		panic(fmt.Sprintf("Invalid color hex %s: %v", hex, err))
	}

	return c
}
