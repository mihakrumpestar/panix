package bootstrap

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Staging paths must be unique per run and live in the /tmp staging area,
// never at a fixed predictable path an attacker could pre-create.
func TestNewStagingDirPath_UniquePerRun(t *testing.T) {
	t.Parallel()

	for _, prefix := range []string{installerStagingPrefix, kexecStagingPrefix} {
		t.Run(prefix, func(t *testing.T) {
			t.Parallel()

			seen := make(map[string]struct{}, 100)

			for range 100 {
				stagingDir, err := newStagingDirPath(prefix)
				require.NoError(t, err)

				assert.True(t, strings.HasPrefix(stagingDir, "/tmp/"+prefix+"-"),
					"staging dir %q must be under the staging base with prefix %q", stagingDir, prefix)
				assert.NotEqual(t, "/tmp/panix-nix-installer", stagingDir)
				assert.NotEqual(t, "/tmp/kexec", stagingDir)
				assert.NotContains(t, stagingDir, "/tmp/panix-nix-installer")
				assert.NotContains(t, stagingDir, "/tmp/kexec")

				_, duplicate := seen[stagingDir]
				assert.False(t, duplicate, "staging dir %q must be unique per run", stagingDir)

				seen[stagingDir] = struct{}{}
			}
		})
	}
}

// The private staging dir is created mode 0700 and owned by the active SSH
// user, so the un-elevated download/extract steps can use it.
func TestCreatePrivateStagingDir_Argv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		isRoot bool
		want   string
	}{
		{
			name:   "non-root SSH user: elevated install with SSH-user ownership",
			isRoot: false,
			want:   "sudo install -d -m 700 -o kexec-user ",
		},
		{
			name:   "root SSH user: bare install with SSH-user ownership",
			isRoot: true,
			want:   "install -d -m 700 -o kexec-user ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newKexecMachine(t, tt.isRoot)
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

			stagingDir, err := newStagingDirPath(installerStagingPrefix)
			require.NoError(t, err)

			require.NoError(t, createPrivateStagingDir(exc, mach, stagingDir))

			assert.Equal(t, tt.want+stagingDir, testutil.LastCommandLine(t, phaseLog))
		})
	}
}

// Cleanup removes exactly the per-run directory, never a fixed path.
func TestRemoveStagingDir_Argv(t *testing.T) {
	t.Parallel()

	mach := newKexecMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

	const stagingDir = "/tmp/panix-installer-0123456789abcdef0123456789abcdef"

	require.NoError(t, removeStagingDir(exc, mach, stagingDir))

	assert.Equal(t, "rm -rf -- "+stagingDir, testutil.LastCommandLine(t, phaseLog))
}
