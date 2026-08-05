package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/chelnak/ysmrr"
	"github.com/chelnak/ysmrr/pkg/colors"
	"github.com/fatih/color"
	"github.com/pkg/errors"
)

const (
	tickerInterval   = 100 * time.Millisecond
	elapsedPrecision = 10 * time.Millisecond
	minutesBase      = 60
	secondsThreshold = 10
)

var testStart time.Time

var noTTY = !isTerminal() || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"

func newSpinnerManager(discardWriter bool) ysmrr.SpinnerManager {
	if discardWriter {
		return ysmrr.NewSpinnerManager(
			ysmrr.WithCompleteCharacter("✓"),
			ysmrr.WithErrorCharacter("✗"),
			ysmrr.WithCompleteColor(colors.FgHiGreen),
			ysmrr.WithErrorColor(colors.FgHiRed),
			ysmrr.WithWriter(io.Discard),
		)
	}

	return ysmrr.NewSpinnerManager(
		ysmrr.WithCompleteCharacter("✓"),
		ysmrr.WithErrorCharacter("✗"),
		ysmrr.WithCompleteColor(colors.FgHiGreen),
		ysmrr.WithErrorColor(colors.FgHiRed),
	)
}

func startElapsedTicker(spinner *ysmrr.Spinner, startTime time.Time, msg string, cancel <-chan struct{}) {
	if noTTY {
		return
	}

	go func() {
		ticker := time.NewTicker(tickerInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				spinner.UpdateMessage(fmt.Sprintf("%s  %s", msg, formatElapsed(time.Since(startTime))))
			case <-cancel:
				return
			}
		}
	}()
}

type parallelGroup struct {
	group   pond.TaskGroup
	pool    pond.Pool
	manager ysmrr.SpinnerManager
	entries []pgEntry
	started bool
}

type pgEntry struct {
	spinner *ysmrr.Spinner
	started time.Time
	name    string
	cancel  chan struct{}
}

func newParallelGroup() *parallelGroup {
	stopSequentialMgr()

	pool := pond.NewPool(0)

	return &parallelGroup{
		group:   pool.NewGroup(),
		pool:    pool,
		manager: newSpinnerManager(noTTY),
	}
}

func (pg *parallelGroup) Go(name string, task func() error) {
	spinner := pg.manager.AddSpinner(name)
	entry := pgEntry{spinner: spinner, started: time.Now(), name: name, cancel: make(chan struct{})}
	pg.entries = append(pg.entries, entry)

	if !pg.started {
		pg.started = true
		pg.manager.Start()
	}

	startElapsedTicker(spinner, entry.started, name, entry.cancel)

	pg.group.SubmitErr(func() error {
		taskErr := task()

		close(entry.cancel)

		elapsed := formatElapsed(time.Since(entry.started))
		if taskErr != nil {
			spinner.ErrorWithMessage(fmt.Sprintf("%s  %s", name, elapsed))
		} else {
			spinner.CompleteWithMessage(fmt.Sprintf("%s  %s", name, elapsed))
		}

		return taskErr
	})
}

func (pg *parallelGroup) Wait() error {
	err := pg.group.Wait()
	if pg.started {
		pg.manager.Stop()
	}

	if noTTY {
		for _, entry := range pg.entries {
			elapsed := formatElapsed(time.Since(entry.started))
			if entry.spinner.IsError() {
				fmt.Printf("  ✗ %s  %s\n", entry.name, elapsed)
			} else {
				fmt.Printf("  ✓ %s  %s\n", entry.name, elapsed)
			}
		}
	}

	pg.pool.Stop()

	return errors.Wrap(err, "parallel group wait")
}

type stepTimer struct {
	started time.Time
	msg     string
	spinner *ysmrr.Spinner
	cancel  chan struct{}
}

var seqMgr struct {
	manager ysmrr.SpinnerManager
	active  bool
}

func stopSequentialMgr() {
	if seqMgr.active {
		seqMgr.manager.Stop()
		seqMgr.active = false
	}
}

func startStep(format string, args ...any) *stepTimer {
	msg := fmt.Sprintf(format, args...)

	if !seqMgr.active {
		seqMgr.manager = newSpinnerManager(noTTY)
		seqMgr.manager.Start()
		seqMgr.active = true
	}

	spinner := seqMgr.manager.AddSpinner(msg)
	cancel := make(chan struct{})
	startTime := time.Now()
	startElapsedTicker(spinner, startTime, msg, cancel)

	return &stepTimer{started: startTime, msg: msg, spinner: spinner, cancel: cancel}
}

func (st *stepTimer) Done() {
	close(st.cancel)

	elapsed := formatElapsed(time.Since(st.started))
	st.spinner.CompleteWithMessage(fmt.Sprintf("%s  %s", st.msg, elapsed))

	if noTTY {
		fmt.Printf("  ✓ %s  %s\n", st.msg, elapsed)
	}
}

func (st *stepTimer) Fail(stepErr error) {
	close(st.cancel)

	elapsed := formatElapsed(time.Since(st.started))
	st.spinner.ErrorWithMessage(fmt.Sprintf("%s  FAILED (%s): %s", st.msg, elapsed, stepErr))

	if noTTY {
		fmt.Printf("  ✗ %s  FAILED (%s): %s\n", st.msg, elapsed, stepErr)
	}
}

func printPhasef(format string) {
	stopSequentialMgr()

	bold := color.New(color.Bold).SprintFunc()
	fmt.Printf("\n%s\n", bold(format))
}

func printFinalf(format string, args ...any) {
	stopSequentialMgr()

	elapsed := formatElapsed(time.Since(testStart))
	green := color.New(color.FgHiGreen, color.Bold).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()
	fmt.Printf("\n  %s %s  %s\n", green("✓"), bold(fmt.Sprintf(format, args...)), bold(elapsed))
}

func failAndExit(exitErr error) {
	stopSequentialMgr()
	cleanupFifos()

	elapsed := formatElapsed(time.Since(testStart))
	red := color.New(color.FgHiRed, color.Bold).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()
	fmt.Fprintf(os.Stderr, "\n  %s FAILED  %s\n    %s\n", red("✗"), bold(elapsed), exitErr)
	os.Exit(1)
}

func formatElapsed(duration time.Duration) string {
	duration = duration.Truncate(elapsedPrecision)
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}

	if duration < time.Minute {
		seconds := duration.Seconds()
		if seconds < float64(secondsThreshold) {
			return fmt.Sprintf("%.1fs", seconds)
		}

		return fmt.Sprintf("%.0fs", seconds)
	}

	minutes := int(duration.Minutes())
	seconds := int(duration.Seconds()) - minutes*minutesBase

	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

func isTerminal() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return fileInfo.Mode()&os.ModeCharDevice != 0
}
