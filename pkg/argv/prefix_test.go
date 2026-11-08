package argv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Global flags always follow the whole prefix, which is the only order nom
// accepts, and nix accepts it anywhere after the subcommand.
func TestArgv(t *testing.T) {
	t.Parallel()

	const features = "--extra-experimental-features"

	tests := []struct {
		name    string
		prefix  []string
		globals []string
		groups  [][]string
		want    []string
	}{
		{
			name:    "nix",
			prefix:  []string{"nix", "build"},
			globals: []string{features, "nix-command flakes"},
			groups:  [][]string{{"--no-link"}, {"flake#attr"}},
			want:    []string{"nix", "build", features, "nix-command flakes", "--no-link", "flake#attr"},
		},
		{
			name:    "drop-in",
			prefix:  []string{"nom", "build"},
			globals: []string{features},
			groups:  [][]string{{"--no-link"}},
			want:    []string{"nom", "build", features, "--no-link"},
		},
		{
			name:   "no globals",
			prefix: []string{"nom", "build"},
			groups: [][]string{{"flake#attr"}},
			want:   []string{"nom", "build", "flake#attr"},
		},
		{
			name:   "no globals, nix",
			prefix: []string{"nix", "eval"},
			groups: [][]string{{"."}},
			want:   []string{"nix", "eval", "."},
		},
		{
			name: "empty",
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, Argv(tt.prefix, tt.globals, tt.groups...))
		})
	}
}
