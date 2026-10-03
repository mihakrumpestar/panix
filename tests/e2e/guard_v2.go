package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mihakrumpestar/panix/internal/guard"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

// Stage 3 Activation Guard e2e legs (docs/design/activation-guard.md section
// 13 additions). The seven v1.3 legs in guard.go stay untouched; these legs
// run after them on the same VM (nixos-iso-vm), the same system-tier guard
// slot, strictly sequentially: every leg leaves the slot terminal (or, for the
// committed legs, committed and swept by the next deploy's pre-start).
//
// Spec-13 leg mapping and groupings (grouping keeps wall time down where a
// separate leg would repeat the same setup without a new observation):
//
//	leg  1 (link-down mid-window)          -> runDeployGuardLinkDown
//	leg  2 (guardian death, relay alive)   -> runDeployGuardianDeath
//	leg  3 (graceful cancel)               -> runDeployGuardCtl (grouped with 14)
//	leg  4 (torn JSONL tail)               -> runDeployGuardTornTail
//	leg  5+12 (legacy slot upgrade, GC
//	          root and binary cleanup)     -> runDeployGuardLegacySweep
//	leg  6 (stdin forwarding through su)   -> runGuardUserTierLeg
//	leg  7 (relay EOF with terminal
//	        record, false-revert guard)     -> runDeployGuardCommittedOutcome
//	leg  8 (guardian startup panic)        -> runDeployGuardianDeath (documented)
//	leg  9 (pre-readiness command frame)   -> runDeployGuardCommittedOutcome (grouped)
//	leg 10 (no echo self-ack)              -> runDeployGuardCommittedOutcome (grouped)
//	leg 11 (slow-reader backpressure)      -> runDeployGuardFlood (grouped with 16)
//	leg 13 (relay spawn failure)           -> runDeployGuardFailedPrecondition (documented)
//	leg 14 (ack-timeout exit 7)            -> runDeployGuardCtl (grouped with 3)
//	leg 15 (boot-mode guarded leg)         -> runDeployGuardBoot
//	leg 16 (LINK_DOWN/LINK_DEGRADED)       -> runDeployGuardFlood (LINK_DOWN half in runDeployGuardLinkDown)
//
// Documented deviations (why a leg is not expressible exactly as specified):
//
//   - Leg 3's cause=cancel assertion: panix's give-up path
//     (cancellationOutcome) does not send a ctl revert-request, and the ctl
//     verb has no --cause flag, so no revert record can carry a cancel cause.
//     The leg covers the expressible guardian-side angle instead: a
//     fresh-connection ctl revert-request mid-window is acked (REVERT_START)
//     and reverts the transaction.
//
//   - Leg 8's pre-wiring guardian death: the transfer -> spawn gap is a few
//     exec round-trips, so a harness kill cannot land before the guardian's
//     first record deterministically. The guardian-death leg covers the same
//     classification contract (EOF with a non-terminal tail -> exit 6 -> panix
//     inline converge) from the earliest deterministic injection point.
//
//   - Leg 13's true spawn failure: spawnDetached fails only on fork/exec
//     errors inside the same pre-start exec chain, which the harness cannot
//     reach deterministically. The leg injects the nearest deterministic
//     zero-effects precondition failure instead: a synthetic non-terminal
//     transaction whose TXN revert list cannot run fails the pre-start sweep,
//     which writes FAILED_PRECONDITION and releases the lock. It also proves
//     the sweep converges from the TXN embedding (spec 7, stage 2).
//
//   - Leg 9's frame-side ordering: command frames are not logged, so the
//     record stream can only prove the ack side: the rid-bearing ack must
//     appear strictly after the ACTIVATED record (the readiness gate panix
//     waits for before writing its first frame).

const (
	// Slot layout (spec 10.1): system-tier v2 slots under /run/panix-guard/v2,
	// user-tier slots under the owner's state home. The legacy (pre-v2) slot
	// of a v2 slot lives one level up under the same name.
	guardV2SlotGlob   = "/run/panix-guard/v2/*/"
	guardUserSlotGlob = "/home/guarduser/.local/state/panix-guard/v2/*/"

	guardSlotBinaryName = "panix-guard"
	guardRecordPrefix   = "@PG2 "

	// Fixture markers asserted on the targets.
	guardCommitMarkerPath    = "/etc/panix-guard-commit-marker"
	guardCommitMarkerContent = "panix-e2e-commit-ok"
	guardUserMarkerCommand   = "su -l guarduser -c 'cat ~/.panix-guard-user-marker'"
	guardUserMarkerContent   = "panix-e2e-user-tier-stdin-ok"

	// Synthetic log content injected by the harness between deploys (the log
	// is only flock-guarded against cooperating writers, so root shell writes
	// land between transactions).
	guardLegacyOrphanKey   = "panix-e2e-legacy-orphan"
	guardLegacyNoiseLine   = "@PG1 panix-e2e legacy noise line"
	guardSynthTxnKey       = "panix-e2e-synth-txn"
	guardSynthRevertStep   = "/run/panix-e2e-nonexistent-revert-step"
	guardSynthNewClosure   = "/nix/store/00000000000000000000000000000000-panix-e2e-synth-new"
	guardLegacyFakeClosure = "/nix/store/00000000000000000000000000000000-panix-e2e-legacy-target"

	// The ctl ack-timeout probe uses a --wait shorter than any ack latency
	// (the guardian is mid-activation, so no ack can arrive): exit 7 with the
	// "may still take effect" note is the distinct degraded outcome (spec 8,
	// 9.5 ctl block).
	guardCtlWaitShort = 2 * time.Second
	guardCtlWaitLong  = 10 * time.Second

	// guardTerminalWait bounds the post-injection waits that involve the
	// guardian's own deadline: 2x activation timeout plus the window slack,
	// mirroring assertGuardWindowBound's bound in guard.go.
	guardTerminalWait = 2*guardActivationTimeout + guardWindowSlack

	// guardWindowOpenWait bounds the wait for the window to open (the HELLO
	// record) before a mid-window injection: the deploy's pre-window phases
	// (inspect, build no-op, transfer, probes) precede it.
	guardWindowOpenWait = 2 * time.Minute

	// guardStallWindow bounds the flood leg's relay-stall window (safety
	// pin b): the consumer must stay SIGSTOPped far inside the machine's
	// 30s activation_timeout. ~5s fills the 64KB fd4 pipe (~32 padded 2KB
	// frames) and then the 256-frame drop-oldest ring deterministically.
	guardStallWindow = 5 * time.Second

	// The flood leg's deferred SIGCONT retry cadence (safety pin a): a
	// leaked STOPped relay wedges every subsequent leg on the VM, so the
	// resume retries through transient SSH hiccups before giving up loudly.
	guardResumeAttempts      = 3
	guardResumeRetryInterval = time.Second

	// guardInlineConvergeEvidence is the panix-log marker of the inline
	// post-mortem path (spec 9.4): its presence distinguishes a converged
	// outcome from a relay-mapped one.
	guardInlineConvergeEvidence = "guard post-mortem convergence"

	// guardFailedPreconditionReport is the deployer-visible wording the
	// phase error must carry for the guardian's failed-precondition exit
	// (reportOutcome's exit-4 mapping in phaseops/guard/deploy.go, spec 9.1):
	// leg 13 asserts the outcome is classified, not an unknown exit.
	guardFailedPreconditionReport = "precondition check failed, zero effects"
)

// guardRecord mirrors the @PG2 envelope (internal/guard records.go) for the
// harness-side log parsing; rid rides beside the envelope exactly like the
// deployer's rid probe.
type guardRecord struct {
	V    int    `json:"v"`
	K    string `json:"k"`
	W    string `json:"w"`
	Seq  uint64 `json:"seq"`
	TS   int64  `json:"ts"`
	Ev   string `json:"ev"`
	St   string `json:"st"`
	PID  int    `json:"pid"`
	CPID int    `json:"cpid"`
	Old  string `json:"old"`
	New  string `json:"new"`
	Gen  int64  `json:"gen"`
	Mode string `json:"mode"`
	Tier string `json:"tier"`
	RC   int    `json:"rc"`
	Rs   string `json:"rs"`
	Err  string `json:"err"`
	Rid  int64  `json:"rid"`
	AA   string `json:"aa"`
	CA   string `json:"ca"`
	RA   string `json:"ra"`
	IV   string `json:"iv"`
}

