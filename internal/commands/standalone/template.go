package commands_standalone

import (
	"os"

	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/template"
	"github.com/mihakrumpestar/panix/internal/pkg/yamlx"
	"github.com/pkg/errors"
)

type TemplateCmd struct {
	OutputFlag

	flags.ConfigFlags
}

func (c *TemplateCmd) Run() error {
	rawYAML, err := os.ReadFile(c.Config)
	if err != nil {
		return errors.Wrapf(err, "failed reading config %s", c.Config)
	}

	processedYAML, err := template.ProcessTemplate(rawYAML)
	if err != nil {
		return errors.Wrap(err, "failed to process templates")
	}

	var decoded struct {
		Flags any
		Fleet any
	}

	err = yamlx.Decode(processedYAML, &decoded)
	if err != nil {
		return errors.Wrap(err, "failed to decode config")
	}

	err = yamlx.WriteTo(decoded, c.Output)

	return errors.Wrap(err, "failed to write templated config")
}
