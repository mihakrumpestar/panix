package executioner

import (
	"github.com/mihakrumpestar/panix/pkg/shellquote"
	"github.com/mihakrumpestar/panix/pkg/ssh"
)

// sshStream transports commandWithArgs over SSH. The boundary re-parses the
// joined command line through the remote shell, so callers build plain argv
// (never pre-escaped) and every element, sh -c and su -c strings included,
// is single-quoted as one literal (pkg/shellquote) that arrives unchanged.
func (ex *Executioner) sshStream(description, statusIfRunning, statusIfFailed string, commandWithArgs []string, excOpt *ExecOptions) error {
	sshCommandWithArgs := sshCommandWithArgs(ex.conf.Machine.GetActiveSSH(), commandWithArgs)

	return ex.shellStream(description, statusIfRunning, statusIfFailed, sshCommandWithArgs, excOpt)
}

// sshCommandWithArgs builds the local ssh argv (flags, target, quoted command
// arguments) that runs commandWithArgs on the remote machine. Tilde paths keep
// their tilde bare (shellquote.QuoteWord) so the remote login shell expands
// preset profile paths such as ~/.local/state/nix/profiles/....
func sshCommandWithArgs(sshClient ssh.SSHClient, commandWithArgs []string) []string {
	out := []string{"ssh", "-o", "LogLevel=ERROR"}
	out = append(out, sshClient.MaybeSSHCommandArguments()...)
	out = append(out, sshClient.SSHTarget())

	for _, arg := range commandWithArgs {
		out = append(out, shellquote.QuoteWord(arg))
	}

	return out
}
