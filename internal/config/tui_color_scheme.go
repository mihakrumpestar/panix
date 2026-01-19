package config

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

type ColorSchemeLogEntity struct {
	Color lipgloss.Style
	Icon  rune
}

// ColorScheme defines a reusable color scheme for the TUI
type ColorScheme struct {
	// Header colors
	HeaderTitle  lipgloss.Style
	HeaderBorder lipgloss.Style

	// Status colors
	StatusOK              lipgloss.Style
	StatusWarning         lipgloss.Style
	StatusError           lipgloss.Style
	StatusRunning         lipgloss.Style
	StatusUnreachable     lipgloss.Style
	StatusSSHFailed       lipgloss.Style
	StatusNotBootstrapped lipgloss.Style

	// Entity colors
	Flake         ColorSchemeLogEntity
	Configuration ColorSchemeLogEntity
	Machine       ColorSchemeLogEntity
	Phase         ColorSchemeLogEntity
	Command       ColorSchemeLogEntity
	Error         ColorSchemeLogEntity

	// Table colors
	TableHeader lipgloss.Style
	TableBorder lipgloss.Style
	TableRow    lipgloss.Style
	TableRowAlt lipgloss.Style

	// Tree colors
	TreeRoot       lipgloss.Style
	TreeNode       lipgloss.Style
	TreeLeaf       lipgloss.Style
	TreeEnumerator lipgloss.Style

	// Spinner colors
	Spinner lipgloss.Style

	PhaseColorPairs map[PhaseState][2]colorful.Color
}

// PhaseState represents the visual state of a phase
type PhaseState int

const (
	PhaseStateDefault PhaseState = iota
	PhaseStateActive
	PhaseStateFailed
	PhaseStateCompleted
)

// DefaultColorScheme returns the default color scheme
func defaultColorScheme() *ColorScheme {
	return &ColorScheme{
		// Header colors
		HeaderTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00ADD8")), // Cyan

		HeaderBorder: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")), // Comment gray

		// Status colors
		StatusOK: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B")), // Green

		StatusWarning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C")), // Orange

		StatusError: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")), // Red

		StatusRunning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8BE9FD")), // Cyan

		StatusUnreachable: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")), // Red

		StatusSSHFailed: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C")), // Orange

		StatusNotBootstrapped: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C")), // Orange

		// Entitys
		Flake: ColorSchemeLogEntity{
			Color: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F1FA8C")), // Yellow
			Icon: '📁',
		},

		Configuration: ColorSchemeLogEntity{
			Color: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFB86C")), // Orange
			Icon: '📦',
		},

		Machine: ColorSchemeLogEntity{
			Color: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8BE9FD")), // Cyan
			Icon: '💻',
		},

		Phase: ColorSchemeLogEntity{
			Color: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF79C6")), // Pink
			Icon: '📋',
		},

		Command: ColorSchemeLogEntity{
			Color: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#BD93F9")), // Purple
			Icon: '⚙',
		},

		Error: ColorSchemeLogEntity{
			Color: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF5555")), // Red
			Icon: '✗',
		},

		// Table colors
		TableHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F8F8F2")), // White

		TableBorder: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")), // Comment gray

		TableRow: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2")), // White

		TableRowAlt: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BFBFBF")), // Light gray

		// Tree colors
		TreeRoot: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F1FA8C")), // Yellow

		TreeNode: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8BE9FD")), // Cyan

		TreeLeaf: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B")), // Green

		TreeEnumerator: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")), // Comment gray

		// Spinner colors
		Spinner: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8BE9FD")), // Cyan

		// Pre-defined and pre-parsed color pairs for different phase states
		PhaseColorPairs: map[PhaseState][2]colorful.Color{
			PhaseStateActive:    {mustColorfullHex("#2952c3"), mustColorfullHex("#3b6bec")}, // Dark blue variations
			PhaseStateFailed:    {mustColorfullHex("#5f1414"), mustColorfullHex("#DC2626")}, // Dark red variations
			PhaseStateCompleted: {mustColorfullHex("#14532D"), mustColorfullHex("#11883d")}, // Dark green variations
			PhaseStateDefault:   {mustColorfullHex("#535862"), mustColorfullHex("#6B7280")}, // Dark gray solid
		},
	}
}

// Helpers

func mustColorfullHex(hex string) colorful.Color {
	c, err := colorful.Hex(hex)
	if err != nil {
		panic(fmt.Sprintf("Invalid color hex %s: %v", hex, err))
	}
	return c
}
