package bootstrap

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pins the bootstrap order: kexec, hardware config, disko. Disko's build
// evaluates the operator's configuration, which may import the generated
// hardware config.
func TestRunPhase_KexecThenHardwareConfigThenDisko(t *testing.T) {
	// Not parallel: t.Setenv forbids it. Sandbox XDG_CACHE_HOME so the
	// dry-run build stays out of the real user cache.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	leaf := newDiskoLeaf(t, true)
	leaf.Machine.HardwareConfigPath = filepath.Join(t.TempDir(), "hardware-configuration.nix")
	leaf.Machine.Bootstrap.Kexec.Image = "https://example.com/kexec.tar.gz"
	leaf.Machine.MetaInspect.Store(&machine.MetaInspect{IsRoot: true, RequiresKexec: true})

	exc, phaseLog := testutil.NewDryRunExecutioner(t, leaf.Machine, phase.Bootstrap)

	require.NoError(t, Handler{}.RunPhase(exc, leaf))

	lines := nonEmptyCommandLines(t, phaseLog)

	kexecIdx := indexOfCommandContaining(t, lines, "/tmp/kexec/kexec/run")
	configIdx := indexOfCommandContaining(t, lines, "nixos-generate-config --show-hardware-config --no-filesystems")
	diskoIdx := indexOfCommandContaining(t, lines, "disko_BUILD_OUTPUT_PATH_PLACEHOLDER")

	assert.Less(t, kexecIdx, configIdx, "hardware config must be generated after kexec boots the installer")
	assert.Less(t, configIdx, diskoIdx, "hardware config must exist before the disko build evaluates the configuration")
}

// nonEmptyCommandLines drops argv-less entries recorded by ExecFn wait loops.
func nonEmptyCommandLines(t *testing.T, phaseLog *phaselogs.PhaseLog) []string {
	t.Helper()

	var lines []string

	for _, line := range testutil.CommandLines(t, phaseLog) {
		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines
}

func indexOfCommandContaining(t *testing.T, lines []string, substr string) int {
	t.Helper()

	for i, line := range lines {
		if strings.Contains(line, substr) {
			return i
		}
	}

	require.FailNowf(t, "command not recorded", "no command containing %q in %v", substr, lines)

	return -1
}
