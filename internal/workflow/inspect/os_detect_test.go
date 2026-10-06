package inspect

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/stringbyte"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realInspectExecutioner builds a non-dry-run executioner on the local
// machine, for tests that execute actual commands through the local PTY
// transport.
func realInspectExecutioner(t *testing.T, mach *machine.Machine) (*executioner.Executioner, *phaselogs.PhaseLog) {
	t.Helper()

	phaseLog := phaselogs.NewPhaseLog()
	exc := executioner.NewExecutioner(executioner.ExecutionerConf{
		Ctx:          context.Background(),
		Timeout:      10 * time.Second,
		Xpath:        xpath.New("test"),
		Machine:      mach,
		Phase:        phase.Inspect,
		PhaseLog:     phaseLog,
		OnUpdateHook: func() {},
	})

	return exc, phaseLog
}

// stubExecutables writes fake commands into a fresh temp dir and prepends it
// to PATH, so the local PTY transport executes the stubs instead of the real
// tools.
func stubExecutables(t *testing.T, scripts map[string]string) {
	t.Helper()

	dir := t.TempDir()

	for name, script := range scripts {
		//nolint:gosec // the stub must be executable to be found on PATH
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700))
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestDetectOS_StoresOSFamily pins that detectOS stores the trimmed uname -s
// output in MetaInspect.OS for both OS families, via a real local execution.
//
//nolint:paralleltest // t.Setenv forbids parallel tests
func TestDetectOS_StoresOSFamily(t *testing.T) {
	tests := []struct {
		name       string
		unameOut   string
		wantOS     string
		wantCmdSeq string
	}{
		{
			name:       "darwin host",
			unameOut:   "Darwin\n",
			wantOS:     "Darwin",
			wantCmdSeq: "uname -s",
		},
		{
			name:       "linux host",
			unameOut:   "Linux\n",
			wantOS:     "Linux",
			wantCmdSeq: "uname -s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubExecutables(t, map[string]string{"uname": "#!/bin/sh\necho '" + tt.unameOut + "'\n"})

			mach := newSkipTestMachine(t, false, "")
			exc, phaseLog := realInspectExecutioner(t, mach)

			require.NoError(t, detectOS(exc, mach))

			mi := mach.MetaInspect.Load()
			require.NotNil(t, mi)
			assert.Equal(t, tt.wantOS, mi.OS.String())
			assert.Equal(t, []string{tt.wantCmdSeq}, testutil.CommandLines(t, phaseLog))
		})
	}
}

// TestDetectOS_DryRunStoresPlaceholder pins the dry-run path: the uname -s
// command is recorded but the OS family gets the DRY_RUN placeholder.
func TestDetectOS_DryRunStoresPlaceholder(t *testing.T) {
	t.Parallel()

	mach := newSkipTestMachine(t, false, "")
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Inspect)

	require.NoError(t, detectOS(exc, mach))

	mi := mach.MetaInspect.Load()
	require.NotNil(t, mi)
	assert.Equal(t, "DRY_RUN", mi.OS.String())
	assert.Equal(t, []string{"uname -s"}, testutil.CommandLines(t, phaseLog))
}

// TestDetectOS_EmptyOutputFailsClosed pins the fail-closed path: an empty
// uname -s output surfaces ErrOSOutputEmpty and stores no OS family.
//
//nolint:paralleltest // t.Setenv forbids parallel tests
func TestDetectOS_EmptyOutputFailsClosed(t *testing.T) {
	stubExecutables(t, map[string]string{"uname": "#!/bin/sh\n"})

	mach := newSkipTestMachine(t, false, "")
	exc, _ := realInspectExecutioner(t, mach)

	err := detectOS(exc, mach)

	require.ErrorContains(t, err, ErrOSOutputEmpty.Error())

	mi := mach.MetaInspect.Load()
	require.NotNil(t, mi)
	assert.Empty(t, mi.OS.String())
}

// TestDetectOSVersion_Darwin pins the macOS pipeline end to end: detectOS
// stores Darwin from uname -s, then detectOSVersion reports the product
// version through sw_vers instead of parsing /etc/os-release.
//
//nolint:paralleltest // t.Setenv forbids parallel tests
func TestDetectOSVersion_Darwin(t *testing.T) {
	stubExecutables(t, map[string]string{
		"uname":   "#!/bin/sh\necho 'Darwin'\n",
		"sw_vers": "#!/bin/sh\necho '14.5'\n",
	})

	mach := newSkipTestMachine(t, false, "")
	exc, phaseLog := realInspectExecutioner(t, mach)

	require.NoError(t, detectOS(exc, mach))
	require.NoError(t, detectOSVersion(exc, mach))

	mi := mach.MetaInspect.Load()
	require.NotNil(t, mi)
	assert.Equal(t, "macOS 14.5", mi.OSVersion.String())
	assert.Equal(t, []string{"uname -s", "sw_vers -productVersion"}, testutil.CommandLines(t, phaseLog))
}