// guardInspectVerdict is the subset of panix-guard inspect's JSON verdict the
// legs assert on (spec 9.5).
type guardInspectVerdict struct {
	Status   string `json:"status"`
	Terminal bool   `json:"terminal"`
	Lock     bool   `json:"lock"`
}

// parseGuardRecords extracts every parsable @PG2 record line from a log
// excerpt, tolerating narrative noise and torn lines exactly like the
// deployer's scanners.
func parseGuardRecords(content string) []guardRecord {
	var out []guardRecord

	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSuffix(line, "\r")
		if !strings.HasPrefix(trimmed, guardRecordPrefix) {
			continue
		}

		var r guardRecord

		if json.Unmarshal([]byte(trimmed[len(guardRecordPrefix):]), &r) != nil {
			continue
		}

		out = append(out, r)
	}

	return out
}

// guardFindRecord returns the first record with the given event.
func guardFindRecord(records []guardRecord, ev string) (guardRecord, bool) {
	for _, r := range records {
		if r.Ev == ev {
			return r, true
		}
	}

	return guardRecord{}, false
}

// guardCountRecords counts the records with the given event.
func guardCountRecords(records []guardRecord, ev string) int {
	count := 0

	for _, r := range records {
		if r.Ev == ev {
			count++
		}
	}

	return count
}

// guardShellQuote single-quotes a string for the target shell; the fragments
// the legs pass never contain single quotes.
func guardShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// guardRecordLines renders synthetic record payloads as complete @PG2 lines.
func guardRecordLines(payloads []string) string {
	lines := make([]string, len(payloads))
	for i, payload := range payloads {
		lines[i] = guardRecordPrefix + payload
	}

	return strings.Join(lines, "\n")
}

// guardResolveSlotDir resolves the single slot directory behind a slot glob.
func guardResolveSlotDir(keyPath, glob string) (string, error) {
	output, err := sshRun(nixosISOPort, keyPath, "ls -d "+glob+" 2>/dev/null | head -n 1")
	if err != nil {
		return "", errors.Wrapf(err, "resolve guard slot behind %s", glob)
	}

	slotDir := strings.TrimSpace(output)
	if slotDir == "" {
		return "", errors.Errorf("no guard slot found behind %s", glob)
	}

	return strings.TrimSuffix(slotDir, "/"), nil
}

// guardFetchRecords reads a slot log and parses its records; the raw content
// comes back for noise-level assertions.
func guardFetchRecords(keyPath, logPath string) ([]guardRecord, string, error) {
	output, err := sshRun(nixosISOPort, keyPath, "cat "+guardShellQuote(logPath))
	if err != nil {
		return nil, "", errors.Wrapf(err, "read guard slot log %s", logPath)
	}

	return parseGuardRecords(output), output, nil
}

// guardLogBaseline is the slot log's staleness baseline, captured before a
// leg's deploy starts: the checksum of the log's first 256 bytes. The log
// persists across transactions and the pre-start sweep truncates it to zero
// in place (cmd/panix-guard TruncateInPlace), so the new transaction's
// records start at byte offset 0 and a byte-size baseline cannot separate old
// from new content. The head can: its first record embeds the per-deploy key
// (unix nanos plus random bytes, spec 7), so the head differs across
// transactions by construction.
type guardLogBaseline struct {
	headMD5 string
}

// guardLogHeadMD5 renders the target-side snippet hashing the log's first 256
// bytes (the empty-input hash while the log is absent or empty).
func guardLogHeadMD5(logPath string) string {
	return "$(head -c 256 " + guardShellQuote(logPath) + " 2>/dev/null | md5sum | cut -d' ' -f1)"
}

// captureGuardLogBaseline reads the baseline head checksum over raw SSH.
func captureGuardLogBaseline(keyPath, logPath string) (guardLogBaseline, error) {
	output, err := sshRun(nixosISOPort, keyPath, "echo "+guardLogHeadMD5(logPath))
	if err != nil {
		return guardLogBaseline{}, errors.Wrapf(err, "read the guard slot log head %s", logPath)
	}

	return guardLogBaseline{headMD5: strings.TrimSpace(output)}, nil
}

// guardWaitForRecordFragment polls the slot log until it carries the fragment
// in the new transaction's era: the log's head must differ from the pre-deploy
// baseline first, because the persistent log still carries the previous leg's
// records until the pre-start truncation lands (the link-down leg's diagnosed
// race: its HELLO wait matched the previous leg's record on the first poll and
// killed the relay before any guardian spawned). Before the reset the wait
// greps nothing at all; after it the whole log is new-transaction content, so
// the fragment match is race-free without clock or timing assumptions. The
// mtime-baseline guardWindowWatcher solves the same problem at mtime
// granularity.
func guardWaitForRecordFragment(keyPath, logPath, fragment string, baseline guardLogBaseline, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		output, err := sshRun(nixosISOPort, keyPath,
			"h="+guardLogHeadMD5(logPath)+"; "+
				"[ \"$h\" != "+guardShellQuote(baseline.headMD5)+" ] && { "+
				"grep -Fq -- "+guardShellQuote(fragment)+" "+guardShellQuote(logPath)+" 2>/dev/null && echo found; }; true")
		if err == nil && strings.Contains(output, "found") {
			return nil
		}

		time.Sleep(guardWindowPollInterval)
	}

	return errors.Errorf("the guard slot log %s never carried %q after its pre-start reset within %s", logPath, fragment, timeout)
}

// guardHeredocWrite writes content to a target path through a quoted heredoc,
// so record JSON survives the shell untouched.
func guardHeredocWrite(keyPath, path, content string) error {
	command := "cat > " + guardShellQuote(path) + " <<'PANIXE2EOF'\n" + content + "\nPANIXE2EOF"

	_, err := sshRun(nixosISOPort, keyPath, command)
	if err != nil {
		return errors.Wrapf(err, "write %s on the target", path)
	}

	return nil
}

// guardRunInspect runs the slot binary's inspect verb over raw SSH and parses
// the one-JSON verdict (spec 9.5).
func guardRunInspect(keyPath, slotDir string) (guardInspectVerdict, error) {
	var verdict guardInspectVerdict

	output, err := sshRun(nixosISOPort, keyPath,
		guardShellQuote(filepath.Join(slotDir, guardSlotBinaryName))+" inspect --dir "+guardShellQuote(slotDir))
	if err != nil {
		return verdict, errors.Wrap(err, "guard inspect")
	}

	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &verdict); err != nil {
		return verdict, errors.Wrapf(err, "parse guard inspect verdict %q", output)
	}

	return verdict, nil
}

// guardDumpSlotLogTail prints the raw tail of a slot log over SSH: the
// parsed-record fetch (guardFetchRecords) discards narrative lines, and a
// failing step's argv plus errno live exactly there (fork/exec-class failures
// never start a child, so no step output streams). REVERT_FAILED-class and
// failed-convergence outcomes call this BEFORE their record assertions, so
// the step evidence reaches the console while the VM is still up.
func guardDumpSlotLogTail(keyPath, slotDir string) error {
	output, _, code, err := sshRunStatus(nixosISOPort, keyPath,
		"tail -c 65536 "+guardShellQuote(slotDir+"/log")+" 2>/dev/null || true")
	if err != nil {
		return errors.Wrapf(err, "fetch the guard slot log tail %s", slotDir)
	}

	if code != 0 {
		return errors.Errorf("the guard slot log tail fetch exited %d", code)
	}

	printPhasef("Guard slot log tail (" + slotDir + "):")

	for line := range strings.SplitSeq(output, "\n") {
		fmt.Printf("  %s\n", strings.TrimSuffix(line, "\r"))
	}

	return nil
}

