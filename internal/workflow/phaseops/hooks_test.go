package phaseops

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExecuteHooks_ShellStringsRunViaSh pins the sh -c conversion: hook command
// strings are shell programs and must run as an explicit sh -c argv, which
// behaves identically on local and remote transports. Lives in phaseops because
// the executioner package cannot import testutil (import cycle).
func TestExecuteHooks_ShellStringsRunViaSh(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

	hooks := []attributes.HookCommand{
		"echo hello",
		"touch /tmp/marker && echo done",
	}
	require.NoError(t, exc.ExecuteHooks(hooks, "test hook", nil))

	lines := testutil.CommandLines(t, phaseLog)
	require.Len(t, lines, 2)
	assert.Equal(t, "sh -c echo hello", lines[0])
	assert.Equal(t, "sh -c touch /tmp/marker && echo done", lines[1])
}

// Hook environment variables must ride an env(1) prefix before sh -c, so
// they reach the hook process on both transports; nil env keeps the bare
// sh -c argv (bootstrap hooks pass none).
func TestExecuteHooks_EnvExportedViaEnvPrefix(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

	hooks := []attributes.HookCommand{"echo hello"}
	env := []string{"PANIX_HOOK_PHASE=pre", "PANIX_CLOSURE=/nix/store/abc"}
	require.NoError(t, exc.ExecuteHooks(hooks, "activation hook", env))

	lines := testutil.CommandLines(t, phaseLog)
	require.Len(t, lines, 1)
	assert.Equal(t, "env PANIX_HOOK_PHASE=pre PANIX_CLOSURE=/nix/store/abc sh -c echo hello", lines[0])
}
