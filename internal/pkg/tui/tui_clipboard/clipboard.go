package tui_clipboard

import (
	"os"
	"strings"

	"github.com/aymanbagabas/go-osc52/v2"
	"github.com/pkg/errors"
)

// CopyToClipboard copies the given text to the system clipboard using OSC52
func CopyToClipboard(text string) error {
	text = strings.TrimSpace(text)

	_, err := osc52.New(text).WriteTo(os.Stdout)
	if err != nil {
		return errors.Wrap(err, "writing to keyboard failed")
	}

	return nil
}