// guardKillRelay kills the relay process (the pre-spawn panix-guard start
// parent) without touching the detached guardian: the guardian is marked by
// PANIX_GUARD_DETACHED in its environ, and shell/su wrappers are excluded by
// their argv shape.
func guardKillRelay(keyPath string) error {
	script := `
killed=0
for p in $(pgrep -f 'panix-guard start' 2>/dev/null); do
	cmd=$(tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null) || continue
	case "$cmd" in
		"su "*|"bash "*|"sh "*|"sudo "*) continue ;;
	esac
	if tr '\0' '\n' < "/proc/$p/environ" 2>/dev/null | grep -q '^PANIX_GUARD_DETACHED=1$'; then
		continue
	fi
	kill -9 "$p" 2>/dev/null && { echo "relay-killed-$p"; killed=1; }
done
[ "$killed" = 1 ] || echo relay-not-found
`

	output, err := sshRun(nixosISOPort, keyPath, script)
	if err != nil {
		return errors.Wrap(err, "kill the guard relay")
	}

	if strings.Contains(output, "relay-not-found") {
		return errors.New("no relay process found to kill (pgrep saw no panix-guard start parent)")
	}

	if !strings.Contains(output, "relay-killed-") {
		return errors.Errorf("the relay kill script reported no kill: %q", output)
	}

	return nil
}

// guardSignalRelay delivers one signal to every relay process (the pre-spawn
// panix-guard start parent), mirroring guardKillRelay's targeting exactly:
// the detached guardian carries 'panix-guard start' in its own argv (it is a
// self re-exec) and is excluded by its PANIX_GUARD_DETACHED environ marker,
// shell/su wrappers by their argv shape.
func guardSignalRelay(keyPath, sig string, required bool) error {
	script := `
signaled=0
for p in $(pgrep -f 'panix-guard start' 2>/dev/null); do
	cmd=$(tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null) || continue
	case "$cmd" in
		"su "*|"bash "*|"sh "*|"sudo "*) continue ;;
	esac
	if tr '\0' '\n' < "/proc/$p/environ" 2>/dev/null | grep -q '^PANIX_GUARD_DETACHED=1$'; then
		continue
	fi
	kill ` + sig + ` "$p" 2>/dev/null && { echo "relay-signaled-$p"; signaled=1; }
done
[ "$signaled" = 1 ] || echo relay-not-found
`

	output, err := sshRun(nixosISOPort, keyPath, script)
	if err != nil {
		return errors.Wrapf(err, "signal the guard relay (%s)", sig)
	}

	if strings.Contains(output, "relay-not-found") {
		if !required {
			// Harmless no-op: the relay may have died during a stopped
			// window or already finished the transaction; the leg fails
			// through its normal assertions in both cases.
			return nil
		}

		return errors.New("no relay process found to signal (pgrep saw no panix-guard start parent)")
	}

	if !strings.Contains(output, "relay-signaled-") {
		return errors.Errorf("the relay signal script reported no signal: %q", output)
	}

	return nil
}

// guardStallRelay SIGSTOPs the relay so its fd4 drain stalls: the
// deterministic consumer-side backpressure the flood leg injects
// (producer-side volume can never outrun the drain). A missing relay is an
// error: the leg depends on the stall landing.
func guardStallRelay(keyPath string) error {
	return guardSignalRelay(keyPath, "-STOP", true)
}

// guardResumeRelay SIGCONTs the relay after a stall. A missing relay is a
// no-op: the relay may have died during the stopped window, and a CONT on a
// running (already resumed) process is equally harmless.
func guardResumeRelay(keyPath string) error {
	return guardSignalRelay(keyPath, "-CONT", false)
}

// guardRunCtl delivers one ctl request over a fresh raw-SSH connection and
// returns its exit status (the per-verb ctl block, spec 9.5).
func guardRunCtl(keyPath, slotDir, key, command string, wait time.Duration) (stdout, stderr string, code int, err error) {
	ctlArgv := fmt.Sprintf("%s ctl --dir %s --key %s --wait %s %s",
		guardShellQuote(filepath.Join(slotDir, guardSlotBinaryName)),
		guardShellQuote(slotDir), guardShellQuote(key), wait, guardShellQuote(command))

	return sshRunStatus(nixosISOPort, keyPath, ctlArgv)
}

// sshRunStatus runs a command over raw SSH and reports its exit status; a
// delivered exit status is not a transport failure.
func sshRunStatus(port int, keyPath, command string) (string, string, int, error) {
	config, err := sshConfig(keyPath)
	if err != nil {
		return "", "", -1, err
	}

	config.Timeout = sshRunTimeout

	addr := fmt.Sprintf("127.0.0.1:%d", port)

	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", "", -1, errors.Wrapf(err, "SSH dial %s", addr)
	}

	defer closeWithoutErrCheck(conn)

	session, err := conn.NewSession()
	if err != nil {
		return "", "", -1, errors.Wrap(err, "SSH session")
	}

	defer closeWithoutErrCheck(session)

	var stdout, stderr strings.Builder

	session.Stdout = &stdout
	session.Stderr = &stderr

	runErr := session.Run(command)

	code := 0 // a clean Run return is exit status 0

	exitErr := &ssh.ExitError{}
	if errors.As(runErr, &exitErr) {
		code = exitErr.ExitStatus()
		runErr = nil
	}

	return stdout.String(), stderr.String(), code, runErr
}

// guardTornTail tears the log's last line mid-window (drops the trailing
// newline) and returns the newest record sequence before the tear. The
// vacuous-pass guard proves the tear landed: the log must no longer end with
// a newline.
func guardTornTail(keyPath, logPath string) (uint64, error) {
	records, _, err := guardFetchRecords(keyPath, logPath)
	if err != nil {
		return 0, err
	}

	var newest uint64

	for _, r := range records {
		if r.Seq > newest {
			newest = r.Seq
		}
	}

	if _, err := sshRun(nixosISOPort, keyPath, "truncate -s -1 "+guardShellQuote(logPath)); err != nil {
		return 0, errors.Wrap(err, "tear the slot log tail")
	}

	output, err := sshRun(nixosISOPort, keyPath, "tail -c 1 "+guardShellQuote(logPath)+" | wc -l")
	if err != nil {
		return 0, errors.Wrap(err, "verify the torn tail")
	}

	if strings.TrimSpace(output) != "0" {
		return 0, errors.Errorf("the tail tear did not land: the log still ends with a newline (wc -l=%q)", output)
	}

	return newest, nil
}

// guardAssertProfileClosure asserts the system profile resolves to want.
func guardAssertProfileClosure(keyPath, want string) error {
	got, err := readSystemProfileClosure(keyPath)
	if err != nil {
		return err
	}

	if got != want {
		return errors.Errorf("expected the system profile to resolve to %s, got %s", want, got)
	}

	return nil
}

// runGuardDeploy runs one guarded deploy with the leg's own test mode, so the
// leg's panix-log assertions read that leg's log file only.
func runGuardDeploy(configPath string, tags, testMode string, res *testResources) error {
	return runPanixDeployWithArgs(configPath,
		[]string{"--tags", tags},
		"PANIX_TEST_MODE="+testMode,
		"PANIX_TEST_SCOPE="+string(testScopeFlag),
		"PANIX_KEXEC_PATH="+res.kexecInstallerPath,
	)
}

