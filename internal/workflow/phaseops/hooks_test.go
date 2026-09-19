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

	hooks := []attributes.PostBootstrapHookCommand{
		"echo hello",
		"touch /tmp/marker && echo done",
	}
	require.NoError(t, exc.ExecuteHooks(hooks, "test hook"))

	lines := testutil.CommandLines(t, phaseLog)
	require.Len(t, lines, 2)
	assert.Equal(t, "sh -c echo hello", lines[0])
	assert.Equal(t, "sh -c touch /tmp/marker && echo done", lines[1])
}
