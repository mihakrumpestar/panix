package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/pkg/errors"
)

const nixFlakeValidationTimeout = 60 * time.Second

var nixExperimentalFeatures = []string{"--extra-experimental-features", "nix-command flakes"}

func validateFlakes(fleet *fleet.Fleet) error {
	var (
		mu   sync.Mutex
		errs []string
	)

	var waitGroup sync.WaitGroup

	for _, flakePair := range fleet.Flakes.Pairs() {
		flakeURL := flakePair.Value.URL
		if flakeURL == "" {
			continue
		}

		flakeName := flakePair.Key

		waitGroup.Go(func() {
			if !checkFlakeURLExists(flakeURL) {
				mu.Lock()

				errs = append(errs, fmt.Sprintf("  - flake URL '%s' does not exist or is unreachable (%s)", flakeName, flakeURL))
				mu.Unlock()
			}
		})

		for _, configPair := range flakePair.Value.Configurations.Pairs() {
			attrPath := strings.ReplaceAll(configPair.Value.FlakeOutput.String(), "<name>", configPair.Key)
			installable := fmt.Sprintf("%s#%s", flakeURL, attrPath)

			waitGroup.Add(1)

			go func(configName, inst string) {
				defer waitGroup.Done()

				if !checkFlakeOutputExists(inst) {
					mu.Lock()

					errs = append(errs, fmt.Sprintf("  - configuration '%s' output not found in flake '%s' (%s)", configName, flakeName, inst))
					mu.Unlock()
				}
			}(configPair.Key, installable)
		}
	}

	waitGroup.Wait()

	if len(errs) > 0 {
		return errors.New("configuration validation errors:\n" + strings.Join(errs, "\n"))
	}

	return nil
}

func checkFlakeURLExists(url string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), nixFlakeValidationTimeout)
	defer cancel()

	//nolint:gosec
	cmd := exec.CommandContext(ctx, "nix",
		slices.Concat(
			nixExperimentalFeatures,
			[]string{"flake", "metadata", "--json", url},
		)...)

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	var metadata json.RawMessage

	return json.Unmarshal(output, &metadata) == nil
}

func checkFlakeOutputExists(installable string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), nixFlakeValidationTimeout)
	defer cancel()

	//nolint:gosec
	cmd := exec.CommandContext(ctx, "nix",
		slices.Concat(
			nixExperimentalFeatures,
			[]string{"eval", installable, "--apply", "x: true", "--json"},
		)...)

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) == "true"
}