// runDeployGuardLegacySweep covers spec-13 legs 5 and 12 as one setup (the
// injections share the same pre-deploy window): the pre-start sequence must
// remove a stale legacy (pre-v2) slot tree wholesale (log, gc-root symlink,
// persisted binary) and classify a v2 log whose tail mixes @PG1 noise with a
// foreign-key terminal record as terminal, truncating it so the fresh
// transaction starts at sequence 1.
func runDeployGuardLegacySweep(configPath string, res *testResources) error {
	printPhasef("Phase: Guard legacy-slot sweep (spec 13 legs 5+12)")

	slotDir, err := guardResolveSlotDir(res.keyPath, guardV2SlotGlob)
	if err != nil {
		return err
	}

	legacyDir := filepath.Dir(filepath.Dir(slotDir)) + "/" + filepath.Base(slotDir)
	v2LogPath := slotDir + "/log"

	now := time.Now().Unix()
	fakeTerminal := fmt.Sprintf(
		`{"v":1,"k":%q,"w":"g","seq":99,"ts":%d,"ev":"REVERTED","st":"reverted"}`,
		guardLegacyOrphanKey, now)

	setup := strings.Join([]string{
		"set -e",
		"rm -rf " + guardShellQuote(legacyDir),
		"mkdir -p " + guardShellQuote(legacyDir),
		"printf '%s\\n' " + guardShellQuote("@PG1 panix-e2e legacy orphan record") + " > " + guardShellQuote(legacyDir+"/log"),
		"ln -sfn " + guardShellQuote(guardLegacyFakeClosure) + " " + guardShellQuote(legacyDir+"/gc-root"),
		"printf '#!/bin/sh\\nexit 1\\n' > " + guardShellQuote(filepath.Join(legacyDir, guardSlotBinaryName)),
		"chmod +x " + guardShellQuote(filepath.Join(legacyDir, guardSlotBinaryName)),
		"printf '%s\\n' " + guardShellQuote(guardRecordPrefix+fakeTerminal) + " >> " + guardShellQuote(v2LogPath),
		"printf '%s\\n' " + guardShellQuote(guardLegacyNoiseLine) + " >> " + guardShellQuote(v2LogPath),
		"echo legacy-injected",
	}, "\n")

	output, err := sshRun(nixosISOPort, res.keyPath, setup)
	if err != nil || !strings.Contains(output, "legacy-injected") {
		return errors.Wrapf(err, "inject the legacy slot state (output %q)", output)
	}

	// Vacuous-pass guard: the legacy tree and the mixed tail must exist before
	// the deploy that has to clean them up.
	output, err = sshRun(nixosISOPort, res.keyPath,
		"test -d "+guardShellQuote(legacyDir)+" && grep -q '@PG1' "+guardShellQuote(v2LogPath)+" && echo prestate-ok || true")
	if err != nil || !strings.Contains(output, "prestate-ok") {
		return errors.Errorf("the legacy pre-state is missing before the sweep deploy (output %q)", output)
	}

	err = runGuardDeploy(configPath, "test-vm-retry", "deploy", res)
	if err != nil {
		return errors.Wrap(err, "the guarded deploy over the injected legacy state failed")
	}

	output, err = sshRun(nixosISOPort, res.keyPath,
		"test -e "+guardShellQuote(legacyDir)+" && echo legacy-present || echo legacy-gone")
	if err != nil {
		return errors.Wrap(err, "check the legacy slot after the sweep")
	}

	if !strings.Contains(output, "legacy-gone") {
		return errors.Errorf("the legacy slot tree %s survived the pre-start sweep (spec 10.1 coexistence)", legacyDir)
	}

	records, content, err := guardFetchRecords(res.keyPath, v2LogPath)
	if err != nil {
		return err
	}

	if strings.Contains(content, "@PG1 ") {
		return errors.New("the v2 slot log still carries @PG1 noise after the pre-start truncation")
	}

	if len(records) == 0 || records[0].Ev != "HELLO" || records[0].Seq != 1 {
		return errors.Errorf("the fresh transaction did not start at sequence 1 with HELLO (first records: %d)", len(records))
	}

	fmt.Printf("  legacy slot swept, v2 log restarted at sequence 1\n")

	return nil
}

// runDeployGuardFailedPrecondition covers spec-13 leg 13's observable
// contract (failed-precondition-shaped outcome, nonzero exit, lock released)
// through the deterministic injection documented in the file header: a
// synthetic non-terminal transaction whose TXN embedding carries a revert
// list that cannot run fails the pre-start sweep, which writes
// FAILED_PRECONDITION with zero effects and releases the lock. The deployer
// side must classify the outcome too: the phase error carries the
// failed-precondition wording, not an unknown exit. The sweep converging from
// the TXN embedding (spec 7) is asserted on the way.
func runDeployGuardFailedPrecondition(configPath string, res *testResources) error {
	printPhasef("Phase: Guard failed-precondition outcome (spec 13 leg 13, sweep variant)")

	slotDir, err := guardResolveSlotDir(res.keyPath, guardV2SlotGlob)
	if err != nil {
		return err
	}

	logPath := slotDir + "/log"

	closureBefore, genBefore, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	content := guardRecordLines([]string{
		fmt.Sprintf(`{"v":1,"k":%q,"w":"g","seq":1,"ts":%d,"ev":"HELLO","old":%q,"new":%q,"gen":%d,"mode":"switch","tier":"full","pf":"/nix/var/nix/profiles/system","dl":%d}`,
			guardSynthTxnKey, now, closureBefore, guardSynthNewClosure, genBefore, now+3600),
		fmt.Sprintf(`{"v":1,"k":%q,"w":"g","seq":2,"ts":%d,"ev":"TXN","aa":%q,"ca":%q,"ra":%q,"iv":%q}`,
			guardSynthTxnKey, now, `[["/bin/true"]]`, `[]`, fmt.Sprintf("[[%q]]", guardSynthRevertStep), closureBefore),
		fmt.Sprintf(`{"v":1,"k":%q,"w":"g","seq":3,"ts":%d,"ev":"STATE","st":"activating"}`,
			guardSynthTxnKey, now),
	})

	err = guardHeredocWrite(res.keyPath, logPath, content)
	if err != nil {
		return err
	}

	err = runGuardDeploy(configPath, "test-vm-retry", "deploy", res)
	if err == nil {
		return errors.New("the deploy over a non-convergable synthetic transaction was expected to fail")
	}

	fmt.Printf("  deploy failed as expected: %v\n", err)

	// The deployer-visible outcome must name the failed-precondition
	// classification (reportOutcome's exit-4 wording), not an unknown exit:
	// the guardian's errStartMutation failure exits the spec'd code and the
	// relay maps it through the outcome vocabulary. The wording lives in the
	// deploy log; the run error itself is only the process exit status.
	deployLogPath, pathErr := newestPanixLog("deploy")
	if pathErr != nil {
		return pathErr
	}

	deployLogBytes, readErr := os.ReadFile(deployLogPath) //nolint:gosec // repo-local test log
	if readErr != nil {
		return errors.Wrapf(readErr, "read panix deploy log %s", deployLogPath)
	}

	if !strings.Contains(string(deployLogBytes), guardFailedPreconditionReport) {
		return errors.New("the deploy log does not name the failed-precondition outcome")
	}

	// The raw slot-log tail first: the parsed records below cannot name the
	// failing step's argv and errno, the narrative lines can.
	if err := guardDumpSlotLogTail(res.keyPath, slotDir); err != nil {
		return err
	}

	records, _, err := guardFetchRecords(res.keyPath, logPath)
	if err != nil {
		return err
	}

	revertFailed, ok := guardFindRecord(records, "REVERT_FAILED")
	if !ok || revertFailed.W != "d" {
		return errors.Errorf("expected a REVERT_FAILED record from the post-mortem writer (w=d), got %+v", revertFailed)
	}

	precondition, ok := guardFindRecord(records, "FAILED_PRECONDITION")
	if !ok {
		return errors.New("the sweep failure did not write the FAILED_PRECONDITION record")
	}

	if !strings.Contains(precondition.Err, "pre-start sweep failed") {
		return errors.Errorf("the FAILED_PRECONDITION record does not name the sweep failure: %q", precondition.Err)
	}

	closureAfter, genAfter, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	err = assertGuardRevertRestored("failed precondition", closureBefore, genBefore, closureAfter, genAfter)
	if err != nil {
		return err
	}

	verdict, err := guardRunInspect(res.keyPath, slotDir)
	if err != nil {
		return err
	}

	if verdict.Lock {
		return errors.Errorf("the slot lock is still held after the failed precondition (verdict %+v)", verdict)
	}

	fmt.Printf("  FAILED_PRECONDITION recorded, zero effects, lock released (status %s)\n", verdict.Status)

	return nil
}

