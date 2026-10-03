package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// Activation Guard e2e legs (docs/design/activation-guard.md section 13, T6).
// Every guarded machine in panix.yml maps to the same VM as nixos-iso-vm
// (port 10022), so the legs share one system profile and one guard slot and
// run strictly sequentially: each leg leaves the slot terminal and the profile
// generation list unchanged for the next one.

const (
	// Testflakes fixture markers; keep in sync with the configuration-*.nix
	// fixtures.
	guardFailingMarker = "forcing activation failure" // configuration-failing.nix

	// guardSlotLogGlob is the target-side glob over the system-tier slot logs
	// (spec 10.1): one v2 slot per profile, truncated at every pre-start sweep.
	// The v2 component is the slot-layout version; legacy slots live one level
	// up and are never parsed.
	guardSlotLogGlob = "/run/panix-guard/v2/*/log"

	// e2e-short guard timeouts, mirrored from the panix.yml machine entries.
	guardActivationTimeout = 30 * time.Second

	// guardWindowSlack covers the poll granularity, the child kill, the revert
	// and panix's reporting on top of the 2x activation_timeout bound.
	guardWindowSlack = 30 * time.Second

	// guardWindowPollInterval is the slot-log mtime poll cadence.
	guardWindowPollInterval = time.Second

	// rollbackMinGenerations is the smallest generation number the standalone
	// rollback leg accepts: --gen=-1 needs a previous generation to exist.
	rollbackMinGenerations = 2
)

// guardLockMessages are the fail-fast lock indicators a losing concurrent
// deploy must carry: the slot lock probe's live-transaction pointer (spec 4.1
// step 3) or the guardian start's flock refusal.
var guardLockMessages = []string{
	"live deploy", // slot lock probe pointer
	"slot log is locked by a live transaction", // guardian start flock refusal
}

// runGuardRollbackLegs runs the Activation Guard legs right after the regular
// NixOS deploy: they need its fresh known-good generation as the rollback
// target. Leg order matters: the standalone rollback leg needs the previous
// generation the gc-fail leg later deletes, and the concurrent leg reuses the
// hang fixture the hang leg already transferred.
//
// The guarded path transfers the guardian binary from panix's embed directory
// (spec 10.3, 15): run the harness with a panix binary built by `task build`
// (populates the local-arch variant, PANIX_BIN) or the flake build (embeds
// all four variants). A plain `go run ./cmd/panix` has an empty embed dir and
// every guarded leg fails loudly at the transfer step.
func runGuardRollbackLegs(configPath string, res *testResources) error {
	err := runDeployGuardRollback(configPath, res)
	if err != nil {
		return err
	}

	err = runDeployGuardRetryAfterRevert(configPath, res)
	if err != nil {
		return err
	}

	err = runStandaloneRollback(configPath, res)
	if err != nil {
		return err
	}

	err = runDeployGuardHang(configPath, res)
	if err != nil {
		return err
	}

	err = runDeployGuardSshdKill(configPath, res)
	if err != nil {
		return err
	}

	err = runDeployGuardGcFail(configPath, res)
	if err != nil {
		return err
	}

	err = runDeployGuardConcurrent(configPath, res)
	if err != nil {
		return err
	}

	// Stage 3 legs (guard_v2.go, spec 13 additions): same VM, same slot,
	// strictly sequential. The legacy sweep runs first (it needs a terminal
	// slot to inject into), then the zero-effects precondition leg, then the
	// committed path, the mid-window injection legs, the boot-mode pair and
	// the wire-flood leg last. The user-tier leg (leg 6) runs separately after
	// the home phase (main.go): it needs guarduser's first home-manager
	// generation as its rollback target.
	err = runDeployGuardLegacySweep(configPath, res)
	if err != nil {
		return err
	}

	err = runDeployGuardFailedPrecondition(configPath, res)
	if err != nil {
		return err
	}

	err = runDeployGuardCommittedOutcome(configPath, res)
	if err != nil {
		return err
	}

	err = runDeployGuardLinkDown(configPath, res)
	if err != nil {
		return err
	}

	err = runDeployGuardianDeath(configPath, res)
	if err != nil {
		return err
	}

	err = runDeployGuardCtl(configPath, res)
	if err != nil {
		return err
	}

	err = runDeployGuardTornTail(configPath, res)
	if err != nil {
		return err
	}

	err = runDeployGuardBoot(configPath, res)
	if err != nil {
		return err
	}

	return runDeployGuardFlood(configPath, res)
}

