package buildlogs

import (
	"os"
	"strconv"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/internal/tui/phaseflow"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

var allPhases = phase.PhasesInOrder()

func Benchmark__View_1x3x1(b *testing.B)  { benchView(b, 1, 3, 1) }
func Benchmark__View_1x3x4(b *testing.B)  { benchView(b, 1, 3, 4) }
func Benchmark__View_2x16x4(b *testing.B) { benchView(b, 2, 16, 4) }

func benchView(b *testing.B, flakesCount, configsCount, machinesN int) {
	b.Helper()

	conf := makeTestConfig(flakesCount, configsCount, machinesN, colorscheme.DefaultColorScheme())
	st := statstable.New(conf.Fleet, conf.ColorScheme)
	ps := phaseflow.New(conf.Fleet, conf.ColorScheme, conf.Phases)
	buildLogs := New(conf, st, ps)
	viewportsInst := viewports.New(&viewports.Dimensions{Width: 200, Height: 80}, conf)
	spinnersInst := spinners.New(conf.ColorScheme.Spinner)

	b.ResetTimer()

	for b.Loop() {
		_ = buildLogs.View(viewportsInst, spinnersInst)
	}
}

func makeTestConfig(flakesCount, configsCount, machinesN int, colorScheme *colorscheme.ColorScheme) *config.Config {
	if colorScheme == nil {
		colorScheme = colorscheme.DefaultColorScheme()
	}

	flakesMap := atomicorderedmap.New[string, *flake.Flake]()

	for flakeIdx := range flakesCount {
		flakeObj := &flake.Flake{}
		flakeObj.Logs = logs.New()
		flakeObj.Configurations = atomicorderedmap.New[string, *configuration.Configuration]()

		for confIdx := range configsCount {
			cfg := &configuration.Configuration{}
			cfg.Logs = logs.New()
			cfg.Machines = atomicorderedmap.New[string, *machine.Machine]()

			for machIdx := range machinesN {
				var mach *machine.Machine

				switch machIdx % 4 {
				case 0:
					mach = newTestMachine(allPhases, true)
				case 1:
					mach = newTestMachine(allPhases, false)
				case 2:
					mach = newTestMachineWithOutput(allPhases)
				case 3:
					mach = newTestMachineWithError(allPhases)
				}

				cfg.Machines.Set("m"+strconv.Itoa(machIdx), mach)
			}

			flakeObj.Configurations.Set("cfg"+strconv.Itoa(confIdx), cfg)
		}

		flakesMap.Set("flake"+strconv.Itoa(flakeIdx), flakeObj)
	}

	return &config.Config{
		ColorScheme: colorScheme,
		Fleet: &fleet.Fleet{
			Flakes: flakesMap,
		},
		Phases: allPhases,
	}
}

func newTestMachine(phases []phase.Phase, running bool) *machine.Machine {
	mach := &machine.Machine{}
	mach.Logs = logs.New()

	for _, phaseI := range phases {
		phaseLog := mach.Logs.PhaseLogs.GetOrCreate(phaseI)
		phaseLog.TimeAndState.StartTimer()

		if running {
			continue
		}

		phaseLog.TimeAndState.EndTimerWithError(nil)

		cmd := phaseLog.NewCommand(describe(phaseI), "in progress...", "failed", []string{"nix", string(phaseI)}, nil)
		cmd.TimeAndState.StartTimer()
		cmd.TimeAndState.EndTimerWithError(nil)
		cmd.Output.WriteLineString("done in 0.5s")
	}

	return mach
}

func newTestMachineWithOutput(phases []phase.Phase) *machine.Machine {
	mach := &machine.Machine{}
	mach.Logs = logs.New()

	for _, phaseI := range phases {
		phaseLog := mach.Logs.PhaseLogs.GetOrCreate(phaseI)
		phaseLog.TimeAndState.StartTimer()

		cmd := phaseLog.NewCommand(describe(phaseI), "in progress...", "failed", []string{"nix", string(phaseI)}, nil)
		cmd.TimeAndState.StartTimer()
		cmd.Output.WriteLineString("output line 1")
		cmd.Output.WriteLineString("output line 2")
		cmd.Output.WriteLineString("output line 3")
	}

	return mach
}

func newTestMachineWithError(phases []phase.Phase) *machine.Machine {
	mach := &machine.Machine{}
	mach.Logs = logs.New()

	for idx, phaseI := range phases {
		phaseLog := mach.Logs.PhaseLogs.GetOrCreate(phaseI)
		phaseLog.TimeAndState.StartTimer()
		phaseLog.TimeAndState.EndTimerWithError(os.ErrNotExist)

		cmd := phaseLog.NewCommand(describe(phaseI), "in progress...", "failed", []string{"nix", string(phaseI)}, nil)
		cmd.TimeAndState.StartTimer()
		cmd.TimeAndState.EndTimerWithError(os.ErrNotExist)

		if idx == 0 {
			cmd.Output.WriteLineString("error: something went wrong")
		}
	}

	return mach
}

func describe(phaseI phase.Phase) string {
	switch phaseI {
	case phase.Inspect:
		return "inspect: check SSH reachability"
	case phase.Build:
		return "nix build .#nixosConfigurations.machine.config.system.build.toplevel"
	case phase.Bootstrap:
		return "bootstrap: initial provisioning"
	case phase.Transfer:
		return "nix copy --to ssh-ng://machine"
	case phase.Secrets:
		return "transfer secrets"
	case phase.Activate:
		return "nixos-rebuild switch"
	case phase.Rollback:
		return "rollback to previous generation"
	default:
		return string(phaseI)
	}
}
