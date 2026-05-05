package profile

import (
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

//nolint:lll
type Profile struct {
	CPU       string `yaml:"cpu" json:"cpu,omitempty" help:"Path for CPU profile output (enables CPU profiling)" validate:"omitempty,filepath"`
	Mem       string `yaml:"mem" json:"mem,omitempty" help:"Path for memory profile output (enables memory profiling)" validate:"omitempty,filepath"`
	Block     string `yaml:"block" json:"block,omitempty" help:"Path for block profile output (enables block profiling)" validate:"omitempty,filepath"`
	Mutex     string `yaml:"mutex" json:"mutex,omitempty" help:"Path for mutex profile output (enables mutex profiling)" validate:"omitempty,filepath"`
	Goroutine string `yaml:"goroutine" json:"goroutine,omitempty" help:"Path for goroutine profile output (enables goroutine profiling)" validate:"omitempty,filepath"`
}

type StopFunc func()

func Start(conf Profile) (StopFunc, error) {
	var stops []func()

	stopFunc := func() {
		for _, stop := range stops {
			stop()
		}
	}

	if conf.CPU != "" {
		err := startCPU(conf.CPU, &stops)
		if err != nil {
			return nil, err
		}
	}

	if conf.Mem != "" {
		stops = append(stops, stopMem(conf.Mem))
	}

	if conf.Block != "" {
		runtime.SetBlockProfileRate(1)

		stops = append(stops, stopRuntimeProfile("block", conf.Block, func() { runtime.SetBlockProfileRate(0) }))
	}

	if conf.Mutex != "" {
		runtime.SetMutexProfileFraction(1)

		stops = append(stops, stopRuntimeProfile("mutex", conf.Mutex, func() { runtime.SetMutexProfileFraction(0) }))
	}

	if conf.Goroutine != "" {
		stops = append(stops, stopRuntimeProfile("goroutine", conf.Goroutine, nil))
	}

	return stopFunc, nil
}

func startCPU(path string, stops *[]func()) error {
	file, err := os.Create(path) //nolint:gosec // Path comes from controlled configuration flag
	if err != nil {
		return errors.Wrap(err, "failed to create CPU profile file")
	}

	err = pprof.StartCPUProfile(file)
	if err != nil {
		_ = file.Close()

		return errors.Wrap(err, "failed to start CPU profile")
	}

	*stops = append(*stops, func() {
		pprof.StopCPUProfile()

		closeErr := file.Close()
		if closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close CPU profile file")
		}
	})

	return nil
}

func stopMem(path string) func() {
	return func() {
		runtime.GC()

		file, err := os.Create(path) //nolint:gosec // Path comes from controlled configuration flag
		if err != nil {
			log.Error().Err(err).Msg("failed to create memory profile file")

			return
		}

		err = pprof.WriteHeapProfile(file)

		closeErr := file.Close()
		if closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close memory profile file")
		}

		if err != nil {
			log.Error().Err(err).Msg("failed to write memory profile")
		}
	}
}

func stopRuntimeProfile(name, path string, disable func()) func() {
	return func() {
		if disable != nil {
			disable()
		}

		err := writeProfile(name, path)
		if err != nil {
			log.Error().Err(err).Msgf("failed to write %s profile", name)
		}
	}
}

func writeProfile(name, path string) error {
	prof := pprof.Lookup(name)
	if prof == nil {
		return errors.Errorf("profile %q not found", name)
	}

	file, err := os.Create(path) //nolint:gosec // Path comes from controlled configuration flag
	if err != nil {
		return errors.Wrapf(err, "failed to create %s profile file", name)
	}

	err = prof.WriteTo(file, 0)

	closeErr := file.Close()
	if closeErr != nil {
		log.Error().Err(closeErr).Msgf("failed to close %s profile file", name)
	}

	if err != nil {
		return errors.Wrapf(err, "failed to write %s profile", name)
	}

	return nil
}
