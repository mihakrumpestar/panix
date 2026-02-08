package tui_clipboard

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"
	"github.com/aymanbagabas/go-osc52/v2"
	"github.com/pkg/errors"
)

const cmdTimeout = 5 * time.Second

// ansiRegex matches ANSI escape sequences
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes ANSI escape sequences from text
func stripANSI(text string) string {
	return ansiRegex.ReplaceAllString(text, "")
}

// normalizeText prepares text for clipboard by stripping ANSI and trimming
func normalizeText(text string) string {
	text = strings.TrimSpace(text)
	text = stripANSI(text)
	return text
}

// envCache caches environment checks to avoid repeated lookups
var envCache = struct {
	sync.Once
	isWayland bool
}{}

// isWayland returns true if running under Wayland
func isWayland() bool {
	envCache.Do(func() {
		envCache.isWayland = os.Getenv("WAYLAND_DISPLAY") != "" ||
			strings.Contains(os.Getenv("XDG_SESSION_TYPE"), "wayland")
	})
	return envCache.isWayland
}

// copyWithCommand tries to copy using system clipboard commands
func copyWithCommand(ctx context.Context, text string) bool {
	// Try Wayland native tool first if on Wayland
	if isWayland() {
		cmd := exec.CommandContext(ctx, "wl-copy", "--", text)
		if err := cmd.Run(); err == nil {
			return true
		}
	}

	// Try xclip for X11
	cmd := exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-in")
	cmd.Stdin = bytes.NewReader([]byte(text))
	if err := cmd.Run(); err == nil {
		return true
	}

	// Try xsel as fallback
	cmd = exec.CommandContext(ctx, "xsel", "--clipboard", "--input")
	cmd.Stdin = bytes.NewReader([]byte(text))
	if err := cmd.Run(); err == nil {
		return true
	}

	return false
}

// copyWithLibrary tries to copy using atotto/clipboard library
func copyWithLibrary(text string) bool {
	if err := clipboard.WriteAll(text); err == nil {
		return true
	}
	return false
}

// copyWithOSC52 copies using OSC52 terminal escape sequences
func copyWithOSC52(text string) error {
	_, err := osc52.New(text).WriteTo(os.Stdout)
	if err != nil {
		return errors.Wrap(err, "writing OSC52 to terminal failed")
	}
	return nil
}

// CopyToClipboard copies the given text to the system clipboard
// It tries multiple methods in order: system commands, library, OSC52
func CopyToClipboard(text string) error {
	normalized := normalizeText(text)

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	// Try system clipboard commands first (Wayland/X11)
	if copyWithCommand(ctx, normalized) {
		return nil
	}

	// Try clipboard library
	if copyWithLibrary(normalized) {
		return nil
	}

	// Fall back to OSC52 terminal-based clipboard
	if err := copyWithOSC52(normalized); err != nil {
		return errors.New("failed to copy to clipboard: no clipboard method available (tried wl-copy, xclip, xsel, atotto/clipboard, and OSC52)")
	}

	return nil
}
