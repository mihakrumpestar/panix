package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/rs/zerolog/log"
)

func (w *Workflow) executeBuildPhaseConfiguration(flake *config.Flake, configuration *config.Configuration) error {
	return w.Phase(configuration.Attributes.Xpath, phases.Build, nil,
		func(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog) error {
			if configuration.MetaBuild == nil {
				configuration.MetaBuild = &config.MetaBuild{}
			}

			flakeOutput := configuration.FlakeOutput
			if flakeOutput == "" {
				flakeOutput = "nixosConfigurations.<name>.config.system.build.toplevel"
			}
			flakeOutput = strings.ReplaceAll(flakeOutput, "<name>", configuration.Name)
			installables := []string{fmt.Sprintf("%s#%s", flake.URL, flakeOutput)}

			parsedOutput, err := w.executeBuildPhaseConfigurationWrapper(exc, phaseLog, flake, configuration, installables)
			if err != nil {
				return err
			}

			configuration.MetaBuild.SystemClosure = parsedOutput[0].Outputs.Out

			return nil
		})
}

func (w *Workflow) executeBuildPhaseConfigurationWrapper(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog, flake *config.Flake, configuration *config.Configuration, installables []string) (BuildOutputJson, error) {
	var parsedOutput BuildOutputJson

	commandWithArgs := append([]string{"nix", "build", "--no-link", "--no-update-lock-file", "--json"}, installables...)

	err := exc.Exec(
		"build",
		"building system closure",
		"build failed",
		commandWithArgs,
		executioner.DisableAutoSshCommand(),
		executioner.OnSuccess(func(log *logs_command.CommandLog) error {
			output := lastNonEmptyLine(log.Bytes())

			err := json.Unmarshal(output, &parsedOutput)
			if err != nil || len(parsedOutput) == 0 {
				return fmt.Errorf("invalid build output for %s/%s: %s", flake.Name, configuration.Name, strconv.Quote(string(output)))
			}

			return nil
		}),
		executioner.OnDryRun(func() {
			parsedOutput = BuildOutputJson{{Outputs: struct {
				Out string `json:"out"`
			}{Out: "BUILD_OUTPUT_PATH_PLACEHOLDER"}}}
		}),
	)
	if err != nil {
		return nil, err
	}

	log.Info().
		Str("flake", flake.Name).
		Str("configuration", configuration.Name).
		Str("closure", configuration.MetaBuild.SystemClosure).
		Msgf("Built %s/%s -> %s", flake.Name, configuration.Name, parsedOutput[0].Outputs.Out)

	return parsedOutput, nil
}

type BuildOutputJson []struct {
	Outputs struct {
		Out string `json:"out"`
	} `json:"outputs"`
}

func lastNonEmptyLine(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	slices.Reverse(lines)

	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			return trimmed
		}
	}

	return []byte{}
}