// runDeployGuardRollback deploys the always-failing test-vm-failing config
// (rollback: magic) after the regular NixOS deploy has left a known-good
// generation, expecting activation to fail and the previous closure restored.
// Under the guard's profile-last ordering the failed activation never touches
// the profile, so the revert leaves the generation list unchanged (spec 4.1
// step 8, 4.2).
func runDeployGuardRollback(configPath string, res *testResources) error {
	printPhasef("Phase: Deploy NixOS with guard rollback")

	closureBefore, genBefore, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	fmt.Printf("  before: closure=%s generation=%d\n", closureBefore, genBefore)

	err = runPanixDeployWithArgs(configPath,
		[]string{"--tags", "test-vm-failing"},
		"PANIX_TEST_MODE=deploy",
		"PANIX_TEST_SCOPE="+string(testScopeFlag),
		"PANIX_KEXEC_PATH="+res.kexecInstallerPath,
	)
	if err == nil {
		return errors.New("expected the guarded deploy to fail")
	}

	fmt.Printf("  forced-failure deploy failed as expected: %v\n", err)

	err = assertGuardWindowRan()
	if err != nil {
		return err
	}

	err = assertGuardErrorText()
	if err != nil {
		return err
	}

	closureAfter, genAfter, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	fmt.Printf("  after: closure=%s generation=%d\n", closureAfter, genAfter)

	return assertGuardRevertRestored("guard rollback", closureBefore, genBefore, closureAfter, genAfter)
}

// runDeployGuardRetryAfterRevert immediately re-deploys the good configuration
// to the same VM through a guarded installable: the pre-start sweep must clear
// the terminal (reverted) slot the failing leg left behind, and the deploy
// must succeed (spec 9.3 step 2).
func runDeployGuardRetryAfterRevert(configPath string, res *testResources) error {
	printPhasef("Phase: Deploy NixOS guarded retry after revert")

	err := runPanixDeployStepWithArgs("Run panix deploy (guarded retry)", configPath,
		[]string{"--tags", "test-vm-retry"},
		"PANIX_TEST_MODE=deploy",
		"PANIX_TEST_SCOPE="+string(testScopeFlag),
		"PANIX_KEXEC_PATH="+res.kexecInstallerPath,
	)
	if err != nil {
		return errors.Wrap(err, "guarded retry after revert failed (slot sweep did not clear the terminal slot?)")
	}

	return nil
}

// runStandaloneRollback runs `panix rollback --gen=-1` against the test VM
// after the retry deploy succeeded: the command must succeed and the system
// profile must end up pointing at the previous generation's closure.
func runStandaloneRollback(configPath string, res *testResources) error {
	printPhasef("Phase: Standalone panix rollback")

	keyPath := res.keyPath

	genBefore, err := readSystemProfileGeneration(keyPath)
	if err != nil {
		return errors.Wrap(err, "read system generation before rollback")
	}

	// The gc-fail leg runs later and deletes non-current generations, so the
	// previous generation must still exist here (vacuous-pass guard).
	if genBefore < rollbackMinGenerations {
		return errors.Errorf("standalone rollback needs a previous generation, current=%d", genBefore)
	}

	previousClosure, err := sshRun(nixosISOPort, keyPath,
		fmt.Sprintf("readlink -f /nix/var/nix/profiles/system-%d-link", genBefore-1))
	if err != nil {
		return errors.Wrapf(err, "read closure of generation %d", genBefore-1)
	}

	previousClosure = strings.TrimSpace(previousClosure)
	if !strings.HasPrefix(previousClosure, "/nix/store/") {
		return errors.Errorf("closure of generation %d is not a store path: %q", genBefore-1, previousClosure)
	}

	fmt.Printf("  before: generation=%d previous closure=%s\n", genBefore, previousClosure)

	err = runPanixRollback(configPath, res)
	if err != nil {
		return errors.Wrap(err, "standalone rollback failed")
	}

	closureAfter, err := readSystemProfileClosure(keyPath)
	if err != nil {
		return errors.Wrap(err, "read system closure after rollback")
	}

	fmt.Printf("  after: closure=%s\n", closureAfter)

	// nix-env --set creates a new generation pointing at the target closure,
	// so only the closure identity is asserted here, not the generation
	// number.
	if closureAfter != previousClosure {
		return errors.Errorf("expected the rollback to restore closure %s, got %s", previousClosure, closureAfter)
	}

	return nil
}

