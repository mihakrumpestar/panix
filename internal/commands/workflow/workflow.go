package commands_workflow

import (
	"context"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/pkg/errors"
)

func runWorkflow(f flags.Flags, commandPhases []phase.Phase) error {
	conf, err := config.LoadConfig(f)
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	conf.Phases, err = phase.ValidatePhases(commandPhases, conf.Flags.SkipPhases)
	if err != nil {
		return errors.Wrap(err, "invalid phases")
	}

	conf.FilterOutUnusedPhases()

	if conf.Flags.Output != flags.OutputModeTui {
		return errors.Wrap(tui.NewHeadless(context.Background(), conf), "headless error")
	}

	return errors.Wrap(tui.New(context.Background(), conf, false), "TUI error")
}
