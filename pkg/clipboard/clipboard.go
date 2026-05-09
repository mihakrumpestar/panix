package clipboard

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/pkg/errors"
)

const cmdTimeout = 5 * time.Second

var errClipboardUnavailable = errors.New("failed to copy to clipboard: no clipboard method available (tried wl-copy, xclip, xsel, and OSC52)")

var envCache = struct {
	sync.Once

	isWayland bool
}{}

// CopyToClipboard copies text to system clipboard. Tries system commands
// (wl-copy, xclip, xsel), then falls back to OSC52 for terminal/SSH.
func CopyToClipboard(text string) error {
	normalized := normalizeText(text)

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	if copyWithCommand(ctx, normalized) {
		return nil
	}

	err := copyWithOSC52(normalized)
	if err != nil {
		return errClipboardUnavailable
	}

	return nil
}

func normalizeText(text string) string {
	text = strings.TrimSpace(text)
	text = style.StripANSI(text)

	return text
}

func isWayland() bool {
	envCache.Do(func() {
		envCache.isWayland = os.Getenv("WAYLAND_DISPLAY") != "" ||
			strings.Contains(os.Getenv("XDG_SESSION_TYPE"), "wayland")
	})

	return envCache.isWayland
}

func copyWithCommand(ctx context.Context, text string) bool {
	if isWayland() {
		cmd := exec.CommandContext(ctx, "wl-copy", "--")
		cmd.Stdin = bytes.NewReader([]byte(text))

		err := cmd.Run()
		if err == nil {
			return true
		}
	}

	cmd := exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-in")
	cmd.Stdin = bytes.NewReader([]byte(text))

	err := cmd.Run()
	if err == nil {
		return true
	}

	cmd = exec.CommandContext(ctx, "xsel", "--clipboard", "--input")
	cmd.Stdin = bytes.NewReader([]byte(text))

	return cmd.Run() == nil
}

func copyWithOSC52(text string) error {
	return writeOSC52(os.Stdout, text)
}

func writeOSC52(w io.Writer, text string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))

	_, err := w.Write([]byte("\x1b]52;" + encoded + "\x07"))
	if err != nil {
		return errors.Wrap(err, "writing OSC52 sequence")
	}

	return nil
}