// runPanixRollback runs the standalone rollback subcommand against the test VM
// (inspect + rollback phases only; no deploy).
func runPanixRollback(configPath string, res *testResources) error {
	return runPanixCommandWithArgs(panixRollbackSubcommand, configPath,
		[]string{"--tags", "test-vm", "--gen=-1"},
		"PANIX_TEST_MODE=rollback",
		"PANIX_TEST_SCOPE="+string(testScopeFlag),
		"PANIX_KEXEC_PATH="+res.kexecInstallerPath,
	)
}

// runDeployGuardHang deploys a configuration whose activation sleeps forever:
// the guardian must kill the child at the activation deadline and revert
// (spec 6.2), the machine stays reachable, and the profile stays untouched.
func runDeployGuardHang(configPath string, res *testResources) error {
	printPhasef("Phase: Deploy NixOS with hanging activation (guard deadline)")

	closureBefore, genBefore, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	fmt.Printf("  before: closure=%s generation=%d\n", closureBefore, genBefore)

	watcher := newGuardWindowWatcher(res.keyPath)

	deployStart := time.Now()

	err = runPanixDeployWithArgs(configPath,
		[]string{"--tags", "test-vm-hang"},
		"PANIX_TEST_MODE=deploy",
		"PANIX_TEST_SCOPE="+string(testScopeFlag),
		"PANIX_KEXEC_PATH="+res.kexecInstallerPath,
	)
	failTime := time.Now()

	if err == nil {
		return errors.New("expected the hanging guarded deploy to fail")
	}

	fmt.Printf("  hanging deploy failed as expected after %s: %v\n", formatElapsed(time.Since(deployStart)), err)

	err = assertGuardWindowRan()
	if err != nil {
		return err
	}

	err = assertGuardWindowBound(failTime, watcher)
	if err != nil {
		return err
	}

	err = waitForSSH(nixosISOPort, res.keyPath)
	if err != nil {
		return errors.Wrap(err, "machine unreachable after the hang revert")
	}

	closureAfter, genAfter, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	fmt.Printf("  after: closure=%s generation=%d\n", closureAfter, genAfter)

	return assertGuardRevertRestored("guard hang deadline", closureBefore, genBefore, closureAfter, genAfter)
}

// runDeployGuardSshdKill deploys a configuration whose activation stops sshd:
// panix loses the transport mid-window while the detached guardian survives
// (KillUserProcesses=no, spec 10.2), self-reverts at the deadline, and the
// revert restarts sshd. Panix must reconnect, resolve the outcome from the
// guard log (spec 9.2), and report the deploy as failed.
func runDeployGuardSshdKill(configPath string, res *testResources) error {
	printPhasef("Phase: Deploy NixOS with sshd killed mid-window")

	closureBefore, genBefore, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	fmt.Printf("  before: closure=%s generation=%d\n", closureBefore, genBefore)

	err = runPanixDeployWithArgs(configPath,
		[]string{"--tags", "test-vm-sshd-kill"},
		"PANIX_TEST_MODE=deploy",
		"PANIX_TEST_SCOPE="+string(testScopeFlag),
		"PANIX_KEXEC_PATH="+res.kexecInstallerPath,
	)
	if err == nil {
		return errors.New("expected the sshd-kill guarded deploy to fail")
	}

	fmt.Printf("  sshd-kill deploy failed as expected: %v\n", err)

	err = assertGuardWindowRan()
	if err != nil {
		return err
	}

	// The revert re-runs the previous generation's activation, which starts
	// sshd again; waitForSSH bounds the heal at two minutes.
	err = waitForSSH(nixosISOPort, res.keyPath)
	if err != nil {
		return errors.Wrap(err, "machine did not become reachable again after the sshd-kill revert")
	}

	closureAfter, genAfter, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	fmt.Printf("  after: closure=%s generation=%d\n", closureAfter, genAfter)

	return assertGuardRevertRestored("guard sshd-kill", closureBefore, genBefore, closureAfter, genAfter)
}

