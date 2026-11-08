package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	installablepkg "github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/pkg/argv"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/pkg/errors"
)

func validateFlakes(fleet *fleet.Fleet, timeout time.Duration) error {
	var (
		mu   sync.Mutex
		errs []string
	)

	// Use the fleet-level nix config for validation commands. Per-installable
	// overrides don't apply here because validation runs before per-installable
	// config is fully exercised.
	nixCfg := &fleet.Nix

	var waitGroup sync.WaitGroup

	for _, flakePair := range fleet.Flakes.Pairs() {
		flakeURL := flakePair.Value.URL
		if flakeURL == "" {
			continue
		}

		flakeName := flakePair.Key

		waitGroup.Go(func() {
			err := checkFlakeURLExists(flakeURL, timeout, nixCfg)
			if err == nil {
				return
			}

			mu.Lock()

			if errors.Is(err, errProbeNegative) {
				errs = append(errs, fmt.Sprintf("  - flake URL '%s' does not exist or is unreachable (%s)", flakeName, flakeURL))
			} else {
				errs = append(errs, "  - "+err.Error())
			}

			mu.Unlock()
		})

		validateInstallablesExist(&waitGroup, &mu, &errs, flakePair.Value.Installables, flakeURL, flakeName, timeout, nixCfg)
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
	nixCfg *nix.NixConfig,
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
				installable.Preset,
			)
			installablePath := fmt.Sprintf("%s#%s", flakeURL, flakeInstallable)

			waitGroup.Add(1)

			go func(outputName, inst string) {
				defer waitGroup.Done()

				err := checkInstallableExists(inst, timeout, nixCfg)
				if err != nil && errors.Is(err, errProbeNegative) {
					mu.Lock()

					*errs = append(*errs, fmt.Sprintf("  - output '%s' not found in flake '%s' (%s)", outputName, flakeName, inst))
					mu.Unlock()
				} else if err != nil {
					mu.Lock()

					*errs = append(*errs, "  - "+err.Error())
					mu.Unlock()
				}
			}(nameKey, installablePath)
		}
	}
}

func checkFlakeURLExists(url string, timeout time.Duration, nixCfg *nix.NixConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	argv := flakeMetadataArgv(nixCfg, url)

	//nolint:gosec
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)

	output, err := cmd.Output()
	if err != nil {
		return probeError(err, argv)
	}

	var metadata json.RawMessage

	err = json.Unmarshal(output, &metadata)
	if err != nil {
		return errors.Errorf("flake metadata is not JSON (is %q the right command?): %v", argv[0], err)
	}

	return nil
}

// probeError separates a broken probe from a negative answer: a missing or
// unrunnable configured command would otherwise surface as "flake does not
// exist", blaming the flake for a bad prefix.
func probeError(err error, argv []string) error {
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return errors.Errorf("cannot run configured command %q: %v (check the *_command setting)", argv[0], err)
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 127 {
		return errors.Errorf("configured command %q is missing on PATH: %v", argv[0], err)
	}

	return errProbeNegative
}

var errProbeNegative = errors.New("probe returned a negative answer")

// flakeMetadataArgv is the flake-existence probe argv: the configured command
// prefix, the experimental features, then the JSON request for url.
func flakeMetadataArgv(nixCfg *nix.NixConfig, url string) []string {
	return argv.Argv(
		nixCfg.FlakeMetadataCommand,
		nixCfg.ExperimentalFeatures,
		[]string{"--json", url},
	)
}

func checkInstallableExists(installable string, timeout time.Duration, nixCfg *nix.NixConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	argv := installableEvalArgv(nixCfg, installable)

	//nolint:gosec
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)

	output, err := cmd.Output()
	if err != nil {
		return probeError(err, argv)
	}

	if strings.TrimSpace(string(output)) != "true" {
		return errProbeNegative
	}

	return nil
}

// installableEvalArgv is the installable-existence probe argv: the configured
// command prefix, the experimental features, then an eval that reduces the
// installable to true.
func installableEvalArgv(nixCfg *nix.NixConfig, installable string) []string {
	return argv.Argv(
		nixCfg.EvalCommand,
		nixCfg.ExperimentalFeatures,
		[]string{installable, "--apply", "x: true", "--json"},
	)
}
