package validate

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const experimentalFeatures = "--extra-experimental-features"

// Both flake-validation probes are the only nix invocations outside the
// workflow, so their prefixes need the same coverage as the rest.

func TestFlakeMetadataArgv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		nix  nix.NixConfig
		want []string
	}{
		{
			name: "defaults",
			want: []string{"nix", "flake", "metadata", experimentalFeatures, "nix-command flakes", "--json", "github:owner/repo"},
		},
		{
			name: "override",
			nix:  nix.NixConfig{FlakeMetadataCommand: []string{"nom", "flake"}},
			want: []string{"nom", "flake", experimentalFeatures, "nix-command flakes", "--json", "github:owner/repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := tt.nix
			require.NoError(t, cfg.Init(nil))

			assert.Equal(t, tt.want, flakeMetadataArgv(&cfg, "github:owner/repo"))
		})
	}
}

func TestInstallableEvalArgv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		nix  nix.NixConfig
		want []string
	}{
		{
			name: "defaults",
			want: []string{"nix", "eval", experimentalFeatures, "nix-command flakes", ".#web", "--apply", "x: true", "--json"},
		},
		{
			name: "override",
			nix:  nix.NixConfig{EvalCommand: []string{"/run/current-system/sw/bin/nix", "eval"}},
			want: []string{"/run/current-system/sw/bin/nix", "eval", experimentalFeatures, "nix-command flakes", ".#web", "--apply", "x: true", "--json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := tt.nix
			require.NoError(t, cfg.Init(nil))

			assert.Equal(t, tt.want, installableEvalArgv(&cfg, ".#web"))
		})
	}
}

// A broken probe must not be reported as a missing flake: a wrong or absent
// command prefix would otherwise send users debugging their flake URL.
func TestProbeError_DistinguishesCause(t *testing.T) {
	t.Parallel()

	argv := []string{"definitely-not-a-real-command-xyz"}
	exit127 := exec.CommandContext(context.Background(), "sh", "-c", "exit 127").Run()

	tests := []struct {
		name     string
		err      error
		wantMsg  string
		negative bool
	}{
		{
			name:     "missing executable points at the command",
			err:      &exec.Error{Name: argv[0], Err: exec.ErrNotFound},
			wantMsg:  "cannot run configured command",
			negative: false,
		},
		{
			name:     "exit 127 points at PATH",
			err:      exit127,
			wantMsg:  "is missing on PATH",
			negative: false,
		},
		{
			name:     "other failures are a plain negative answer",
			err:      context.DeadlineExceeded,
			wantMsg:  "probe returned a negative answer",
			negative: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := probeError(tt.err, argv)
			assert.Contains(t, got.Error(), tt.wantMsg)
			assert.Equal(t, tt.negative, errors.Is(got, errProbeNegative))
		})
	}
}

// The real missing-executable path: a configured prefix that does not exist must
// name the command rather than claim the flake is gone.
func TestCheckFlakeURLExists_MissingCommand(t *testing.T) {
	t.Parallel()

	nixCfg := &nix.NixConfig{FlakeMetadataCommand: []string{"panix-no-such-binary-xyz"}}
	require.NoError(t, nixCfg.Init(nil))

	err := checkFlakeURLExists("github:owner/repo", 5*time.Second, nixCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot run configured command")
	assert.NotErrorIs(t, err, errProbeNegative, "a broken prefix is not a missing flake")
}