// runDeployGuardGcFail deploys a configuration whose activation runs
// nix-collect-garbage -d and then fails: the slot's gc-root must keep the new
// closure alive while the old one is the current generation, so the revert
// still finds both closures intact (spec 4.3).
func runDeployGuardGcFail(configPath string, res *testResources) error {
	printPhasef("Phase: Deploy NixOS with GC during the guard window")

	closureBefore, genBefore, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	fmt.Printf("  before: closure=%s generation=%d\n", closureBefore, genBefore)

	err = runPanixDeployWithArgs(configPath,
		[]string{"--tags", "test-vm-gc-fail"},
		"PANIX_TEST_MODE=deploy",
		"PANIX_TEST_SCOPE="+string(testScopeFlag),
		"PANIX_KEXEC_PATH="+res.kexecInstallerPath,
	)
	if err == nil {
		return errors.New("expected the gc-failing guarded deploy to fail")
	}

	fmt.Printf("  gc-fail deploy failed as expected: %v\n", err)

	err = assertGuardWindowRan()
	if err != nil {
		return err
	}

	closureAfter, genAfter, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	fmt.Printf("  after: closure=%s generation=%d\n", closureAfter, genAfter)

	return assertGuardRevertRestored("guard gc-fail", closureBefore, genBefore, closureAfter, genAfter)
}

// runDeployGuardConcurrent runs two guarded deploys of the hang fixture
// against the same machine concurrently: the deployer-side lock (spec 4.1
// step 3) must fail one deploy fast with the live-transaction indicator while
// the other proceeds. The hang fixture keeps the winner's lock window wide
// (activation deadline), which makes the overlap deterministic.
func runDeployGuardConcurrent(configPath string, res *testResources) error {
	printPhasef("Phase: Deploy NixOS concurrently to one machine (guard lock)")

	stopSequentialMgr()

	results := runConcurrentGuardDeploys(configPath, res)

	var locked, proceeded *concurrentGuardResult

	for i := range results {
		result := &results[i]

		if result.hasLockMessage() {
			if locked != nil {
				return errors.New("both concurrent deploys hit the slot lock")
			}

			locked = result

			continue
		}

		if proceeded != nil {
			return errors.New("both concurrent deploys proceeded past the slot lock")
		}

		proceeded = result
	}

	if locked == nil || proceeded == nil {
		return errors.New("expected exactly one concurrent deploy to fail fast on the slot lock")
	}

	if locked.err == nil {
		return errors.Errorf("the lock-losing deploy (%s) must fail fast", locked.mode)
	}

	fmt.Printf("  %s failed fast on the slot lock: %v\n", locked.mode, locked.err)

	// The winner proceeds into the guarded window and fails at the hang
	// fixture's deadline (that leg's own contract); its output must not carry
	// the lock indicator.
	if proceeded.err == nil {
		return errors.Errorf("the hanging deploy (%s) was expected to fail at the activation deadline", proceeded.mode)
	}

	fmt.Printf("  %s proceeded past the lock and failed at the deadline: %v\n", proceeded.mode, proceeded.err)

	return nil
}

// concurrentGuardResult is one concurrent deploy's outcome.
type concurrentGuardResult struct {
	mode   string
	err    error
	output string
}

// hasLockMessage checks the captured process output and, as a fallback, the
// process's own log file: the headless console writer may summarize step
// failures, while the file log records every command event with its output.
func (r concurrentGuardResult) hasLockMessage() bool {
	if containsGuardLockMessage(r.output) {
		return true
	}

	logPath, err := newestPanixLog(r.mode)
	if err != nil {
		return false
	}

	content, readErr := os.ReadFile(logPath) //nolint:gosec // repo-local test log
	if readErr != nil {
		return false
	}

	return containsGuardLockMessage(string(content))
}

