package tui_clipboard

import (
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/aymanbagabas/go-osc52/v2"
	"github.com/pkg/errors"
)

// ansiRegex matches ANSI escape sequences
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes ANSI escape sequences from text
func stripANSI(text string) string {
	return ansiRegex.ReplaceAllString(text, "")
}

// isWayland returns true if running under Wayland
func isWayland() bool {
	return os.Getenv("WAYLAND_DISPLAY") != "" || strings.Contains(os.Getenv("XDG_SESSION_TYPE"), "wayland")
}

// copyWithCommand tries to copy using system clipboard commands
// Returns true if successful, false otherwise
func copyWithCommand(text string) bool {
	text = strings.TrimSpace(text)
	text = stripANSI(text)

	// Try Wayland native tool first if on Wayland
	if isWayland() {
		cmd := exec.Command("wl-copy")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return true
		}
	}

	// Try xclip for X11
	cmd := exec.Command("xclip", "-selection", "clipboard", "-in")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err == nil {
		return true
	}

	// Try xsel as fallback
	cmd = exec.Command("xsel", "--clipboard", "--input")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err == nil {
		return true
	}

	return false
}

// copyWithLibrary tries to copy using atotto/clipboard library
// Returns true if successful, false otherwise
func copyWithLibrary(text string) bool {
	text = strings.TrimSpace(text)
	text = stripANSI(text)

	// Try atotto/clipboard library
	if err := clipboard.WriteAll(text); err == nil {
		return true
	}

	return false
}

// copyWithOSC52 copies using OSC52 terminal escape sequences
func copyWithOSC52(text string) error {
	text = strings.TrimSpace(text)
	text = stripANSI(text)

	_, err := osc52.New(text).WriteTo(os.Stdout)
	if err != nil {
		return errors.Wrap(err, "writing OSC52 to terminal failed")
	}

	return nil
}

// CopyToClipboard copies the given text to the system clipboard
// It tries multiple methods in order: system commands, library, OSC52
// ANSI escape sequences are stripped to ensure plain text is copied
func CopyToClipboard(text string) error {
	// Try system clipboard commands first (Wayland/X11)
	if copyWithCommand(text) {
		return nil
	}

	// Try clipboard library
	if copyWithLibrary(text) {
		return nil
	}

	// Fall back to OSC52 terminal-based clipboard
	if err := copyWithOSC52(text); err != nil {
		return errors.New("failed to copy to clipboard: no clipboard method available (tried wl-copy, xclip, xsel, atotto/clipboard, and OSC52)")
	}

	return nil
}
