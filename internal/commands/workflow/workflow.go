package commands_workflow

import (
	"context"
	"os"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/pkg/errors"
)

func runWorkflow(cmd any, f flags.Flags, commandPhases []phase.Phase, prepare func(*config.Config) error) error {
	conf, err := config.LoadConfig(f, flags.ExplicitMask(os.Args[1:], cmd))
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	if prepare != nil {
		err = prepare(conf)
		if err != nil {
			return err
		}
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