// runConcurrentGuardDeploys starts two guarded deploys of the hang fixture at
// the same time and waits for both. Each process gets its own output buffer
// (no shared terminal TUI) and its own log file via its test mode.
func runConcurrentGuardDeploys(configPath string, res *testResources) []concurrentGuardResult {
	modes := []string{"deploy-conc-a", "deploy-conc-b"}

	results := make([]concurrentGuardResult, len(modes))

	var wg sync.WaitGroup

	for index, mode := range modes {
		wg.Add(1)

		go func(index int, testMode string) {
			defer wg.Done()

			var output bytes.Buffer

			cmd := panixCommandSpec(panixDeploySubcommand, configPath,
				[]string{"--tags", "test-vm-hang"},
				[]string{
					"PANIX_TEST_MODE=" + testMode,
					"PANIX_TEST_SCOPE=" + string(testScopeFlag),
					"PANIX_KEXEC_PATH=" + res.kexecInstallerPath,
				},
				&output,
			)

			results[index] = concurrentGuardResult{mode: testMode, err: cmd.Run(), output: output.String()}
		}(index, mode)
	}

	wg.Wait()

	return results
}

// containsGuardLockMessage reports whether a deploy's output carries the
// fail-fast lock contract (spec 4.1 step 3).
func containsGuardLockMessage(output string) bool {
	for _, message := range guardLockMessages {
		if strings.Contains(output, message) {
			return true
		}
	}

	return false
}

// assertGuardWindowRan proves the deploy reached the guard window and failed
// through the guard's outcome path, not at an earlier phase: the log must
// carry the guarded activation step and the revert outcome (spec 9.1). Without
// this check a deploy failing before pre-start would satisfy the "expected to
// fail" assertions vacuously.
func assertGuardWindowRan() error {
	logPath, err := newestPanixLog("deploy")
	if err != nil {
		return err
	}

	content, err := os.ReadFile(logPath) //nolint:gosec // repo-local test log
	if err != nil {
		return errors.Wrapf(err, "read panix deploy log %s", logPath)
	}

	deployLog := string(content)

	for _, evidence := range []struct{ needle, why string }{
		{"guarded activation", "the guardian never spawned: the deploy failed before the guard window"},
		{"activation failed", "the deploy did not report a guard-resolved outcome"},
	} {
		if !strings.Contains(deployLog, evidence.needle) {
			return errors.Errorf(
				"panix deploy log %s is missing %q: %s",
				logPath, evidence.needle, evidence.why,
			)
		}
	}

	return nil
}

// assertGuardErrorText proves the deployer surfaces the original activation
// error, not just the revert: the failing deploy's log must contain the
// fixture's error text (spec 9.1: every failure report carries the original
// error excerpt and the guard log path). Under the v2 record protocol the text
// travels inside the terminal record's err field as a JSON string (the fixture
// text carries no escapable characters, so it appears verbatim in the record
// line) and is streamed raw by the failing child before that.
func assertGuardErrorText() error {
	logPath, err := newestPanixLog("deploy")
	if err != nil {
		return err
	}

	content, err := os.ReadFile(logPath) //nolint:gosec // repo-local test log
	if err != nil {
		return errors.Wrapf(err, "read panix deploy log %s", logPath)
	}

	if !strings.Contains(string(content), guardFailingMarker) {
		return errors.Errorf(
			"panix deploy log %s does not contain the injected error text %q: "+
				"panix must report the original activation error, not just the revert",
			logPath, guardFailingMarker,
		)
	}

	return nil
}

// newestPanixLog returns the newest log file panix wrote for a test mode.
// Each run appends an epoch timestamp before .log (internal/logger), and the
// harness wipes the log dir at startup, so the newest match is the last
// panix invocation for that mode.
func newestPanixLog(mode string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(logDirPath, "panix-"+mode+".*.log"))
	if err != nil {
		return "", errors.Wrapf(err, "glob panix %s logs", mode)
	}

	if len(matches) == 0 {
		return "", errors.Errorf("no panix %s log found in %s", mode, logDirPath)
	}

	sort.Strings(matches)

	return matches[len(matches)-1], nil
}

