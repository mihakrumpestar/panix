package commands_standalone

import (
	"os"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
	"github.com/pkg/errors"
)

type EvalCmd struct {
	OutputFlag

	flags.ConfigFlags
	flags.EvalFlags
	flags.Logging
	flags.Tui     `embed:"" prefix:"tui."`
	flags.Runtime `embed:"" prefix:"runtime."`
}

func (c *EvalCmd) Run() error {
	parsed := flags.Flags{
		WorkflowFlags: flags.WorkflowFlags{
			ConfigFlags: c.ConfigFlags,
			EvalFlags:   c.EvalFlags,
			Logging:     c.Logging,
			Tui:         c.Tui,
			Runtime:     c.Runtime,
		},
	}

	parsed.WorkflowFlags.EvalFlags.Validate.Flakes = true
	parsed.WorkflowFlags.EvalFlags.Validate.BootstrapSecrets = true

	conf, err := config.LoadConfig(parsed, flags.ExplicitMask(os.Args[1:], c))
	if err != nil {
		return errors.Wrap(err, "evaluation failed")
	}

	err = yamlx.WriteTo(conf, c.Output)

	return errors.Wrap(err, "failed to write evaluated config")
}
