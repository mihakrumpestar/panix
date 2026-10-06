package clipboard

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/pkg/errors"
)

const cmdTimeout = 5 * time.Second

var errClipboardUnavailable = errors.New("failed to copy to clipboard: no clipboard method available (tried pbcopy, wl-copy, xclip, xsel, and OSC52)") //nolint:lll

var envCache = struct {
	sync.Once

	isWayland bool
}{}

// clipboardCommand is one candidate clipboard command.
type clipboardCommand struct {
	name string
	args []string
}

// clipboardPlan returns the clipboard commands to probe, in order, for the
// given platform. Darwin always has pbcopy; Linux probes the Wayland tool
// first when a Wayland session is detected, then the X11 tools; other
// platforms have no command (OSC52 handles them).
func clipboardPlan(goos string, isWayland bool) []clipboardCommand {
	switch goos {
	case "darwin":
		return []clipboardCommand{{name: "pbcopy"}}
	case "linux":
		if isWayland {
			return []clipboardCommand{
				{name: "wl-copy", args: []string{"--"}},
				{name: "xclip", args: []string{"-selection", "clipboard", "-in"}},
				{name: "xsel", args: []string{"--clipboard", "--input"}},
			}
		}

		return []clipboardCommand{
			{name: "xclip", args: []string{"-selection", "clipboard", "-in"}},
			{name: "xsel", args: []string{"--clipboard", "--input"}},
		}
	default:
		return nil
	}
}

// CopyToClipboard copies text to system clipboard. Tries system commands
// (pbcopy, wl-copy, xclip, xsel), then falls back to OSC52 for terminal/SSH.
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
	text = string(style.StripANSI([]byte(text)))

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
	return probeClipboardCommands(ctx, clipboardPlan(runtime.GOOS, isWayland()), text)
}

// probeClipboardCommands feeds text to each plan candidate via stdin and
// reports whether any of them succeeded.
func probeClipboardCommands(ctx context.Context, plan []clipboardCommand, text string) bool {
	for _, candidate := range plan {
		//nolint:gosec // the candidate comes from the fixed clipboard plan, not user input
		cmd := exec.CommandContext(ctx, candidate.name, candidate.args...)
		cmd.Stdin = bytes.NewReader([]byte(text))

		if cmd.Run() == nil {
			return true
		}
	}

	return false
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
