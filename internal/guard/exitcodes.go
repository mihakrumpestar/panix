package guard

// Per-verb exit-code blocks (spec 9.5). The same numeric code means different
// things per verb by design; per-verb scoping makes the verified v1.3
// collisions explicit (3 = revert_failed / lock-held / guardian-dead across
// verbs; 5 = activation-exited / refused). This file is the single source of
// truth for both panix-guard and the panix deployer side.

// Guardian outcome vocabulary: the exit codes of start, the relay's terminal
// classification, and attach's terminal-record mapping.
const (
	// ExitCommitted: the transaction committed.
	ExitCommitted = 0
	// ExitReverted: the transaction reverted to the previous generation.
	ExitReverted = 2
	// ExitRevertFailed: the revert itself failed; operator action required.
	ExitRevertFailed = 3
	// ExitFailedPrecondition: the transaction aborted with zero effects.
	ExitFailedPrecondition = 4
	// ExitActivationExited: minimal tier; the activation ran and exited, no
	// commit or revert applies.
	ExitActivationExited = 5
	// ExitCancelled: no terminal state was observed; the outcome is unknown and
	// the deployer runs inline convergence.
	ExitCancelled = 6
	// ExitUsage: argv contract violation.
	ExitUsage = 64
)

// ctl verb: control delivery over a degraded (wire-dead) connection.
const (
	// CtlExitConsumed: the request was delivered and acked.
	CtlExitConsumed = 0
	// CtlExitDead: the guardian is dead (liveness check failed).
	CtlExitDead = 3
	// CtlExitNoSlot: the slot or its log is missing.
	CtlExitNoSlot = 4
	// CtlExitRefused: the state machine refused the request (CONFIRM_IGNORED,
	// terminal-state ack, status-late ack after COMMIT_START; always on the
	// auto tier for confirm).
	CtlExitRefused = 5
	// CtlExitAckTimedOut: the request was delivered but no ack arrived within
	// --wait; the request may still take effect.
	CtlExitAckTimedOut = 7
)

// converge verb: offline convergence (classify, converge, truncate).
const (
	// ConvergeExitConverged: converged to a terminal state, or the slot was
	// already terminal and was cleared.
	ConvergeExitConverged = 0
	// ConvergeExitFailed: convergence failed; the log is left untouched.
	ConvergeExitFailed = 1
	// ConvergeExitLocked: a live transaction holds the lock; no mutation.
	ConvergeExitLocked = 3
)

// inspect verb: one JSON verdict of the derived slot state.
const (
	// InspectExitOK: the verdict was printed.
	InspectExitOK = 0
	// InspectExitUnparseable: the log is unreadable or unparseable beyond
	// noise (the verdict still reports the lock state).
	InspectExitUnparseable = 1
	// InspectExitNoSlot: the slot or its log is missing.
	InspectExitNoSlot = 4
)

// Legacy lock probe (pre-v2 binary only; the v2 binary deletes the verb and
// inspect carries the lock state).
const (
	// LockProbeExitFree: no live transaction.
	LockProbeExitFree = 0
	// LockProbeExitLive: a live transaction holds the lock.
	LockProbeExitLive = 3
)
