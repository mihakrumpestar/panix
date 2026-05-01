package buildlogs

import (
	"os"
	"strconv"
	"strings"
	"testing"

	lptree "charm.land/lipgloss/v2/tree"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	tuittree "github.com/mihakrumpestar/panix/internal/pkg/tui/tree"
	"github.com/mihakrumpestar/panix/internal/tui/phasestatus"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
)

func TestMain(m *testing.M) {
	zone.NewGlobal()
	os.Exit(m.Run())
}

var allPhases = phase.PhasesInOrder()

// Tree rendering comparison: lipgloss tree vs tuittree at realistic scale.
// Measures only the tree.String() cost — no viewports, no spinners, no durations.

func BenchmarkSimpleTreeComparison_1x3x1(b *testing.B)    { benchTreeComparison(b, 1, 3, 1, true) }
func BenchmarkLipglossTreeComparison_1x3x1(b *testing.B)  { benchTreeComparison(b, 1, 3, 1, false) }
func BenchmarkSimpleTreeComparison_2x16x4(b *testing.B)   { benchTreeComparison(b, 2, 16, 4, true) }
func BenchmarkLipglossTreeComparison_2x16x4(b *testing.B) { benchTreeComparison(b, 2, 16, 4, false) }

func benchTreeComparison(b *testing.B, flakesCount, configsCount, machinesN int, useSimple bool) {
	b.Helper()

	conf := makeTestConfig(flakesCount, configsCount, machinesN, colorscheme.DefaultColorScheme())
	cs := conf.ColorScheme
	trees := buildComparisonTrees(conf, cs, useSimple)

	b.ResetTimer()

	if useSimple {
		for b.Loop() {
			for _, t := range trees.st {
				_ = t.String()
			}
		}
	} else {
		for b.Loop() {
			for _, t := range trees.lp {
				_ = t.String()
			}
		}
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
			Flakes:      flakesMap,
			StatsTable:  statstable.NewStatsTable(),
			PhaseStatus: phasestatus.NewPhaseStatus(),
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

type comparisonTrees struct {
	lp []*lptree.Tree
	st []*tuittree.Node
}

func buildComparisonTrees(conf *config.Config, colorScheme *colorscheme.ColorScheme, useSimple bool) comparisonTrees {
	var trees comparisonTrees

	for _, fp := range conf.Fleet.Flakes.Pairs() {
		flake := fp.Value
		if flake == nil {
			continue
		}

		flakeLP, flakeST := buildComparisonFlake(colorScheme, flake, useSimple)
		addComparisonConfigs(colorScheme, flake, flakeLP, flakeST, useSimple)

		if useSimple && flakeST.Length() > 0 {
			trees.st = append(trees.st, flakeST)
		} else if !useSimple && flakeLP.Children().Length() > 0 {
			trees.lp = append(trees.lp, flakeLP)
		}
	}

	return trees
}

func buildComparisonFlake(
	colorScheme *colorscheme.ColorScheme, flake *flake.Flake, useSimple bool,
) (*lptree.Tree, *tuittree.Node) {
	if useSimple {
		return nil, tuittree.New().Root(fmtNode(colorScheme.Flake, flake.Name)).
			Enumerator(tuittree.EnumeratorRounded).
			EnumeratorStyle(colorScheme.Tree.Enumerator).
			IndenterStyle(colorScheme.Tree.Enumerator)
	}

	return lptree.New().Root(fmtNode(colorScheme.Flake, flake.Name)).
		Enumerator(lptree.RoundedEnumerator).
		EnumeratorStyle(colorScheme.Tree.Enumerator).
		IndenterStyle(colorScheme.Tree.Enumerator), nil
}

func addComparisonConfigs(
	colorScheme *colorscheme.ColorScheme, flake *flake.Flake,
	flakeLP *lptree.Tree, flakeST *tuittree.Node, useSimple bool,
) {
	for _, cp := range flake.Configurations.Pairs() {
		cfg := cp.Value
		if cfg == nil {
			continue
		}

		cfgLP, cfgST := buildComparisonCfg(colorScheme, cfg, useSimple)
		addComparisonMachines(colorScheme, cfg, cfgLP, cfgST, useSimple)

		if useSimple && cfgST.Length() > 0 {
			flakeST.Child(cfgST)
		} else if !useSimple && cfgLP.Children().Length() > 0 {
			flakeLP.Child(cfgLP)
		}
	}
}

func buildComparisonCfg(
	colorScheme *colorscheme.ColorScheme, cfg *configuration.Configuration, useSimple bool,
) (*lptree.Tree, *tuittree.Node) {
	name := fmtNode(colorScheme.Configuration, cfg.Name)
	if useSimple {
		return nil, tuittree.New().Root(name)
	}

	return lptree.New().Root(name), nil
}

func addComparisonMachines(
	colorScheme *colorscheme.ColorScheme, cfg *configuration.Configuration,
	cfgLP *lptree.Tree, cfgST *tuittree.Node, useSimple bool,
) {
	for _, mp := range cfg.Machines.Pairs() {
		mach := mp.Value
		if mach == nil {
			continue
		}

		machLP, machST := buildComparisonMachine(colorScheme, mach, useSimple)
		if mach.Logs == nil {
			continue
		}

		addComparisonPhases(colorScheme, mach, machLP, machST, useSimple)

		if useSimple {
			cfgST.Child(machST)
		} else {
			cfgLP.Child(machLP)
		}
	}
}

func buildComparisonMachine(
	colorScheme *colorscheme.ColorScheme, mach *machine.Machine, useSimple bool,
) (*lptree.Tree, *tuittree.Node) {
	name := fmtNode(colorScheme.Machine, mach.Name)
	if useSimple {
		return nil, tuittree.New().Root(name)
	}

	return lptree.New().Root(name), nil
}

func addComparisonPhases(
	colorScheme *colorscheme.ColorScheme, mach *machine.Machine,
	machLP *lptree.Tree, machST *tuittree.Node, useSimple bool,
) {
	for _, plp := range mach.Logs.PhaseLogs.Pairs() {
		phaseI := plp.Key
		phaseLabel := fmtNode(colorScheme.Phase, strings.ToUpper(phaseI.String()))
		phaseLP := lptree.New().Root(phaseLabel)
		phaseST := tuittree.New().Root(phaseLabel)

		if plp.Value.CommandLogs != nil {
			for _, cmd := range plp.Value.CommandLogs.Values() {
				if cmd == nil {
					continue
				}

				cmdLabel := fmtNode(colorScheme.Command, cmd.Description)
				cmdLP := lptree.New().Root(cmdLabel)
				cmdST := tuittree.New().Root(cmdLabel)

				if useSimple {
					phaseST.Child(cmdST)
				} else {
					phaseLP.Child(cmdLP)
				}
			}
		}

		if useSimple {
			machST.Child(phaseST)
		} else {
			machLP.Child(phaseLP)
		}
	}
}

func fmtNode(style colorscheme.ColorSchemeLogEntity, name string) string {
	return style.Color.Render(string(style.Icon) + " " + name)
}