// captureGuardBaseline reads the system profile state the guard legs assert
// against: the current closure and generation on the shared VM.
func captureGuardBaseline(keyPath string) (string, uint, error) {
	closure, err := readSystemProfileClosure(keyPath)
	if err != nil {
		return "", 0, errors.Wrap(err, "read system closure")
	}

	generation, err := readSystemProfileGeneration(keyPath)
	if err != nil {
		return "", 0, errors.Wrap(err, "read system generation")
	}

	return closure, generation, nil
}

// assertGuardRevertRestored asserts the profile-last invariant every failing
// guarded deploy must leave behind: the same closure (the revert restored it)
// and an unchanged generation list (the failed generation is not kept, spec
// 4.2 revert).
func assertGuardRevertRestored(leg string, closureBefore string, genBefore uint, closureAfter string, genAfter uint) error {
	if closureAfter != closureBefore {
		return errors.Errorf("%s: expected the revert to restore closure %s, got %s", leg, closureBefore, closureAfter)
	}

	if genAfter != genBefore {
		return errors.Errorf("%s: expected the generation list to stay unchanged (profile-last), before=%d after=%d", leg, genBefore, genAfter)
	}

	return nil
}

// assertGuardWindowBound fails the leg when the deploy needed longer than 2x
// activation_timeout (plus slack for the kill, the revert and the reporting)
// from the guard window's start: the deadline, not panix, must bound a hung
// activation.
func assertGuardWindowBound(failTime time.Time, watcher *guardWindowWatcher) error {
	result := watcher.result()
	if result.watchErr != nil {
		return result.watchErr
	}

	window := failTime.Sub(result.started)

	bound := 2*guardActivationTimeout + guardWindowSlack
	if window > bound {
		return errors.Errorf("the guard needed %s from the window start to fail the deploy, bound %s", formatElapsed(window), formatElapsed(bound))
	}

	fmt.Printf("  guard window bound ok: %s (bound %s)\n", formatElapsed(window), formatElapsed(bound))

	return nil
}

// guardWindowResult is the watcher's outcome: the detected window start or
// the reason it could not be detected.
type guardWindowResult struct {
	started  time.Time
	watchErr error
}

// guardWindowWatcher detects the moment the guard window opens on the shared
// VM: the pre-start sweep truncates the slot log (or creates it on a fresh
// slot), which changes the log's mtime. Comparing against the mtime captured
// before the deploy keeps the detection free of host/VM clock skew.
type guardWindowWatcher struct {
	results chan guardWindowResult
	done    chan struct{}
	once    sync.Once
}

func newGuardWindowWatcher(keyPath string) *guardWindowWatcher {
	watcher := &guardWindowWatcher{
		results: make(chan guardWindowResult, 1),
		done:    make(chan struct{}),
	}

	previous, err := guardSlotLogMtime(keyPath)
	if err != nil {
		watcher.results <- guardWindowResult{watchErr: errors.Wrap(err, "read the guard slot log mtime before the deploy")}

		return watcher
	}

	go func() {
		ticker := time.NewTicker(guardWindowPollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-watcher.done:
				watcher.results <- guardWindowResult{watchErr: errors.New("guard window start not detected before the deploy finished")}

				return
			case <-ticker.C:
				current, statErr := guardSlotLogMtime(keyPath)
				if statErr != nil {
					continue // the transport may legitimately be down mid-window
				}

				if current != previous {
					watcher.results <- guardWindowResult{started: time.Now()}

					return
				}
			}
		}
	}()

	return watcher
}

// result stops the watcher and returns the detection outcome.
func (w *guardWindowWatcher) result() guardWindowResult {
	w.once.Do(func() { close(w.done) })

	return <-w.results
}

// guardSlotLogMtime returns the target-side guard slot log's mtime epoch as a
// string, or "none" while no slot exists. The string form keeps the change
// detection free of clock assumptions; multiple slots (impossible on this VM)
// collapse to the first line.
func guardSlotLogMtime(keyPath string) (string, error) {
	output, err := sshRun(nixosISOPort, keyPath, "stat -c '%Y' "+guardSlotLogGlob+" 2>/dev/null || echo none")
	if err != nil {
		return "", errors.Wrap(err, "stat guard slot log")
	}

	return strings.TrimSpace(strings.SplitN(strings.TrimSpace(output), "\n", splitParts)[0]), nil
}
