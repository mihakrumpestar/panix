package workflow

import (
	"fmt"
	"slices"
	"strings"

	"github.com/alitto/pond/v2"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
)

type StatusMetadatas struct {
	Statuses []StatusMetadata
	Err      error
}

type StatusMetadata struct {
	MetadataID
	Reachable         bool
	SSHConnectable    bool
	Bootstrapped      bool
	CurrentGeneration string
	LastDeployTime    string
	CommandOutputs    []executioner.ExecutionerOutput
	Finished          bool
	Error             error
}

// executeStatusPhase runs status checks in parallel across all machines
// and must complete fully before proceeding to next phase
func (w *WorkflowExecutor) ExecuteStatusPhase() <-chan StatusMetadatas {
	out := make(chan StatusMetadatas)

	sms := StatusMetadatas{
		Statuses: make([]StatusMetadata, 0),
	}

	if w.cfg.Global.Verbose {
		fmt.Println("Executing status phase across all machines")
	}

	pool := pond.NewPool(w.cfg.Global.Concurrency)
	group := pool.NewGroupContext(w.ctx)

	forAllMachines(w.cfg.Flakes, func(i int, flakeName, configurationName, machineName string, machine *config.Machine) {
		group.Submit(func() {

			// initialize sm with MetadataID
			sm := StatusMetadata{
				MetadataID: MetadataID{
					FlakeName:         flakeName,
					ConfigurationName: configurationName,
					MachineName:       machineName,
				},
			}

			wp := WorkflowExecutorForConfigurationAndMachine{w.ctx, &w.cfg.Global}
			updates := wp.executeStatusPhaseMachineStreams(machine, sm)
			sms.Statuses = append(sms.Statuses, sm)

			var lastUpdate StatusMetadata
			for update := range updates {
				lastUpdate = update
				sms.Statuses[i] = lastUpdate
				out <- sms
			}

			lastUpdate.Finished = true
			sms.Statuses[i] = lastUpdate
			out <- sms

			if w.cfg.Global.RequireAllSuccess && lastUpdate.Error != nil {
				sms.Err = fmt.Errorf("status phase failed because of 'RequireAllSuccess': %v", lastUpdate.Error)
				out <- sms
				return
			}
		})
	})

	// Stop the pool and wait for all submitted tasks to complete

	go func() {
		defer close(out)

		_ = group.Wait()

		if !slices.ContainsFunc(sms.Statuses, func(sm StatusMetadata) bool { return sm.Error == nil }) {
			sms.Err = fmt.Errorf("status phase failed because of all machines failed status phase")
			out <- sms
		}

		w.PrintStatusPhaseMachineTable(sms.Statuses)
	}()

	return out
}

