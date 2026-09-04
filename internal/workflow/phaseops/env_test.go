package phaseops

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithEnv(t *testing.T) {
	t.Parallel()

	command := []string{"nix", "profile", "add", "/nix/store/abc"}

	tests := []struct {
		name string
		env  []string
		want []string
	}{
		{
			name: "nil env returns command unchanged",
			env:  nil,
			want: command,
		},
		{
			name: "empty env returns command unchanged",
			env:  []string{},
			want: command,
		},
		{
			name: "single pair prefixed via env(1)",
			env:  []string{"NIX_CONFIG=extra-experimental-features = ca-derivations"},
			want: []string{"env", "NIX_CONFIG=extra-experimental-features = ca-derivations", "nix", "profile", "add", "/nix/store/abc"},
		},
		{
			name: "multiple pairs keep order",
			env:  []string{"A=1", "B=2"},
			want: []string{"env", "A=1", "B=2", "nix", "profile", "add", "/nix/store/abc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := WithEnv(tt.env, command)
			assert.Equal(t, tt.want, got)
		})
	}
}

// WithEnv must return a new slice, never mutating or aliasing command.
func TestWithEnv_DoesNotMutateCommand(t *testing.T) {
	t.Parallel()

	command := []string{"nix", "build"}
	spare := []string{"nix", "build"}

	_ = WithEnv([]string{"A=1"}, command)

	assert.Equal(t, spare, command)
}

// Env pairs must land inside the su -c string, after the XDG prefix and
// shell-quoted, so the login shell applies them after its env reset.
func TestWithEnv_AsUserComposition(t *testing.T) {
	t.Parallel()

	cmd := WithEnv([]string{"NIX_CONFIG=a b"}, []string{"nix-env", "--list-generations"})
	result := AsUser("alice", cmd)

	require.Len(t, result, 5)

	inner := strings.Trim(result[4], `"`)

	assert.Contains(t, inner, `env 'NIX_CONFIG=a b' nix-env`)
}