// TestDetectMacOSVersion_EmptyVersionFailsClosed pins the fail-closed path:
// an empty sw_vers -productVersion output surfaces ErrMacOSVersionEmpty and
// leaves the OS version untouched.
//
//nolint:paralleltest // t.Setenv forbids parallel tests
func TestDetectMacOSVersion_EmptyVersionFailsClosed(t *testing.T) {
	stubExecutables(t, map[string]string{
		"uname":   "#!/bin/sh\necho 'Darwin'\n",
		"sw_vers": "#!/bin/sh\n",
	})

	mach := newSkipTestMachine(t, false, "")
	exc, _ := realInspectExecutioner(t, mach)

	require.NoError(t, detectOS(exc, mach))
	err := detectOSVersion(exc, mach)

	require.ErrorContains(t, err, ErrMacOSVersionEmpty.Error())

	mi := mach.MetaInspect.Load()
	require.NotNil(t, mi)
	assert.Empty(t, mi.OSVersion.String())
}

// TestDetectOSVersion_BranchSelection pins the branch choice on the detected
// OS family: darwin probes sw_vers, linux (and unknown state) keeps the
// /etc/os-release path unchanged. Dry-run pins the command without executing.
func TestDetectOSVersion_BranchSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		os        string
		wantCmd   string
		wantOSVer string
	}{
		{
			name:      "darwin host probes sw_vers",
			os:        "Darwin",
			wantCmd:   "sw_vers -productVersion",
			wantOSVer: "DRY_RUN",
		},
		{
			name:      "linux host keeps the os-release path",
			os:        "Linux",
			wantCmd:   "cat /etc/os-release",
			wantOSVer: "DRY_RUN",
		},
		{
			name:      "unknown OS family keeps the os-release path",
			os:        "",
			wantCmd:   "cat /etc/os-release",
			wantOSVer: "DRY_RUN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newSkipTestMachine(t, false, "")
			mach.MetaInspect.Update(func(mi *machine.MetaInspect) { mi.OS = stringbyte.StringByte(tt.os) })
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Inspect)

			require.NoError(t, detectOSVersion(exc, mach))

			mi := mach.MetaInspect.Load()
			require.NotNil(t, mi)
			assert.Equal(t, tt.wantCmd, testutil.LastCommandLine(t, phaseLog))
			assert.Equal(t, tt.wantOSVer, mi.OSVersion.String())
		})
	}
}

// nonEmptyCommandLines returns the recorded command lines, skipping the
// commandless ExecFn entries (mirrors the bootstrap package's helper).
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

// TestRunBootstrapModeInspect_DarwinHostGuard pins the macOS guard: a NixOS
// bootstrap preset fails closed on a detected darwin host before running any
// command, while linux hosts proceed and the nix-install mode stays allowed.
func TestRunBootstrapModeInspect_DarwinHostGuard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mode         installable.BootstrapMode
		os           string
		wantErr      string
		wantCommands []string
	}{
		{
			name:         "nixos bootstrap rejects a darwin host",
			mode:         installable.BootstrapNixOS,
			os:           "Darwin",
			wantErr:      "NixOS bootstrap requires a Linux target, but macOS was detected",
			wantCommands: nil,
		},
		{
			name:         "nixos bootstrap proceeds on a linux host",
			mode:         installable.BootstrapNixOS,
			os:           "Linux",
			wantCommands: []string{"cat /etc/os-release"},
		},
		{
			name:         "nix-install mode is allowed on a darwin host",
			mode:         installable.BootstrapNixInstall,
			os:           "Darwin",
			wantCommands: []string{"nix --version"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newSkipTestMachine(t, false, "")
			mach.MetaInspect.Update(func(mi *machine.MetaInspect) { mi.OS = stringbyte.StringByte(tt.os) })

			leaf := &fleet.FleetLeaf{
				Installable: &installable.Installable{Preset: installable.Preset{Bootstrap: tt.mode}},
				Machine:     mach,
			}

			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Inspect)

			err := runBootstrapModeInspect(exc, leaf)

			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantCommands, nonEmptyCommandLines(t, phaseLog))
		})
	}
}