// executeStatusPhaseMachineStreams returns a read-only channel of StatusMetadata.
// Each time a field on sm changes (or an error occurs), an updated copy
// of sm is sent.  The channel is closed as soon as we finish or hit an unrecoverable error.
func (w *WorkflowExecutorForConfigurationAndMachine) executeStatusPhaseMachineStreams(
	machine *config.Machine, sm StatusMetadata) <-chan StatusMetadata {
	ch := make(chan StatusMetadata)

	go func() {
		defer close(ch)
		send := func() { ch <- sm }
		sendWithError := func(err error) {
			sm.Error = err
			fmt.Printf("\n\nERR: %s\n\n", err.Error())
			send()
		}
		send() // initial state

		// create executor
		exc, err := executioner.New(w.ctx, w.cfg.DryRun, machine)
		if err != nil {
			sendWithError(fmt.Errorf("failed to create executor: %w", err))
			return
		}

		mkStep := func(
			stream <-chan executioner.ExecutionerOutput,
			onError func(out executioner.ExecutionerOutput),
			onSuccess func(out executioner.ExecutionerOutput),
		) executioner.ExecStep {
			return executioner.ExecStep{
				Stream: stream,
				OnInit: func() {
					sm.CommandOutputs = append(sm.CommandOutputs, executioner.ExecutionerOutput{})
				},
				OnEvent: func(out executioner.ExecutionerOutput) {
					last := len(sm.CommandOutputs) - 1
					sm.CommandOutputs[last] = out
					send()
				},
				OnError: func(out executioner.ExecutionerOutput) {
					last := len(sm.CommandOutputs) - 1
					sm.CommandOutputs[last] = out
					onError(out)
					send()
				},
				OnSuccess: func(out executioner.ExecutionerOutput) {
					last := len(sm.CommandOutputs) - 1
					sm.CommandOutputs[last] = out
					onSuccess(out)
					send()
				},
			}
		}

		exc.ExecBatch(
			// 1) Ping (TCP reachability)
			mkStep(
				exc.PingStream(),
				func(out executioner.ExecutionerOutput) {
					sendWithError(fmt.Errorf("unreachable: %w\n%s", out.Error, out.Stderr.String()))
				},
				func(_ executioner.ExecutionerOutput) {
					sm.Reachable = true
				},
			),

			// SSH connectivity
			mkStep(
				exc.Exec("exit 0"),
				func(out executioner.ExecutionerOutput) {
					sendWithError(fmt.Errorf("ssh failed: %w\n%s", out.Error, out.Stderr.String()))
				},
				func(_ executioner.ExecutionerOutput) {
					sm.SSHConnectable = true
				},
			),

			// Bootstrap detection
			mkStep(
				exc.Exec("test -e /run/current-system"),
				func(_ executioner.ExecutionerOutput) {
					// not bootstrapped → stop, but not an error
				},
				func(_ executioner.ExecutionerOutput) {
					sm.Bootstrapped = true
				},
			),

			// Current generation
			mkStep(
				exc.Exec("nixos-rebuild list-generations | tail -1 | awk '{print $1}'"),
				func(out executioner.ExecutionerOutput) {
					sendWithError(fmt.Errorf("generation lookup failed: %w\n%s", out.Error, out.Stderr.String()))
				},
				func(out executioner.ExecutionerOutput) {
					sm.CurrentGeneration = strings.TrimSpace(out.Stdout.String())
				},
			),

			// Last deploy time
			mkStep(
				exc.Exec("stat -c %Y /run/current-system 2>/dev/null | xargs -I {} date -d @{} '+%Y-%m-%d %H:%M:%S' || echo 'unknown'"),
				func(out executioner.ExecutionerOutput) {
					sendWithError(fmt.Errorf("failed to get last deploy time: %w\n%s", out.Error, out.Stderr.String()))
				},
				func(out executioner.ExecutionerOutput) {
					sm.LastDeployTime = strings.TrimSpace(out.Stdout.String())
				},
			),
		)
	}()

	return ch
}

func (w *WorkflowExecutor) PrintStatusPhaseMachineTable(sms []StatusMetadata) {
	if w.cfg.Global.DryRun {
		if w.cfg.Global.Verbose {
			fmt.Println("No status table when dry-run option is enabled")
		}
		return
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("INDEX", "ICON", "FLAKE", "CONFIGURATION", "MACHINE" /* "HOST", */, "STATUS", "GENERATION", "LAST_DEPLOY", "ERROR")

	for i, sm := range sms {
		err := ""
		if sm.Error != nil {
			err = sm.Error.Error()
		}

		t.Row(
			fmt.Sprintf("%d", i),
			sm.getStatusIcon(),
			sm.FlakeName,
			sm.ConfigurationName,
			sm.MachineName,
			//machine.Ssh.Alias,
			sm.getStatusText(),
			sm.CurrentGeneration,
			sm.LastDeployTime,
			err,
		)
	}

	fmt.Println(t)
}

func (s *StatusMetadata) getStatusIcon() string {
	if !s.Finished {
		return spinner.New().View()
	}
	if !s.Reachable {
		return "🔴"
	}
	if !s.SSHConnectable {
		return "🟡"
	}
	if !s.Bootstrapped {
		return "🟠"
	}
	return "✅"
}

func (s *StatusMetadata) getStatusText() string {
	if !s.Reachable {
		return "UNREACHABLE"
	}
	if !s.SSHConnectable {
		return "SSH_FAILED"
	}
	if !s.Bootstrapped {
		return "NOT_BOOTSTRAPPED"
	}
	return "OK"
}
