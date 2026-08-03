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
	installablepkg "github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
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

		validateInstallablesExist(&waitGroup, &mu, &errs, flakePair.Value.Installables, flakeURL, flakeName)
	}

	waitGroup.Wait()

	if len(errs) > 0 {
		return errors.New("configuration validation errors:\n" + strings.Join(errs, "\n"))
	}

	return nil
}

// validateInstallablesExist launches goroutines to check that each installable
// exists in the given flake URL. Errors are appended to errs under mu.
func validateInstallablesExist(
	waitGroup *sync.WaitGroup,
	mu *sync.Mutex,
	errs *[]string,
	installables *atomicorderedmap.AtomicOrderedMap[string, *atomicorderedmap.AtomicOrderedMap[string, *installablepkg.Installable]],
	flakeURL, flakeName string,
) {
	for _, typePair := range installables.Pairs() {
		typeKey := typePair.Key

		attrMap := typePair.Value
		if attrMap == nil {
			continue
		}

		for _, namePair := range attrMap.Pairs() {
			nameKey := namePair.Key

			installable := namePair.Value
			if installable == nil {
				continue
			}

			flakeInstallable := installablepkg.ResolveFlakeInstallable(
				installablepkg.FlakeOutputType(typeKey),
				installablepkg.AttributeName(nameKey),
				installable.Preset.BuildPath,
			)
			installablePath := fmt.Sprintf("%s#%s", flakeURL, flakeInstallable)

			waitGroup.Add(1)

			go func(outputName, inst string) {
				defer waitGroup.Done()

				if !checkInstallableExists(inst) {
					mu.Lock()

					*errs = append(*errs, fmt.Sprintf("  - output '%s' not found in flake '%s' (%s)", outputName, flakeName, inst))
					mu.Unlock()
				}
			}(nameKey, installablePath)
		}
	}
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

func checkInstallableExists(installable string) bool {
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
