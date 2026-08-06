package commands_standalone

import (
	"context"

	"github.com/mihakrumpestar/panix/internal/snapshot"
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/pkg/errors"
)

type SnapshotCmd struct {
	Path string `name:"path" short:"p" help:"Snapshot file path" required:"" completion-predictor:"json-file"`
}

func (c *SnapshotCmd) Run() error {
	snap, err := snapshot.Read(c.Path)
	if err != nil {
		return errors.Wrap(err, "failed to read snapshot file")
	}

	return errors.Wrap(tui.New(context.Background(), snap, true), "snapshot TUI error")
}
