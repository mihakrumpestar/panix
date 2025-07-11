package panix

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Do full workflow (preflight→bootstrap→secrets→build→push→activate)",
	Long: `Deploy performs the complete deployment workflow:
1. Preflight checks (SSH connectivity, bootstrap status)
2. Bootstrap uninitialized machines (if needed)
3. Deploy secrets to all machines
4. Build NixOS configurations
5. Transfer closures to target machines
6. Activate new configurations

This is the main command for deploying NixOS configurations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		machines := getFilteredMachines()
		if len(machines) == 0 {
			return fmt.Errorf("no machines match the specified filters")
		}

		fmt.Printf("Deploying to %d machines...\n", len(machines))

		executor := workflow.NewWorkflowExecutor(cmd.Context(), &config.C.Global, machines)
		opts := workflow.WorkflowOptions{
			DryRun:  config.C.Global.DryRun,
			Verbose: config.C.Global.Verbose,
			Phases: []workflow.WorkflowPhase{
				workflow.PhasePreflight,
				workflow.PhaseBootstrap,
				workflow.PhaseSecrets,
				workflow.PhaseBuild,
				workflow.PhaseTransfer,
				workflow.PhaseActivate,
			},
		}

		return executor.Execute(opts)
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
}