// runDeployGuardCommittedOutcome covers spec-13 legs 7, 9 and 10: the first
// e2e commit. The relay must map the post-commit event-pipe EOF to the
// terminal COMMITTED record (exit 0, no inline converge, no revert), the
// confirm must be consumed exactly once with panix's first rid after the
// readiness gate, and the committed outcome must survive on the machine.
func runDeployGuardCommittedOutcome(configPath string, res *testResources) error {
	printPhasef("Phase: Guard committed outcome (spec 13 legs 7+9+10)")

	err := runGuardDeploy(configPath, "test-vm-commit", "deploy-commit", res)
	if err != nil {
		return errors.Wrap(err, "the guarded commit deploy failed")
	}

	slotDir, err := guardResolveSlotDir(res.keyPath, guardV2SlotGlob)
	if err != nil {
		return err
	}

	records, _, err := guardFetchRecords(res.keyPath, slotDir+"/log")
	if err != nil {
		return err
	}

	txn, ok := guardFindRecord(records, "TXN")
	if !ok {
		return errors.New("the committed deploy's slot log carries no TXN record")
	}

	if !strings.Contains(txn.CA, "--set") {
		return errors.Errorf("the switch-mode commit list lost the profile set step: %q", txn.CA)
	}

	hello, ok := guardFindRecord(records, "HELLO")
	if !ok {
		return errors.New("the committed deploy's slot log carries no HELLO record")
	}

	// The guardian surfaces readiness as a STATE snapshot at the activated
	// status (there is no ACTIVATED event record); the confirm loop keys its
	// decision off the same shape.
	readySeq := uint64(0)

	for _, r := range records {
		if r.St == "activated" {
			readySeq = r.Seq

			break
		}
	}

	if readySeq == 0 {
		return errors.New("the committed deploy's slot log carries no activated STATE snapshot")
	}

	if _, ok := guardFindRecord(records, "COMMITTED"); !ok {
		return errors.New("the committed deploy's slot log carries no COMMITTED record")
	}

	consumed := guardCountRecords(records, "CONFIRM_CONSUMED")
	if consumed != 1 {
		return errors.Errorf("expected exactly one CONFIRM_CONSUMED, got %d (echo or double-ack)", consumed)
	}

	confirm, _ := guardFindRecord(records, "CONFIRM_CONSUMED")
	if confirm.Rid != 1 {
		return errors.Errorf("the confirm ack carries rid %d, want panix's first frame rid 1", confirm.Rid)
	}

	if guardCountRecords(records, "CONFIRM_IGNORED") != 0 {
		return errors.New("a confirm was ignored on the committed path (self-ack or echo loop)")
	}

	for _, r := range records {
		if r.Rid != 0 && r.Seq < readySeq {
			return errors.Errorf("rid-bearing record %s (seq %d) precedes the activated STATE (seq %d): a frame was written before readiness",
				r.Ev, r.Seq, readySeq)
		}
	}

	if err := guardAssertProfileClosure(res.keyPath, hello.New); err != nil {
		return err
	}

	marker, err := sshRun(nixosISOPort, res.keyPath, "cat "+guardShellQuote(guardCommitMarkerPath))
	if err != nil || strings.TrimSpace(marker) != guardCommitMarkerContent {
		return errors.Errorf("the committed closure's marker is not live (got %q, err %v)", marker, err)
	}

	logPath, err := newestPanixLog("deploy-commit")
	if err != nil {
		return err
	}

	content, err := os.ReadFile(logPath) //nolint:gosec // repo-local test log
	if err != nil {
		return errors.Wrapf(err, "read panix deploy-commit log %s", logPath)
	}

	if strings.Contains(string(content), guardInlineConvergeEvidence) {
		return errors.Errorf("panix ran the inline post-mortem on a committed deploy (%s): the relay must map the terminal record, not exit 6", logPath)
	}

	fmt.Printf("  committed: profile=%s marker ok, one rid-1 confirm after readiness, no inline converge\n", hello.New)

	return nil
}

// runDeployGuardLinkDown covers spec-13 leg 1: the relay dies mid-window
// WITHOUT sshd being touched (the sshd-kill leg's transport-death overlap is
// documented in guard.go), the guardian logs LINK_DOWN for the dead event
// pipe, keeps converging autonomously, and meets the revert deadline; the
// profile ends restored.
func runDeployGuardLinkDown(configPath string, res *testResources) error {
	printPhasef("Phase: Guard link-down (relay death, sshd untouched; spec 13 leg 1)")

	closureBefore, genBefore, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	slotDir, err := guardResolveSlotDir(res.keyPath, guardV2SlotGlob)
	if err != nil {
		return err
	}

	logPath := slotDir + "/log"

	// The fragment waits baseline on the pre-deploy log size (the committed
	// leg before this one leaves its own HELLO in the persistent log; a whole
	// log grep matches it on the first poll and kills the relay ~1s in,
	// before any guardian spawned).
	baseline, err := captureGuardLogBaseline(res.keyPath, logPath)
	if err != nil {
		return err
	}

	var deployErr error

	deployDone := make(chan struct{})

	go func() {
		defer close(deployDone)

		deployErr = runGuardDeploy(configPath, "test-vm-hang", "deploy-link-down", res)
	}()

	err = guardWaitForRecordFragment(res.keyPath, logPath, `"ev":"HELLO"`, baseline, guardWindowOpenWait)
	if err != nil {
		<-deployDone

		return err
	}

	err = guardKillRelay(res.keyPath)
	if err != nil {
		<-deployDone

		return err
	}

	fmt.Printf("  relay killed mid-window; the guardian continues degraded\n")

	<-deployDone

	if deployErr == nil {
		return errors.New("the deploy was expected to fail after the relay died")
	}

	fmt.Printf("  deploy failed as expected: %v\n", deployErr)

	err = guardWaitForRecordFragment(res.keyPath, logPath, `"ev":"LINK_DOWN"`, baseline, guardTerminalWait)
	if err != nil {
		return err
	}

	err = guardWaitForRecordFragment(res.keyPath, logPath, `"ev":"REVERTED"`, baseline, guardTerminalWait)
	if err != nil {
		return err
	}

	err = waitForSSH(nixosISOPort, res.keyPath)
	if err != nil {
		return errors.Wrap(err, "machine unreachable after the link-down revert")
	}

	closureAfter, genAfter, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	return assertGuardRevertRestored("guard link-down", closureBefore, genBefore, closureAfter, genAfter)
}

// runDeployGuardianDeath covers spec-13 leg 2 (and documents leg 8, see the
// file header): the fixture kills the detached guardian mid-activation while
// the relay stays alive. The relay must classify the event-pipe EOF as
// cancelled (exit 6), panix must converge inline from the transaction's own
// records (killing the orphaned activation child first when it is still
// alive; the orphan may also have died on its own SIGPIPE, which is equally
// correct), and the converged outcome must be the deploy's own.
func runDeployGuardianDeath(configPath string, res *testResources) error {
	printPhasef("Phase: Guard guardian death with relay alive (spec 13 legs 2+8)")

	closureBefore, genBefore, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	err = runGuardDeploy(configPath, "test-vm-guardian-kill", "deploy-guardian-death", res)
	if err == nil {
		return errors.New("the guardian-death deploy was expected to fail with the converged revert outcome")
	}

	fmt.Printf("  guardian-death deploy failed as expected: %v\n", err)

	slotDir, err := guardResolveSlotDir(res.keyPath, guardV2SlotGlob)
	if err != nil {
		return err
	}

	records, _, err := guardFetchRecords(res.keyPath, slotDir+"/log")
	if err != nil {
		return err
	}

	// ORPHAN_KILLED is timing-dependent, not mandatory: the orphaned child
	// inherits guardian-held pipes, so its first write after the guardian
	// dies raises SIGPIPE (default-fatal) and the orphan may already be gone
	// when the converge checks liveness. Both outcomes converge correctly:
	// a live orphan is killed first (freeing the STC lock), a dead one needs
	// no kill. The mandatory assertions are the terminal REVERTED from the
	// post-mortem writer and the absence of any surviving orphan process.
	if orphan, ok := guardFindRecord(records, "ORPHAN_KILLED"); ok {
		fmt.Printf("  orphan killed by the converge (cpid %d)\n", orphan.CPID)
	}

	reverted, ok := guardFindRecord(records, "REVERTED")
	if !ok {
		return errors.New("the inline converge did not write a terminal REVERTED record")
	}

	if reverted.W != "d" {
		return errors.Errorf("the terminal REVERTED record came from writer %q, want the post-mortem writer d", reverted.W)
	}

	// No orphan may survive the converge: a live orphan would hold the STC
	// global lock and wedge any later activation.
	childPID := 0

	for _, r := range records {
		if r.CPID > 0 {
			childPID = r.CPID
		}
	}

	if childPID > 0 {
		out, _, code, err := sshRunStatus(nixosISOPort, res.keyPath,
			fmt.Sprintf("kill -0 %d 2>/dev/null && echo alive || echo dead", childPID))
		if err != nil {
			return errors.Wrap(err, "orphan liveness probe failed")
		}

		if code == 0 && strings.TrimSpace(out) == "alive" {
			return errors.Errorf("the orphaned activation child (cpid %d) survived the converge", childPID)
		}
	}

	logPath, err := newestPanixLog("deploy-guardian-death")
	if err != nil {
		return err
	}

	content, err := os.ReadFile(logPath) //nolint:gosec // repo-local test log
	if err != nil {
		return errors.Wrapf(err, "read panix deploy-guardian-death log %s", logPath)
	}

	if !strings.Contains(string(content), guardInlineConvergeEvidence) {
		return errors.Errorf("panix's log %s does not show the inline post-mortem convergence path", logPath)
	}

	err = waitForSSH(nixosISOPort, res.keyPath)
	if err != nil {
		return errors.Wrap(err, "machine unreachable after the guardian-death converge")
	}

	closureAfter, genAfter, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	return assertGuardRevertRestored("guardian death", closureBefore, genBefore, closureAfter, genAfter)
}

