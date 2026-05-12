package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoinCommand_Empty(t *testing.T) {
	t.Parallel()

	lb := joinCommand(nil, nil)

	assert.Empty(t, lb.Bytes())
}

func TestJoinCommand_CommandOnly(t *testing.T) {
	t.Parallel()

	lb := joinCommand(nil, []string{"echo", "hello"})

	assert.Equal(t, "echo hello", string(lb.Bytes()))
}

func TestJoinCommand_CommandWithSpaces(t *testing.T) {
	t.Parallel()

	lb := joinCommand(nil, []string{"echo", "hello world"})

	assert.Equal(t, "echo 'hello world'", string(lb.Bytes()))
}

func TestJoinCommand_CommandWithQuotes(t *testing.T) {
	t.Parallel()

	lb := joinCommand(nil, []string{"echo", `it's "fine"`})

	assert.Equal(t, `echo 'it's "fine"'`, string(lb.Bytes()))
}

func TestJoinCommand_EnvOnly(t *testing.T) {
	t.Parallel()

	lb := joinCommand([]string{"MY_VAR=simple"}, nil)

	assert.Equal(t, "MY_VAR=simple", string(lb.Bytes()))
}

func TestJoinCommand_EnvWithSpaces(t *testing.T) {
	t.Parallel()

	lb := joinCommand([]string{"MY_VAR=some some", "OTHER=here"}, nil)

	assert.Equal(t, "MY_VAR='some some' OTHER=here", string(lb.Bytes()))
}

func TestJoinCommand_EnvWithValueQuoting(t *testing.T) {
	t.Parallel()

	lb := joinCommand([]string{"PATH=/usr/bin:/my dir/bin"}, nil)

	assert.Equal(t, "PATH='/usr/bin:/my dir/bin'", string(lb.Bytes()))
}

func TestJoinCommand_EnvMalformedNoEquals(t *testing.T) {
	t.Parallel()

	lb := joinCommand([]string{"JUST_A_WORD"}, nil)

	assert.Equal(t, "JUST_A_WORD", string(lb.Bytes()))
}

func TestJoinCommand_EnvMalformedNoEqualsWithSpaces(t *testing.T) {
	t.Parallel()

	lb := joinCommand([]string{"HAS SPACE"}, nil)

	assert.Equal(t, "'HAS SPACE'", string(lb.Bytes()))
}

func TestJoinCommand_EnvEmptyValue(t *testing.T) {
	t.Parallel()

	lb := joinCommand([]string{"EMPTY_VAR="}, nil)

	assert.Equal(t, "EMPTY_VAR=", string(lb.Bytes()))
}

func TestJoinCommand_EnvAndCommand(t *testing.T) {
	t.Parallel()

	lb := joinCommand(
		[]string{"FOO=bar", "BAZ=qux qux"},
		[]string{"nix", "build", ".#foo"},
	)

	assert.Equal(t, "FOO=bar BAZ='qux qux' nix build .#foo", string(lb.Bytes()))
}

func TestJoinCommand_MultipleEnv(t *testing.T) {
	t.Parallel()

	lb := joinCommand(
		[]string{"A=1", "B=2", "C=3"},
		[]string{"cmd"},
	)

	assert.Equal(t, "A=1 B=2 C=3 cmd", string(lb.Bytes()))
}

func TestJoinCommand_EnvValueWithTab(t *testing.T) {
	t.Parallel()

	lb := joinCommand([]string{"TAB=\tval"}, nil)

	assert.Equal(t, "TAB='\tval'", string(lb.Bytes()))
}

func TestJoinCommand_EqualsInValue(t *testing.T) {
	t.Parallel()

	lb := joinCommand([]string{"OPTS=--flag=value"}, nil)

	assert.Equal(t, "OPTS=--flag=value", string(lb.Bytes()))
}

func TestNewCommandLog_Fields(t *testing.T) {
	t.Parallel()

	commandLog := NewCommandLog("desc", "running", "failed", []string{"ls", "-la"}, []string{"ENV=val"})

	require.NotNil(t, commandLog)
	assert.Equal(t, "desc", commandLog.Description)
	assert.Equal(t, "running", commandLog.StatusIfRunning)
	assert.Equal(t, "failed", commandLog.StatusIfFailed)
	assert.Equal(t, "ENV=val ls -la", string(commandLog.Command.Bytes()))
	assert.NotNil(t, commandLog.Output)
	assert.NotNil(t, commandLog.TimeAndState)
}

func TestNewCommandLog_NilCommandAndEnv(t *testing.T) {
	t.Parallel()

	cl := NewCommandLog("desc", "running", "failed", nil, nil)

	require.NotNil(t, cl)
	assert.Empty(t, cl.Command.Bytes())
}
