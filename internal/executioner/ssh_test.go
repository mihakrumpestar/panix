package executioner

import (
	"testing"

	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/stretchr/testify/assert"
)

// Pins the quoting shape: ssh flags, then the target, then one quoted word
// per argv element.
func TestSSHCommandWithArgs(t *testing.T) {
	t.Parallel()

	client := ssh.SSHClient{Hostname: "10.0.0.1", Port: 22, Username: "deploy"}

	// Internal fields default to zero; MaybeSSHCommandArguments fills -l/-p
	// plus the always-on ControlMaster/host-key options.
	got := sshCommandWithArgs(client, []string{
		"nix", "--extra-experimental-features", "nix-command flakes", "build",
	})

	// Client options between head and target are environment-dependent
	// (ControlPath uses os.TempDir), so only the stable parts are asserted.
	assert.Equal(t, []string{"ssh", "-o", "LogLevel=ERROR", "-l", "deploy", "-p", "22"}, got[:7])
	assert.Equal(t, "10.0.0.1", got[len(got)-5])
	assert.Equal(t, []string{
		`'nix'`, `'--extra-experimental-features'`, `'nix-command flakes'`, `'build'`,
	}, got[len(got)-4:])
}

// Pins the safety property: shell-active characters arrive as literals inside
// single quotes, so they can never re-execute or redirect.
func TestSSHCommandWithArgs_ShellMetacharactersStayLiteral(t *testing.T) {
	t.Parallel()

	client := ssh.SSHClient{Hostname: "host", Port: 22, Username: "root"}

	got := sshCommandWithArgs(client, []string{
		"sh", "-c", "echo hi > /tmp/f",
		"echo", "$HOME", `say "hello"`, "it's",
	})

	tail := got[len(got)-7:]
	assert.Equal(t, []string{
		`'sh'`, `'-c'`, `'echo hi > /tmp/f'`,
		`'echo'`, `'$HOME'`, `'say "hello"'`, `'it'\''s'`,
	}, tail)
}

// Pins the tilde rule: inert ~/ paths stay bare so the remote login shell
// expands them. Load-bearing for the preset profile paths, valid only after
// expansion (a quoted tilde resolves relative to the CWD); tilde paths with
// shell-active characters still fall back to quoting.
func TestSSHCommandWithArgs_TildeStaysBare(t *testing.T) {
	t.Parallel()

	client := ssh.SSHClient{Hostname: "host", Port: 22, Username: "root"}

	got := sshCommandWithArgs(client, []string{
		"nix-env", "--profile", "~/.local/state/nix/profiles/home-manager", "--list-generations",
		"weird", "~/a b",
	})

	tail := got[len(got)-6:]
	assert.Equal(t, []string{
		`'nix-env'`, `'--profile'`, `~/.local/state/nix/profiles/home-manager`, `'--list-generations'`,
		`'weird'`, `'~/a b'`,
	}, tail)
}
