package snapshot

import (
	"fmt"
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
)

func Restore(snap *Snapshot) (*config.Config, error) {
	conf, err := restoreConfig(snap)
	if err != nil {
		return nil, fmt.Errorf("restore config: %w", err)
	}

	initRestoredFleet(conf)

	conf.Fleet.CalculateDurationAndError(snap.Phases)

	return conf, nil
}

func restoreConfig(snap *Snapshot) (*config.Config, error) {
	cs := config.DefaultColorScheme()

	conf := &config.Config{
		Flags:        &snap.Flags,
		ColorScheme:  cs,
		Phases:       snap.Phases,
		StartTime:    time.Unix(snap.AppStartTime, 0),
		PanixVersion: snap.Version,
	}

	if snap.Fleet == nil {
		return nil, fmt.Errorf("fleet snapshot is nil")
	}

	conf.Fleet = snap.Fleet

	return conf, nil
}

func initRestoredFleet(conf *config.Config) {
	fleet := conf.Fleet
	fl := conf.Flags

	fleet.Flags = fl
	if fleet.Logs == nil {
		fleet.Logs = mustNewPhaseLogs(fleet.Xpath, fl.Logging)
	}

	for _, flake := range fleet.Flakes {
		flake.Flags = fl
		if flake.Logs == nil {
			flake.Logs = mustNewPhaseLogs(flake.Xpath, fl.Logging)
		}

		for _, cfg := range flake.Configurations {
			cfg.ParentFlake = flake
			cfg.Flags = fl
			if cfg.Logs == nil {
				cfg.Logs = mustNewPhaseLogs(cfg.Xpath, fl.Logging)
			}

			if cfg.MetaBuild != nil && cfg.MetaBuild.SystemClosure != "" {
				cfg.MetaBuild = &config.MetaBuild{
					SystemClosure: cfg.MetaBuild.SystemClosure,
				}
			}

			for _, machine := range cfg.Machines {
				machine.Flags = fl
				if machine.Logs == nil {
					machine.Logs = mustNewPhaseLogs(machine.Xpath, fl.Logging)
				}
				machine.ParentConfiguration = cfg

				restorePhaseLogs(machine.Logs, machine, fl.Logging)
			}

			restorePhaseLogs(cfg.Logs, cfg, fl.Logging)
		}

		restorePhaseLogs(flake.Logs, flake, fl.Logging)
	}

	restorePhaseLogs(fleet.Logs, fleet, fl.Logging)

	fleet.InitChildren()
}

func mustNewPhaseLogs(xpath attributes.Xpath, loggingFlags flags.Logging) *phase.PhaseLogs {
	logs, err := phase.NewPhaseLogs(xpath, loggingFlags)
	if err != nil {
		panic(fmt.Sprintf("failed to create phase logs for %s: %v", xpath.String(), err))
	}

	return logs
}

// restorePhaseLogs restores phase logs from the PhaseLogs' own JSON data.
// It calls MustGetOrCreateLog which propagates shared PhaseLog instances to children.
// Phases that were running at snapshot time are marked as finished with their start time as end time.
func restorePhaseLogs(logs *phase.PhaseLogs, logNode config.LogNode, loggingFlags flags.Logging) {
	if logs == nil {
		return
	}

	for _, pair := range logs.All() {
		p := pair.Key
		phaseLog := logNode.MustGetOrCreateLog(p)

		tas := pair.Value.TimeAndState()
		startMs := tas.GetStartTime().UnixMilli()
		endMs := tas.GetEndTime().UnixMilli()

		// If the phase was running when the snapshot was taken (has start but no end),
		// set end time to start time so it appears finished rather than still running.
		if startMs != 0 && endMs == 0 {
			endMs = startMs
		}

		cmdSnaps := make([]phase.CommandLogSnapshot, len(pair.Value.CommandLogs()))
		for i, cmd := range pair.Value.CommandLogs() {
			cmdStartMs := cmd.TimeAndState.GetStartTime().UnixMilli()
			cmdEndMs := cmd.TimeAndState.GetEndTime().UnixMilli()

			// Same treatment for commands that were running
			if cmdStartMs != 0 && cmdEndMs == 0 {
				cmdEndMs = cmdStartMs
			}

			cmdSnaps[i] = phase.CommandLogSnapshot{
				Description: cmd.Description,
				StartTime:   cmdStartMs,
				EndTime:     cmdEndMs,
				Error:       errorString(cmd.TimeAndState.GetEndError()),
				Output:      cmd.String(),
			}
		}

		phaseLog.RestoreFromSnapshot(
			startMs,
			endMs,
			errorString(tas.GetEndError()),
			cmdSnaps,
			loggingFlags,
		)
	}
}

func errorString(err error) string {
	if err != nil {
		return err.Error()
	}

	return ""
}
