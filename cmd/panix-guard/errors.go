package main

import "errors"

// Sentinel errors for the guard verbs (err113: dynamic errors are defined
// once here and wrapped with %w at the call sites; the static text carries
// the verb context, the dynamic detail rides the wrap).

var (
	// errInspectUsage marks an inspect argv contract violation.
	errInspectUsage = errors.New("inspect: --dir is required")
	// errConvergeUsage marks a converge argv contract violation.
	errConvergeUsage = errors.New("converge: --dir is required")
	// errAttachUsage marks an attach argv contract violation.
	errAttachUsage = errors.New("attach: --dir and --key are required")
	// errCtlUsage marks a ctl argv contract violation.
	errCtlUsage = errors.New("ctl: --dir, --key and exactly one of confirm|revert-request are required")
	// errCtlUnknownCommand marks an unrecognized ctl command name.
	errCtlUnknownCommand = errors.New("ctl: unknown command (want confirm|revert-request)")
	// errStartUsage marks a start argv contract violation (missing required flags).
	errStartUsage = errors.New("start: --dir, --key, --new and --tier are required")
	// errStartUnknownTier marks an unknown tier class.
	errStartUnknownTier = errors.New("start: unknown tier")
	// errStartUnknownGate marks an unknown confirmation gate.
	errStartUnknownGate = errors.New("start: unknown confirmation gate")
	// errStartActivationSteps marks an activation argv that is not exactly one step.
	errStartActivationSteps = errors.New("start: --activation-argv must contain exactly one step")
	// errArgvStepEmpty marks an argv step without a command.
	errArgvStepEmpty = errors.New("argv step is empty")
	// errParseJSONStrings marks a malformed JSON string array payload.
	errParseJSONStrings = errors.New("parse JSON strings")
	// errParseArgvSteps marks a malformed JSON argv-array payload.
	errParseArgvSteps = errors.New("parse argv steps")
	// errGuardianPanic marks the top-level recover of a guardian panic.
	errGuardianPanic = errors.New("guardian panic")
	// errSlotOwner marks a slot directory owned by another identity.
	errSlotOwner = errors.New("slot dir is owned by another identity")
	// errStartMutation marks a failed start-phase mutation (the reason and
	// the FAILED_PRECONDITION record carry the detail).
	errStartMutation = errors.New("start mutation failed")
	// errSweepNotConverged marks a pre-start sweep whose convergence failed.
	errSweepNotConverged = errors.New("previous transaction did not converge")
	// errMutationStep marks a failed mutation step execution.
	errMutationStep = errors.New("mutation step failed")
	// errLogUnwritable marks a slot log that cannot be opened for the writer.
	errLogUnwritable = errors.New("slot log is not writable")
	// errTailScan marks a slot-log tail that could not be read for classification.
	errTailScan = errors.New("scan log tail")
)