// runDeployGuardCtl covers spec-13 legs 3 and 14 in one hang window: the
// degraded ctl path's ack timeout (a confirm with a --wait shorter than any
// ack latency exits 7 with the "may still take effect" note, and the queued
// request really is still pending - the later revert discards it with
// CONFIRM_IGNORED), and the graceful-cancel delivery (a fresh-connection
// revert-request is acked by REVERT_START and reverts the transaction). The
// cause=cancel half of leg 3 is documented as not expressible in the file
// header.
func runDeployGuardCtl(configPath string, res *testResources) error {
	printPhasef("Phase: Guard ctl paths (ack timeout + graceful cancel; spec 13 legs 3+14)")

	closureBefore, genBefore, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	slotDir, err := guardResolveSlotDir(res.keyPath, guardV2SlotGlob)
	if err != nil {
		return err
	}

	logPath := slotDir + "/log"

	// The fragment wait baselines on the pre-deploy log size: the guardian
	// death leg before this one leaves its own HELLO in the persistent log,
	// and a whole-log grep would hand this leg's ctl calls the previous
	// transaction's key.
	baseline, err := captureGuardLogBaseline(res.keyPath, logPath)
	if err != nil {
		return err
	}

	var deployErr error

	deployDone := make(chan struct{})

	go func() {
		defer close(deployDone)

		deployErr = runGuardDeploy(configPath, "test-vm-hang", "deploy-ctl", res)
	}()

	err = guardWaitForRecordFragment(res.keyPath, logPath, `"ev":"HELLO"`, baseline, guardWindowOpenWait)
	if err != nil {
		<-deployDone

		return err
	}

	records, _, err := guardFetchRecords(res.keyPath, logPath)
	if err != nil {
		<-deployDone

		return err
	}

	hello, ok := guardFindRecord(records, "HELLO")
	if !ok {
		<-deployDone

		return errors.New("the slot log carries no HELLO record for the ctl key")
	}

	// (a) Ack timeout (leg 14): the guardian is mid-activation, so the confirm
	// cannot be acked within the short wait; exit 7 with the note is the
	// distinct "delivered but may still take effect" outcome.
	_, stderr, code, err := guardRunCtl(res.keyPath, slotDir, hello.K, "confirm", guardCtlWaitShort)
	if err != nil {
		<-deployDone

		return errors.Wrap(err, "ctl confirm transport failure")
	}

	if code != guard.CtlExitAckTimedOut {
		<-deployDone

		return errors.Errorf("ctl confirm exited %d, want the ack-timeout code %d (stderr %q)", code, guard.CtlExitAckTimedOut, stderr)
	}

	if !strings.Contains(stderr, "may still take effect") {
		<-deployDone

		return errors.Errorf("the ctl ack timeout did not report the may-still-take-effect caveat: %q", stderr)
	}

	fmt.Printf("  ctl confirm ack timed out (exit %d) with the caveat note\n", code)

	// (b) Graceful cancel (leg 3): one fresh-connection revert-request,
	// delivered and acked while the transaction is live.
	_, stderr, code, err = guardRunCtl(res.keyPath, slotDir, hello.K, "revert-request", guardCtlWaitLong)
	if err != nil {
		<-deployDone

		return errors.Wrap(err, "ctl revert-request transport failure")
	}

	if code != guard.CtlExitConsumed {
		<-deployDone

		return errors.Errorf("ctl revert-request exited %d, want consumed %d (stderr %q)", code, guard.CtlExitConsumed, stderr)
	}

	fmt.Printf("  ctl revert-request consumed (exit %d)\n", code)

	<-deployDone

	if deployErr == nil {
		return errors.New("the deploy was expected to fail with the cancelled transaction's revert outcome")
	}

	fmt.Printf("  deploy failed as expected: %v\n", deployErr)

	records, _, err = guardFetchRecords(res.keyPath, logPath)
	if err != nil {
		return err
	}

	ignored, ok := guardFindRecord(records, "CONFIRM_IGNORED")
	if !ok || !strings.Contains(ignored.Rs, "superseded by revert") {
		return errors.Errorf("the queued confirm was not discarded on revert (CONFIRM_IGNORED record: %+v)", ignored)
	}

	if _, ok := guardFindRecord(records, "REVERT_START"); !ok {
		return errors.New("the graceful cancel left no REVERT_START record")
	}

	if _, ok := guardFindRecord(records, "REVERTED"); !ok {
		return errors.New("the graceful cancel left no terminal REVERTED record")
	}

	err = waitForSSH(nixosISOPort, res.keyPath)
	if err != nil {
		return errors.Wrap(err, "machine unreachable after the cancel revert")
	}

	closureAfter, genAfter, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	return assertGuardRevertRestored("guard ctl", closureBefore, genBefore, closureAfter, genAfter)
}

// runDeployGuardTornTail covers spec-13 leg 4: corrupting the log's last line
// mid-window must not derail the transaction - the torn trailing write is
// noise to every scanner, the first post-tear record glues into it and is
// lost with it, and the next heartbeat's record lands cleanly and carries the
// transaction to its deadline revert.
func runDeployGuardTornTail(configPath string, res *testResources) error {
	printPhasef("Phase: Guard torn JSONL tail (spec 13 leg 4)")

	closureBefore, genBefore, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	slotDir, err := guardResolveSlotDir(res.keyPath, guardV2SlotGlob)
	if err != nil {
		return err
	}

	logPath := slotDir + "/log"

	// The fragment waits baseline on the pre-deploy log size: the ctl leg
	// before this one leaves its own HELLO (and REVERTED) in the persistent
	// log, and a whole-log grep matches those instead of this transaction's.
	baseline, err := captureGuardLogBaseline(res.keyPath, logPath)
	if err != nil {
		return err
	}

	var deployErr error

	deployDone := make(chan struct{})

	go func() {
		defer close(deployDone)

		deployErr = runGuardDeploy(configPath, "test-vm-hang", "deploy-torn-tail", res)
	}()

	err = guardWaitForRecordFragment(res.keyPath, logPath, `"ev":"HELLO"`, baseline, guardWindowOpenWait)
	if err != nil {
		<-deployDone

		return err
	}

	// Let at least one heartbeat record land so the tear splits a real record.
	time.Sleep(2 * guardWindowPollInterval)

	tornSeq, err := guardTornTail(res.keyPath, logPath)
	if err != nil {
		<-deployDone

		return err
	}

	fmt.Printf("  tail torn after seq %d\n", tornSeq)

	<-deployDone

	if deployErr == nil {
		return errors.New("the hanging deploy was expected to fail at the deadline despite the torn tail")
	}

	fmt.Printf("  deploy failed as expected: %v\n", deployErr)

	err = guardWaitForRecordFragment(res.keyPath, logPath, `"ev":"REVERTED"`, baseline, guardTerminalWait)
	if err != nil {
		return err
	}

	records, _, err := guardFetchRecords(res.keyPath, logPath)
	if err != nil {
		return err
	}

	// The tear glues exactly one post-tear WRITE into the torn line (a record
	// emit or a narrative line - only emits consume sequence numbers), so the
	// first surviving record is seq tornSeq+1 or tornSeq+2. Either proves the
	// writer recovered past the glued line.
	recovered := false

	for _, r := range records {
		if r.Seq > tornSeq {
			recovered = true
		}
	}

	if !recovered {
		return errors.Errorf("no record beyond seq %d after the tear: the writer did not recover past the glued line", tornSeq)
	}

	err = waitForSSH(nixosISOPort, res.keyPath)
	if err != nil {
		return errors.Wrap(err, "machine unreachable after the torn-tail revert")
	}

	closureAfter, genAfter, err := captureGuardBaseline(res.keyPath)
	if err != nil {
		return err
	}

	return assertGuardRevertRestored("guard torn tail", closureBefore, genBefore, closureAfter, genAfter)
}

