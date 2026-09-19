package phaseops

import (
	"fmt"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/pkg/errors"
)

// RollbackActivationMode returns the preset's default activation mode, or an
// empty string when unset: no mode argument is then passed to the activation
// script.
func RollbackActivationMode(preset installable.Preset) string {
	return preset.ActivationDefaultMode
}

// FindGenerationClosure resolves a generation link's closure path via readlink,
// wrapped as the installable's target user via WrapAsTargetUser.
func FindGenerationClosure(
	exc *executioner.Executioner,
	machine *machine.Machine,
	preset installable.Preset,
	targetUser string,
	profilePath string,
	generation uint,
) (string, error) {
	var closurePath string

	generationLink := fmt.Sprintf("%s-%d-link", profilePath, generation)

	err := exc.Exec(
		"find generation closure",
		"finding generation closure path",
		"failed to find generation closure",
		WrapAsTargetUser(machine, preset, targetUser, []string{"readlink", generationLink}),
		executioner.OnSuccess(func(log *command.CommandLog) error {
			closurePath = strings.TrimSpace(log.Output.String())

			if closurePath == "" {
				return errors.New("generation closure path is empty")
			}

			return nil
		}),
		executioner.OnDryRun(func() {
			closurePath = "/nix/store/dry-run-closure"
		}),
	)
	if err != nil {
		return "", err //nolint:wrapcheck // error is pre-annotated with statusIfFailed
	}

	return closurePath, nil
}
