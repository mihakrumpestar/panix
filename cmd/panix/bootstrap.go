package panix

import (
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/spf13/cobra"
)

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Explicit bootstrap phase",
	Long: `Bootstrap initializes uninitialized machines.
This command will:
- Check which machines need bootstrapping
- Copy pre-bootstrap secrets if configured
- Run nixos-anywhere to install NixOS
- Wait for machines to reboot and become available

Same as "deploy --bootstrap.only".`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		conf := ConfFromContext(ctx)

		conf.Flags.Bootstrap.Only = true

		phases := phases.PhasesInOrder()

		workflowExec, err := workflow.NewWorkflow(ctx, conf, phases)
		if err != nil {
			return err
		}

		return RunTUI(workflowExec)
	},
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
}