// runDeployGuardBoot covers spec-13 leg 15 with both boot-mode halves in one
// leg: the commit (the pre-start boot set must be visible as the TXN's empty
// commit list, and the profile must resolve to the TXN's new closure after
// the commit skips the redundant set) and the revert (the activation child
// fails immediately because the fixture's activation_path does not exist in
// the closure; boot mode's effective gate is auto, so the revert comes from
// the activation outcome, not a health check) whose profile-first restore
// must put the profile back on the TXN's old closure. captureOld runs BEFORE
// the boot set inside start, so old is the pre-deploy closure by construction.
func runDeployGuardBoot(configPath string, res *testResources) error {
	printPhasef("Phase: Guard boot mode (commit + revert; spec 13 leg 15)")

	// (a) Boot-mode commit.
	err := runGuardDeploy(configPath, "test-vm-boot", "deploy-boot", res)
	if err != nil {
		return errors.Wrap(err, "the boot-mode guarded deploy failed")
	}

	slotDir, err := guardResolveSlotDir(res.keyPath, guardV2SlotGlob)
	if err != nil {
		return err
	}

	records, _, err := guardFetchRecords(res.keyPath, slotDir+"/log")
	if err != nil {
		return err
	}

	txn, ok := guardFindRecord(records, "TXN")
	if !ok {
		return errors.New("the boot-mode deploy's slot log carries no TXN record")
	}

	if txn.CA != "[]" {
		return errors.Errorf("the boot-mode commit list is not empty (the profile set belongs to pre-start): %q", txn.CA)
	}

	if !strings.Contains(txn.RA, "--set") {
		return errors.Errorf("the boot-mode revert list lost the profile-first restore: %q", txn.RA)
	}

	hello, ok := guardFindRecord(records, "HELLO")
	if !ok {
		return errors.New("the boot-mode deploy's slot log carries no HELLO record")
	}

	if _, ok := guardFindRecord(records, "COMMITTED"); !ok {
		return errors.New("the boot-mode deploy did not commit")
	}

	if err := guardAssertProfileClosure(res.keyPath, hello.New); err != nil {
		return errors.Wrap(err, "boot-mode commit: the profile must resolve to the HELLO new closure")
	}

	committedClosure := hello.New
	fmt.Printf("  boot-mode commit ok: profile=%s, empty commit list, profile-first revert list\n", committedClosure)

	// Vacuous-pass guard for the revert injection (build-time fixture
	// sanity): the OLD closure the revert re-runs must ship the executable
	// wrapper the activation_path override points at. The committed profile
	// IS that old closure, so probe it over SSH: a missing or
	// non-executable wrapper would make the revert's own OLD step
	// fork/exec-fail and degrade the leg into REVERT_FAILED.
	output, err := sshRun(nixosISOPort, res.keyPath,
		"test -x /nix/var/nix/profiles/system/etc/panix-e2e-boot-activation && echo wrapper-ok || { test -e /nix/var/nix/profiles/system/etc/panix-e2e-boot-activation && echo wrapper-not-executable || echo wrapper-missing; }")
	if err != nil {
		return errors.Wrap(err, "probe the committed boot closure's activation wrapper")
	}

	if !strings.Contains(output, "wrapper-ok") {
		return errors.Errorf("the committed boot closure does not ship the executable activation wrapper (fixture regression; probe output %q)", strings.TrimSpace(output))
	}

	// (b) Boot-mode revert: the same closure family deployed with the
	// activation_path override (panix.yml: etc/panix-e2e-boot-activation),
	// the wrapper the boot fixture's closure ships. The deploy closure does
	// not carry it, so the activation child fails immediately and the
	// boot-mode revert re-runs OLD's activation through it, which execs
	// OLD's real switch-to-configuration (configuration-boot closure).
	// Boot mode's effective gate is auto (nothing to confirm, spec 5), so a
	// magic-gate health check cannot be the trigger and the config validator
	// rejects one anyway; a failing activation is the designed reverting
	// injection.
	err = runGuardDeploy(configPath, "test-vm-boot-revert", "deploy-boot-revert", res)
	if err == nil {
		return errors.New("the boot-mode deploy with the nonexistent activation path was expected to revert")
	}

	// The failure must be the guarded revert, not an earlier config-load or
	// pre-start failure: assert the deploy log reached the guarded path
	// before accepting it as the expected outcome (the vacuous-failure trap:
	// a config-load error would fail this deploy too, and the record
	// assertions below would then run against the previous transaction's
	// slot log).
	bootRevertLogPath, bootLogErr := newestPanixLog("deploy-boot-revert")
	if bootLogErr != nil {
		return bootLogErr
	}

	bootRevertLog, bootReadErr := os.ReadFile(bootRevertLogPath) //nolint:gosec // repo-local test log
	if bootReadErr != nil {
		return errors.Wrapf(bootReadErr, "read panix deploy-boot-revert log %s", bootRevertLogPath)
	}

	if !strings.Contains(string(bootRevertLog), "guarded activation") {
		return errors.New("the boot-mode revert deploy never reached the guarded path (config load or pre-start failure)")
	}

	fmt.Printf("  boot-mode revert deploy failed as expected: %v\n", err)

	// The raw slot-log tail first: the parsed records below cannot name the
	// failing step's argv and errno, the narrative lines can.
	err = guardDumpSlotLogTail(res.keyPath, slotDir)
	if err != nil {
		return err
	}

	records, _, err = guardFetchRecords(res.keyPath, slotDir+"/log")
	if err != nil {
		return err
	}

	txn, ok = guardFindRecord(records, "TXN")
	if !ok {
		return errors.New("the boot-mode revert deploy's slot log carries no TXN record")
	}

	if txn.CA != "[]" {
		return errors.Errorf("the boot-mode revert deploy's commit list is not empty: %q", txn.CA)
	}

	hello, ok = guardFindRecord(records, "HELLO")
	if !ok {
		return errors.New("the boot-mode revert deploy's slot log carries no HELLO record")
	}

	if _, ok := guardFindRecord(records, "REVERTED"); !ok {
		return errors.New("the boot-mode deploy did not revert")
	}

	if err := guardAssertProfileClosure(res.keyPath, hello.Old); err != nil {
		return errors.Wrap(err, "boot-mode revert: the profile must be restored to the HELLO old closure")
	}

	if hello.Old != committedClosure {
		return errors.Errorf("the boot-mode revert restored %s, want the pre-leg closure %s", hello.Old, committedClosure)
	}

	// The auto-degraded boot gate never arms the confirm loop, so no remote
	// check ran and no confirm frame was written: the revert reason comes
	// from the activation outcome, not a confirm-side decision.
	if guardCountRecords(records, "CONFIRM_CONSUMED") != 0 {
		return errors.New("the boot-mode transaction consumed a confirm although its effective gate is auto")
	}

	fmt.Printf("  boot-mode revert ok: profile restored to %s\n", hello.Old)

	return nil
}

