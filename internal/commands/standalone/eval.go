package commands_standalone

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
	"github.com/pkg/errors"
)

type EvalCmd struct {
	OutputFlag

	flags.ConfigFlags
	flags.EvalFlags
}

func (c *EvalCmd) Run() error {
	flags := flags.Flags{
		WorkflowFlags: flags.WorkflowFlags{
			ConfigFlags: c.ConfigFlags,
			EvalFlags:   c.EvalFlags,
		},
	}

	flags.WorkflowFlags.EvalFlags.Validate.Flakes = true
	flags.WorkflowFlags.EvalFlags.Validate.BootstrapSecrets = true

	conf, err := config.LoadConfig(flags)
	if err != nil {
		return errors.Wrap(err, "evaluation failed")
	}

	err = yamlx.WriteTo(conf, c.Output)

	return errors.Wrap(err, "failed to write evaluated config")
}
