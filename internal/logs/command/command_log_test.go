package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/pkg/xpath"
)

func TestJoinCommand(t *testing.T) {
	t.Parallel()

	// Rendered lines are the verbatim argv with recognized secret tokens
	// redacted and transport-quoted elements as-is.
	tests := []struct {
		name string
		cmd  []string
		want string
	}{
		{"nil", nil, ""},
		{"plain", []string{"nix", "build", ".#foo"}, "nix build .#foo"},
		{"non-secret env argv", []string{"env", "A=1", "B=2", "nix-env"}, "env A=1 B=2 nix-env"},
		{"secret env argv", []string{"env", "NIX_CONFIG=access-tokens=abc", "nix-env"}, "env NIX_CONFIG=access-tokens=*** nix-env"},
		{"sudo env argv", []string{"sudo", "env", "NIX_CONFIG=access-tokens=abc", "nix", "build"},
			"sudo env NIX_CONFIG=access-tokens=*** nix build"},
		{"transport-quoted env argv", []string{"ssh", "host", "'env'", "'NIX_CONFIG=access-tokens=abc'", "'nix'"},
			"ssh host 'env' 'NIX_CONFIG=access-tokens=***' 'nix'"},
		{"joined su -c env assignment is redacted",
			[]string{"su", "-l", "bob", "-c", `XDG_RUNTIME_DIR=/run/user/$(id -u) 'sudo' 'env' 'NIX_CONFIG=access-tokens=abc' 'nix' 'build'`},
			`su -l bob -c XDG_RUNTIME_DIR=/run/user/$(id -u) 'sudo' 'env' 'NIX_CONFIG=access-tokens=***' 'nix' 'build'`},
		{"joined su -c without env stays verbatim",
			[]string{"su", "-l", "bob", "-c", `XDG_RUNTIME_DIR=/run/user/$(id -u) 'echo' 'A=B'`},
			`su -l bob -c XDG_RUNTIME_DIR=/run/user/$(id -u) 'echo' 'A=B'`},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, string(joinCommand(testCase.cmd).Bytes()))
		})
	}
}

func TestJoinCommand_DoesNotMutateArgv(t *testing.T) {
	t.Parallel()

	cmd := []string{"sudo", "env", "NIX_CONFIG=access-tokens=abc", "nix", "build"}
	want := []string{"sudo", "env", "NIX_CONFIG=access-tokens=abc", "nix", "build"}

	_ = joinCommand(cmd)

	assert.Equal(t, want, cmd, "redaction must not change the argv used for execution")
}

func TestNewCommandLog_Fields(t *testing.T) {
	t.Parallel()

	commandLog := NewCommandLog(xpath.New("test"), 0, "desc", "running", "failed", []string{"env", "NIX_PAGER=cat", "ls", "-la"})

	require.NotNil(t, commandLog)
	assert.Equal(t, "desc", commandLog.Description)
	assert.Equal(t, "running", commandLog.StatusIfRunning)
	assert.Equal(t, "failed", commandLog.StatusIfFailed)
	assert.Equal(t, "env NIX_PAGER=cat ls -la", string(commandLog.Command.Bytes()))
	assert.NotNil(t, commandLog.Output)
	assert.NotNil(t, commandLog.TimeAndState)
}

func TestNewCommandLog_NilCommand(t *testing.T) {
	t.Parallel()

	cl := NewCommandLog(xpath.New("test"), 0, "desc", "running", "failed", nil)

	require.NotNil(t, cl)
	assert.Empty(t, cl.Command.Bytes())
}
