package phaseops

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Env pairs must land inside the su -c string, after the XDG prefix and
// shell-quoted, so the login shell applies them after its env reset.
func TestWithEnv_AsUserComposition(t *testing.T) {
	t.Parallel()

	cmd := executioner.WithEnv([]string{"NIX_CONFIG=a b"}, []string{"nix-env", "--list-generations"})
	result := asUser("alice", cmd)

	require.Len(t, result, 5)

	assert.Equal(t, `XDG_RUNTIME_DIR=/run/user/$(id -u) 'env' 'NIX_CONFIG=a b' 'nix-env' '--list-generations'`, result[4])
}
