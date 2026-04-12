package fleet

import (
	"iter"
	"strconv"

	config_attributes "github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tables"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/stats"
	"github.com/mihakrumpestar/panix/internal/pkg/orderedmap"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

type Fleet struct {
	config_attributes.Attributes `yaml:",inline"`

	Flakes orderedmap.OrderedMap[string, *flake.Flake] `yaml:"flakes,required" json:"flakes"`

	// Internal
	Logs           *logs.Logs                `yaml:"-" json:"logs,omitempty"`
	statsCache     *stats.StatisticsPerPhase `yaml:"-" json:"-"`
	statsTableData *tables.StatsTable        `yaml:"-" json:"-"`
}

func (r *Fleet) Init(f *flags.Flags) error {
	err := r.Attributes.Init("fleet", &config_attributes.Attributes{Flags: f}, false)
	if err != nil {
		return errors.Wrap(err, "failed to initialize fleet attributes")
	}

	r.Logs.PhaseLogs, err = phase.NewPhaseLogs(r.Xpath)
	if err != nil {
		return errors.Wrap(err, "failed to initialize fleet logs")
	}

	return nil
}

func (f *Fleet) CalculateDurationAndError(workflowPhases []phases.Phase) logs.DurationAndError {
	f.Logs.CalculateDurationAndError(workflowPhases)

	f.statsCache = stats.NewStatisticsPerPhase()

	for _, treeLeaf := range f.AllMachines() {
		machine := treeLeaf.Machine

		ms := machine.ComputeMachineState(workflowPhases)
		if ms.Status == "" {
			continue
		}

		f.statsCache.Add(ms.Phase, ms.Status, machine.Xpath)
	}

	return f.cache
}

func (f *Fleet) ComputeStatisticsPerPhase() *stats.StatisticsPerPhase {
	return f.statsCache
}

func (f *Fleet) CollectStatsTableData() {
	rows := make([]log.MachineRow, 0)
	xpaths := make([]config_attributes.Xpath, 0)

	f.IterateMachines(func(machine *machine.Machine) {
		configuration := machine.ParentConfiguration
		flake := configuration.ParentFlake

		xpaths = append(xpaths, machine.Xpath)

		ms := machine.GetMachineState()

		var arch, generation, date, nixos, kernel string
		if mi := machine.MetaInspect.Load(); mi != nil {
			arch = mi.Architecture
			if mi.Generations != nil {
				generation = strconv.FormatUint(uint64(mi.Generations.Current), 10)

				for _, gen := range mi.Generations.Generations {
					if gen.Number == mi.Generations.Current {
						date = gen.Date
						nixos = gen.Nixos
						kernel = gen.Kernel
						break
					}
				}
			}
		}

		rows = append(rows, tables.MachineRow{
			Xpath:        machine.Xpath.String(),
			FlakeName:    flake.Name,
			ConfigName:   configuration.Name,
			MachineName:  machine.Name,
			Status:       string(ms.Status),
			Phase:        string(ms.Phase),
			Architecture: arch,
			Generation:   generation,
			Date:         date,
			Nixos:        nixos,
			Kernel:       kernel,
		})
	})

	f.statsTableData = &tables.StatsTable{
		Rows:          rows,
		MachineXpaths: xpaths,
	}
}

func (f *Fleet) ResetState() {
	f.Logs.ClearLogs()
	f.statsCache = nil
	f.statsTableData = nil

	for _, flakeP := range f.Flakes.Pairs() {
		flakeV := flakeP.Value

		flakeV.Logs.ClearLogs()

		for _, configurationP := range flakeV.Configurations.Pairs() {
			configurationV := configurationP.Value

			configurationV.Logs.ClearLogs()

			for _, machineP := range configurationV.Machines.Pairs() {
				machineV := machineP.Value

				machineV.Logs.ClearLogs()
				machineV.MetaInspect.Set(&machine.MetaInspect{})
				machineV.State.ActiveSSH = machine.SSHTypeRegular
			}
		}
	}
}

// Helpers

type TreeLeaf struct {
	Flake         *flake.Flake
	Configuration *configuration.Configuration
	Machine       *machine.Machine
}

func (f *Fleet) AllMachines() iter.Seq2[int, TreeLeaf] {
	return func(yield func(int, TreeLeaf) bool) {
		i := 0
		for _, flake := range f.Flakes.Pairs() {
			for _, configuration := range flake.Value.Configurations.Pairs() {
				for _, machine := range configuration.Value.Machines.Pairs() {
					if !yield(i, TreeLeaf{flake.Value, configuration.Value, machine.Value}) {
						return
					}

					i++
				}
			}
		}
	}
}
