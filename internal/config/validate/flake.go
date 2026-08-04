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

func validateFlakes(fleet *fleet.Fleet, timeout time.Duration) error {
	var (
		mu   sync.Mutex
		errs []string
	)

	// Use the fleet-level experimental features config for validation commands.
	// Per-installable overrides don't apply here because validation runs before
	// per-installable config is fully exercised.
	experimentalFeatures := fleet.Nix.GetExperimentalFeatures()

	var waitGroup sync.WaitGroup

	for _, flakePair := range fleet.Flakes.Pairs() {
		flakeURL := flakePair.Value.URL
		if flakeURL == "" {
			continue
		}

		flakeName := flakePair.Key

		waitGroup.Go(func() {
			if !checkFlakeURLExists(flakeURL, timeout, experimentalFeatures) {
				mu.Lock()

				errs = append(errs, fmt.Sprintf("  - flake URL '%s' does not exist or is unreachable (%s)", flakeName, flakeURL))
				mu.Unlock()
			}
		})

		validateInstallablesExist(&waitGroup, &mu, &errs, flakePair.Value.Installables, flakeURL, flakeName, timeout, experimentalFeatures)
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
	timeout time.Duration,
	experimentalFeatures []string,
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

				if !checkInstallableExists(inst, timeout, experimentalFeatures) {
					mu.Lock()

					*errs = append(*errs, fmt.Sprintf("  - output '%s' not found in flake '%s' (%s)", outputName, flakeName, inst))
					mu.Unlock()
				}
			}(nameKey, installablePath)
		}
	}
}

func checkFlakeURLExists(url string, timeout time.Duration, experimentalFeatures []string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	//nolint:gosec
	cmd := exec.CommandContext(ctx, "nix",
		slices.Concat(
			experimentalFeatures,
			[]string{"flake", "metadata", "--json", url},
		)...)

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	var metadata json.RawMessage

	return json.Unmarshal(output, &metadata) == nil
}

func checkInstallableExists(installable string, timeout time.Duration, experimentalFeatures []string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	//nolint:gosec
	cmd := exec.CommandContext(ctx, "nix",
		slices.Concat(
			experimentalFeatures,
			[]string{"eval", installable, "--apply", "x: true", "--json"},
		)...)

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) == "true"
}
