package commands_standalone

import (
	"os"

	"github.com/mihakrumpestar/panix/examples"
	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/pkg/errors"
)

type InitCmd struct {
	Output string `name:"output" short:"o" help:"Output file path" default:"panix.yml"`
	Force  bool   `name:"force" short:"f" help:"Overwrite existing file"`
}

func (c *InitCmd) Run() error {
	if !c.Force {
		_, err := os.Stat(c.Output)
		if err == nil {
			return errors.Errorf("file %s already exists, use --force to overwrite", c.Output)
		}
	}

	err := os.WriteFile(c.Output, examples.ExampleConfig, filepermissions.DefaultFilePermissions)
	if err != nil {
		return errors.Wrap(err, "failed to write config file")
	}

	return nil
}
