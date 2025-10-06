package config

import "github.com/charmbracelet/lipgloss"

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
	Flake         lipgloss.Style
	Configuration lipgloss.Style
	Machine       lipgloss.Style
	Phase         lipgloss.Style
	Command       lipgloss.Style
	Error         lipgloss.Style

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

	// Icons
	IconFlake         rune
	IconConfiguration rune
	IconMachine       rune
	IconPhase         rune
	IconCommand       rune
	IconError         rune
}

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

		// Entity colors
		Flake: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F1FA8C")), // Yellow

		Configuration: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C")), // Orange

		Machine: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8BE9FD")), // Cyan

		Phase: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF79C6")), // Pink

		Command: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BD93F9")), // Purple

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")), // Red

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

		// Icons
		IconFlake:         '📁',
		IconConfiguration: '📦',
		IconMachine:       '💻',
		IconPhase:         '📋',
		IconCommand:       '⚙',
		IconError:         '✗',
	}
}
