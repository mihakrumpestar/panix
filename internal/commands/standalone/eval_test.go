package commands_standalone

import (
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEvalResolvesFlagDefaults pins that eval declares the flag groups config
// loading reads. A missing group leaves its `default` tags unresolved, so a valid
// config fails eval with an empty theme name.
func TestEvalResolvesFlagDefaults(t *testing.T) {
	t.Parallel()

	var parsed struct{ EvalCmd }

	_, err := kong.Must(&parsed, kong.NoDefaultHelp()).Parse(nil)
	require.NoError(t, err)

	assert.Equal(t, "panix", parsed.Tui.Theme)
	assert.Equal(t, "panix.log", parsed.Logging.LogFile)
}
