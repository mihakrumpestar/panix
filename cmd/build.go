package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/spf13/cobra"
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build all selected closures",
	Long:  `Build compiles the NixOS configurations for all selected machines without deploying them.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		machines := getFilteredMachines()
		if len(machines) == 0 {
			return fmt.Errorf("no machines match the specified filters")
		}
		fmt.Printf("Building configurations for %d machines...\n", len(machines))
		if config.C.Global.DryRun {
			fmt.Println("DRY RUN: Would build the following configurations:")
			for _, m := range machines {
				fmt.Printf("  - %s: %s\n", m.Name, m.FlakeOutput)
			}
			return nil
		}
		results := buildMachines(machines)
		var failed int
		for _, result := range results {
			if result.Errors != nil {
				fmt.Printf("%s failed: %v\n", result.Name, result.Errors)
				failed++
			} else {
				fmt.Printf("%s built: %s\n", result.Name, result.FlakeBuildOutputPath)
			}
		}
		if failed > 0 {
			return fmt.Errorf("%d builds failed", failed)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

// buildMachines runs nix build for each machine and returns results
func buildMachines(machines []config.MachineConfig) []config.MachineConfig {
	results := make([]config.MachineConfig, 0)

	for _, machine := range machines {
		abs, err := filepath.Abs(machine.FlakePath)
		if err != nil {
			machine.Errors = err
			continue
		}

		nixRef := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", abs, machine.FlakeOutput)
		cmd := exec.Command("nix", "build", "--no-link", "--no-update-lock-file", "--json", "path:"+nixRef)
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			machine.Errors = fmt.Errorf("%w: %s", err, errBuf.String())
			continue
		}
		var nr []struct {
			Outputs struct {
				Out string `json:"out"`
			} `json:"outputs"`
		}
		if err := json.Unmarshal(outBuf.Bytes(), &nr); err != nil || len(nr) == 0 {
			machine.Errors = fmt.Errorf("invalid build output: %s", outBuf.String())
			continue
		}

		machine.FlakeBuildOutputPath = nr[0].Outputs.Out

		results = append(results, machine)
	}

	return results
}
