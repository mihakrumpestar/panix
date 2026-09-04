package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/pkg/xpath"
)

func TestJoinCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  []string
		want string
	}{
		{"nil", nil, ""},
		{"plain", []string{"nix", "build", ".#foo"}, "nix build .#foo"},
		{"arg with spaces", []string{"echo", "hello world"}, "echo 'hello world'"},
		{"arg with quotes", []string{"echo", `it's "fine"`}, `echo 'it's "fine"'`},
		{"env argv", []string{"env", "NIX_PAGER=cat", "nix-env"}, "env NIX_PAGER=cat nix-env"},
		{"env argv with spaces", []string{"env", "NIX_CONFIG=a b", "nix", "build"}, "env 'NIX_CONFIG=a b' nix build"},
		{"env argv with equals", []string{"env", "OPTS=--flag=value", "cmd"}, "env OPTS=--flag=value cmd"},
		{"env argv empty value", []string{"env", "EMPTY=", "cmd"}, "env EMPTY= cmd"},
		{"env argv with tab", []string{"env", "TAB=\tval", "cmd"}, "env 'TAB=\tval' cmd"},
		{"multiple env argv", []string{"env", "A=1", "B=2", "cmd"}, "env A=1 B=2 cmd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, string(joinCommand(tt.cmd).Bytes()))
		})
	}
}

func TestNewCommandLog_Fields(t *testing.T) {
	t.Parallel()

	commandLog := NewCommandLog(xpath.New("test"), "desc", "running", "failed", []string{"env", "NIX_PAGER=cat", "ls", "-la"})

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

	cl := NewCommandLog(xpath.New("test"), "desc", "running", "failed", nil)

	require.NotNil(t, cl)
	assert.Empty(t, cl.Command.Bytes())
}
