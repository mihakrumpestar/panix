package executioner

import (
	"github.com/mihakrumpestar/panix/pkg/shellquote"
	"github.com/mihakrumpestar/panix/pkg/ssh"
)

// sshStream transports commandWithArgs over SSH. The boundary re-parses the
// joined command line through the remote shell, so callers build plain argv
// (never pre-escaped) and every element, sh -c and su -c strings included,
// is single-quoted as one literal (pkg/shellquote) that arrives unchanged.
//
// The streaming exec requests no remote PTY: no -t/-tt is passed, so the
// remote command runs without a controlling terminal and the local PTY is
// only the transport in front of the ssh child. Non-interactive elevation
// prerequisite that downstream guard wiring must respect: sudo without a tty
// fails fast on a password prompt, and su -l without a remote tty reads the
// password from stdin, which would consume piped control frames. This path
// is only usable with passwordless elevation (NOPASSWD sudo or direct root
// login), never with interactive password prompts.
func (ex *Executioner) sshStream(description, statusIfRunning, statusIfFailed string, commandWithArgs []string, excOpt *ExecOptions) error {
	remoteArgv := sshCommandWithArgsOptions(
		ex.conf.Machine.GetActiveSSH(),
		commandWithArgs,
		sshArgvOptions{freshConnection: excOpt.freshConnection},
	)

	return ex.shellStream(description, statusIfRunning, statusIfFailed, remoteArgv, excOpt)
}

// sshArgvOptions tweaks the ssh argv composition beyond the defaults.
type sshArgvOptions struct {
	// freshConnection appends -o ControlMaster=no -o ControlPath=none after
	// MaybeSSHCommandArguments: later -o flags override earlier ones, so the
	// command opens a dedicated connection instead of the multiplexed master.
	freshConnection bool
}

// sshCommandWithArgs builds the local ssh argv (flags, target, quoted command
// arguments) that runs commandWithArgs on the remote machine with the default
// connection behavior. Tilde paths keep their tilde bare
// (shellquote.QuoteWord) so the remote login shell expands preset profile
// paths such as ~/.local/state/nix/profiles/....
func sshCommandWithArgs(sshClient ssh.SSHClient, commandWithArgs []string) []string {
	return sshCommandWithArgsOptions(sshClient, commandWithArgs, sshArgvOptions{})
}

// sshCommandWithArgsOptions is sshCommandWithArgs with argv tweaks. The
// fresh-connection options are appended after the client arguments and stay
// before the target, keeping the argv valid while overriding the
// multiplexing defaults.
func sshCommandWithArgsOptions(sshClient ssh.SSHClient, commandWithArgs []string, opts sshArgvOptions) []string {
	out := []string{"ssh", "-o", "LogLevel=ERROR"}
	out = append(out, sshClient.MaybeSSHCommandArguments()...)

	if opts.freshConnection {
		out = append(out, sshClient.FreshConnectionArgs()...)
	}

	out = append(out, sshClient.SSHTarget())

	for _, arg := range commandWithArgs {
		out = append(out, shellquote.QuoteWord(arg))
	}

	return out
}
