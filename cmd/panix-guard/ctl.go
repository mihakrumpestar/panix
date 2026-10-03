package main

import (
	"flag"
	"fmt"
	"github.com/mihakrumpestar/panix/internal/guard"
	"os"
	"syscall"
	"time"
)

// runCtl implements the control channel (spec 8): locate the guardian, deliver
// SIGUSR1 (confirm) or SIGUSR2 (revert-request), and wait for the matching record.
// Never signals before a HELLO record exists (handlers are installed before it).
func runCtl(args []string) error {
	fs := flag.NewFlagSet("ctl", flag.ContinueOnError)
	dir := fs.String("dir", "", "slot directory")
	key := fs.String("key", "", "per-deploy key (deploy nonce)")

	wait := fs.Duration("wait", 5*time.Second, "how long to wait for the matching marker")

	// DESIGN NOTE (binding, spec 8): --cause is narration-only by design. The
	// guardian cannot learn the cause: signals carry no payload, and
	// log-appended command records were rejected at spec level (the in-place
	// front-truncation window destroys concurrent appends). So no record
	// field can carry a cancel-vs-failed distinction; the deployer makes it
	// in its own outcome reporting (panixGiveUpCause), and the record carries
	// the revert's own reason. The flag never changes ctl's exit-code
	// contract (0/5/3/4/7).
	cause := fs.String("cause", "", "free-form cause narration for the request (diagnostics only; signals cannot carry it)")

	err := fs.Parse(args)
	if err != nil {
		return err
	}

	rest := fs.Args()
	if *dir == "" || *key == "" || len(rest) != 1 {
		return errCtlUsage
	}

	if rest[0] != "confirm" && rest[0] != "revert-request" {
		return fmt.Errorf("%w: %q", errCtlUnknownCommand, rest[0])
	}

	if *cause != "" {
		// Non-empty causes ride ctl's own stderr narration (single line, the
		// diagnostic channel): the exit-code contract is untouched.
		fmt.Fprintf(os.Stderr, "panix-guard: ctl %s (cause: %s)\n", rest[0], *cause)
	}

	os.Exit(ctlSend(*dir, *key, rest[0], *wait))

	return nil
}

// ctlSend delivers the request and waits for the matching record (spec 8):
// confirm is consumed by CONFIRM_CONSUMED (or an already-committed outcome),
// refused by CONFIRM_IGNORED or a revert outcome; revert-request is consumed
// by REVERT_REQUESTED or REVERT_START. The exit codes are the per-verb ctl
// block (spec 9.5): an ack timeout is its own outcome (7): the request was
// delivered but no ack arrived within --wait; the caller re-derives the
// outcome from the log, and the request may still take effect
// (pending-confirm semantics).
func ctlSend(dir, key, command string, wait time.Duration) int {
	path := slotLogPath(dir)
	if _, err := os.Stat(path); err != nil {
		return guard.CtlExitNoSlot
	}

	folded, err := guard.ScanFile(path, key)
	if err != nil || !folded.Found {
		return guard.CtlExitDead
	}

	if folded.PID <= 0 || !guardianAlive(folded.PID, key) {
		return guard.CtlExitDead
	}

	sig := syscall.SIGUSR2
	if command == "confirm" {
		sig = syscall.SIGUSR1
	}

	if err := syscall.Kill(folded.PID, sig); err != nil {
		return guard.CtlExitDead
	}

	st, err := os.Stat(path)
	if err != nil {
		return guard.CtlExitNoSlot
	}

	offset := st.Size()

	var partial string

	deadline := time.Now().Add(wait)

	for {
		lines, err := readNewLines(path, &offset, &partial)
		if err != nil {
			return guard.CtlExitNoSlot
		}

		for _, line := range lines {
			r, ok := guard.ParseRecordLine(line, key)
			if !ok {
				continue
			}

			if command == "confirm" {
				switch r.Ev {
				case guard.EventConfirmConsumed:
					return guard.CtlExitConsumed
				case guard.EventConfirmIgnored:
					return guard.CtlExitRefused
				case guard.EventCommitted:
					return guard.CtlExitConsumed // committed without our ack line: consumed
				case guard.EventReverted, guard.EventRevertFailed:
					return guard.CtlExitRefused // a revert outcome refused the confirm
				}
			} else {
				switch r.Ev {
				case guard.EventRevertRequested, guard.EventRevertStart:
					return guard.CtlExitConsumed
				case guard.EventCommitted:
					return guard.CtlExitRefused // the commit won the race
				}
			}
		}

		if !time.Now().Before(deadline) {
			fmt.Fprintln(os.Stderr, "panix-guard: ctl ack timeout; the request may still take effect")

			return guard.CtlExitAckTimedOut
		}

		time.Sleep(100 * time.Millisecond)
	}
}
