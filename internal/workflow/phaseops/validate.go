package phaseops

import (
	"os/user"
	"strconv"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/pkg/errors"
)

// RefreshSuperuser probes whether the SSH user of the active connection is root
// and stores it in MetaInspect.IsRoot. Activate re-probes because the connection
// may have switched since Inspect (kexec, bootstrap reboot), and every
// elevation decision keys off the stored value.
func RefreshSuperuser(exc *executioner.Executioner, mach *machine.Machine) error {
	err := exc.Exec(
		"superuser check",
		"checking superuser privileges",
		"checking superuser failed",
		[]string{"id", "-u"},
		executioner.OnFailure(func(log *command.CommandLog, err error) error {
			return errors.Wrap(err, log.Output.String())
		}),
		executioner.OnSuccess(func(log *command.CommandLog) error {
			output := strings.Trim(log.Output.String(), "\n ")

			parsedOutput, err := strconv.ParseUint(output, 10, 64)
			if err != nil {
				return errors.Wrapf(err, "failed to parse raw output %s to uint", strconv.Quote(output))
			}

			mach.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.IsRoot = parsedOutput == 0
				mi.IsRootProbed = true
			})

			return nil
		}),
		executioner.OnDryRun(func() {
			// A dry-run probe has no real answer: keep an already-probed
			// real value (a real Inspect under --dry-run-with-inspect)
			// instead of a placeholder that would make the preview lie
			// about the elevation a real run would use.
			mach.MetaInspect.Update(func(mi *machine.MetaInspect) {
				if !mi.IsRootProbed {
					mi.IsRoot = true
				}
			})
		}),
	)
	if err != nil {
		return errors.Wrap(err, "superuser check failed")
	}

	return nil
}

// ValidateTargetUser enforces the su -l precondition: only a root executor
// (su -l from root never prompts) or no wrap at all works, because su -l
// prompts even for the same non-root user and panix cannot supply a password on
// the PTY (failure mode: a hang until the command timeout). The normalized
// target user is used, so system-level "root" never trips the check.
//
// The executing user is the SSH username; on local machines that username is a
// config default, so user.Current is compared instead.
func ValidateTargetUser(inst *installable.Installable, mach *machine.Machine) error {
	targetUser := NormalizedTargetUser(inst.Preset, inst.User)
	if targetUser == "" {
		return nil
	}

	mi := mach.MetaInspect.Load()
	if mi != nil && mi.IsRoot {
		return nil
	}

	executingUser := mach.GetActiveSSH().Username
	if mach.SSH.IsLocal() {
		current, err := user.Current()
		if err != nil {
			return errors.Wrap(err, "failed to determine local user for target user validation")
		}

		executingUser = current.Username
	}

	if executingUser == targetUser {
		return errors.Errorf(
			"%s: target user %q equals the executing user %q, but su -l would still prompt for that user's password; unset user to run as %q, or SSH as root",
			inst.Xpath, targetUser, executingUser, targetUser,
		)
	}

	return errors.Errorf(
		"%s: target user %q requires SSHing as root (su -l would prompt for a password panix cannot supply); executing user is %q",
		inst.Xpath, targetUser, executingUser,
	)
}