// runDeployGuardFlood covers spec-13 legs 16 and 11: the wire's drop-oldest
// buffer must overflow under backpressure while the transaction itself
// proceeds untouched through the confirm gate to the commit. Producer-side
// volume can never outrun the relay's drain (one write syscall per frame
// versus three per produced line), so the injection is consumer-side and
// deterministic: the deploy runs with a sequencing goroutine (the link-down
// pattern), the harness waits for the baseline-aware HELLO, SIGSTOPs the
// relay so the fd4 drain stalls (the fixture's pre-burst sleep gives the
// stop a window to land), holds the window during which the burst fills the
// 64KB pipe and then the 256-frame ring (LINK_DEGRADED is log-only and
// survives the stall), SIGCONTs, and only then reads the deploy's outcome.
func runDeployGuardFlood(configPath string, res *testResources) (err error) {
	printPhasef("Phase: Guard wire backpressure (consumer-stall; spec 13 legs 16+11)")

	slotDir, err := guardResolveSlotDir(res.keyPath, guardV2SlotGlob)
	if err != nil {
		return err
	}

	logPath := slotDir + "/log"

	baseline, err := captureGuardLogBaseline(res.keyPath, logPath)
	if err != nil {
		return err
	}

	// stallArmed tracks the window from the successful SIGSTOP on: every
	// exit path past it must CONT the relay (safety pin a), because a
	// leaked STOPped relay wedges every subsequent leg on this VM.
	stallArmed := false

	defer func() {
		if !stallArmed {
			return
		}

		for range guardResumeAttempts {
			resumeErr := guardResumeRelay(res.keyPath)
			if resumeErr == nil {
				return
			}

			time.Sleep(guardResumeRetryInterval)
		}

		leakErr := errors.New("the relay could not be CONTinued after the flood stall (a leaked STOPped relay wedges every subsequent leg on this VM)")
		if err != nil {
			err = errors.Wrap(err, leakErr.Error())
		} else {
			err = leakErr
		}
	}()

	var deployErr error

	deployDone := make(chan struct{})

	go func() {
		defer close(deployDone)

		deployErr = runGuardDeploy(configPath, "test-vm-flood", "deploy-flood", res)
	}()

	err = guardWaitForRecordFragment(res.keyPath, logPath, `"ev":"HELLO"`, baseline, guardWindowOpenWait)
	if err != nil {
		<-deployDone

		return err
	}

	err = guardStallRelay(res.keyPath)
	if err != nil {
		<-deployDone

		return errors.Wrap(err, "stall the guard relay")
	}

	stallArmed = true

	fmt.Printf("  relay stopped mid-window; the fixture burst runs against a stalled consumer\n")

	// Safety pin b: ~5s, far inside the machine's 30s activation_timeout.
	time.Sleep(guardStallWindow)

	err = guardResumeRelay(res.keyPath)
	if err != nil {
		<-deployDone

		return errors.Wrap(err, "resume the guard relay after the stall")
	}

	stallArmed = false // explicitly resumed; the deferred retry loop stays as the safety net

	fmt.Printf("  relay resumed; the stalled burst drains and the transaction continues\n")

	<-deployDone

	if deployErr != nil {
		return errors.Wrap(deployErr, "the flooded guarded deploy failed")
	}

	records, _, err := guardFetchRecords(res.keyPath, slotDir+"/log")
	if err != nil {
		return err
	}

	degraded := guardCountRecords(records, "LINK_DEGRADED")
	if degraded == 0 {
		return errors.New("the flood produced no LINK_DEGRADED record: the wire buffer never overflowed")
	}

	if _, ok := guardFindRecord(records, "COMMITTED"); !ok {
		return errors.New("the flooded transaction did not commit")
	}

	if guardCountRecords(records, "CONFIRM_CONSUMED") != 1 {
		return errors.New("the flooded transaction's confirm was not consumed exactly once")
	}

	fmt.Printf("  flood ok: %d LINK_DEGRADED records, transaction committed\n", degraded)

	return nil
}

// guardUserTierHMGenVersion probes the VM's home-manager generation for
// driver-1 support (the same contract the deployer's pre-start detection
// uses, run over SSH as the target user): integer >= 1 means the leg deploys
// a modern profile-last transaction.
func guardUserTierHMGenVersion(keyPath string) (string, bool) {
	script := `p=$(readlink -f "${XDG_STATE_HOME:-$HOME/.local/state}/nix/profiles/home-manager" 2>/dev/null) || true
[ -n "$p" ] || p="${NIX_STATE_DIR:-/nix/var/nix}/profiles/per-user/$USER/home-manager"
cat "$p/gen-version" 2>/dev/null || true
`

	output, err := sshRun(nixosISOPort, keyPath, "su -l guarduser -c "+guardShellQuote(script))
	if err != nil {
		return "", false
	}

	version := strings.TrimSpace(output)

	number, err := strconv.Atoi(version)
	if err != nil {
		return version, false
	}

	return version, number >= 1
}

// runGuardUserTierLeg covers spec-13 leg 6: a guarded user-tier deploy runs
// the whole relay window through the su -l wrapper, so the magic confirm
// riding the wire stdin through su -l is the forwarding proof: a successful
// guarded deploy with a consumed confirm means the duplex pipes survived the
// elevation. It runs after the home phase, which created guarduser's first
// profile generation (the guard route needs a rollback target).
func runGuardUserTierLeg(configPath string, res *testResources) error {
	printPhasef("Phase: Guard user tier (stdin forwarding through su -l; spec 13 leg 6)")

	_, hmModern := guardUserTierHMGenVersion(res.keyPath)
	fmt.Printf("  home-manager detection: gen-version >= 1 (profile-last) = %v\n", hmModern)

	err := runGuardDeploy(configPath, "test-home-guard", "deploy-home-guard", res)
	if err != nil {
		return errors.Wrap(err, "the guarded user-tier deploy failed")
	}

	slotDir, err := guardResolveSlotDir(res.keyPath, guardUserSlotGlob)
	if err != nil {
		return errors.Wrap(err, "no user-tier guard slot: the deploy did not route through the guard")
	}

	records, _, err := guardFetchRecords(res.keyPath, slotDir+"/log")
	if err != nil {
		return err
	}

	if guardCountRecords(records, "CONFIRM_CONSUMED") != 1 {
		return errors.New("the user-tier confirm was not consumed exactly once over the su -l wire")
	}

	if guardCountRecords(records, "CONFIRM_IGNORED") != 0 {
		return errors.New("the user-tier confirm was ignored (the frames did not reach the guardian through su -l)")
	}

	if _, ok := guardFindRecord(records, "COMMITTED"); !ok {
		return errors.New("the user-tier transaction did not commit")
	}

	txn, ok := guardFindRecord(records, "TXN")
	if !ok {
		return errors.New("the user-tier transaction's slot log carries no TXN record")
	}

	// Modern HM composes profile-last (spec 4.2 tier table): the driver-1
	// activation and the --set commit are the tier's evidence. Legacy HM
	// keeps today's assertions unchanged (no TXN shape pin existed).
	if hmModern {
		if !strings.Contains(txn.AA, "--driver-version") || !strings.Contains(txn.AA, `"1"`) {
			return errors.Errorf("the modern user-tier activation lost the driver-1 argument: %q", txn.AA)
		}

		if !strings.Contains(txn.CA, "--set") {
			return errors.Errorf("the modern user-tier commit list lost the profile set: %q", txn.CA)
		}
	}

	marker, err := sshRun(nixosISOPort, res.keyPath, guardUserMarkerCommand)
	if err != nil || strings.TrimSpace(marker) != guardUserMarkerContent {
		return errors.Errorf("the guarduser home marker is not live (got %q, err %v)", marker, err)
	}

	fmt.Printf("  user tier ok: confirm consumed through su -l, marker %s live\n", guardUserMarkerContent)

	return nil
}
