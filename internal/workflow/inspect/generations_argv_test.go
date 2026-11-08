package inspect

import (
	"context"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readGenerations is the one nix-env invocation on the inspect path, so its
// command prefix override must reach the recorded argv while the inline
// NIX_PAGER guard and panix's own arguments stay in place.
func TestReadGenerations_CommandOverride(t *testing.T) {
	t.Parallel()

	mach := newSkipTestMachine(t, false, "")
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Inspect)

	nixCfg := &nix.NixConfig{NixEnvCommand: []string{"/run/wrappers/bin/nix-env"}}
	require.NoError(t, nixCfg.Init(nil))

	require.NoError(t, readGenerations(
		exc,
		mach,
		installable.Preset{ProfilePath: "/nix/var/nix/profiles/system"},
		"/nix/var/nix/profiles/system",
		"",
		nixCfg,
	))

	assert.Equal(t,
		"env NIX_PAGER=cat /run/wrappers/bin/nix-env --profile /nix/var/nix/profiles/system --list-generations",
		testutil.LastCommandLine(t, phaseLog))
}

// The generation listing flags are configured, so the profile operand reaches a
// replacement tool the way that tool expects, while panix's NIX_PAGER guard and
// privilege wrap stay in place.
func TestReadGenerations_DefaultFlagOverride(t *testing.T) {
	t.Parallel()

	mach := newSkipTestMachine(t, false, "")
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Inspect)

	nixCfg := &nix.NixConfig{
		NixEnvListGenerationsDefaultFlags: []string{"--list-generations", "--profile", "$PANIX_PROFILE_PATH"},
	}
	require.NoError(t, nixCfg.Init(nil))

	require.NoError(t, readGenerations(
		exc,
		mach,
		installable.Preset{ProfilePath: "/nix/var/nix/profiles/system"},
		"/nix/var/nix/profiles/system",
		"",
		nixCfg,
	))

	assert.Equal(t,
		"env NIX_PAGER=cat nix-env --list-generations --profile /nix/var/nix/profiles/system",
		testutil.LastCommandLine(t, phaseLog))
}

// A flag list referencing a value the inspect step does not supply fails the
// listing instead of running a half-expanded argv.
func TestReadGenerations_UnavailableRuntimeVariableFails(t *testing.T) {
	t.Parallel()

	mach := newSkipTestMachine(t, false, "")
	exc, _ := testutil.NewDryRunExecutioner(t, mach, phase.Inspect)

	nixCfg := &nix.NixConfig{NixEnvListGenerationsDefaultFlags: []string{"--profile", "$PANIX_STORE_URL"}}
	require.NoError(t, nixCfg.Init(nil))

	err := readGenerations(
		exc,
		mach,
		installable.Preset{ProfilePath: "/nix/var/nix/profiles/system"},
		"/nix/var/nix/profiles/system",
		"",
		nixCfg,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nix_env_list_generations_default_flags")
	assert.Contains(t, err.Error(), "PANIX_STORE_URL")
}

// The built-in nix-env must keep both the profile operand and the listing
// operation: dropping the profile reads the default profile, and dropping the
// operation exits without a generation list, which silently disables
// auto-rollback. A replacement command owns its own argv.
func TestGenerationsArgv_RequiresBuiltinOperands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command []string
		flags   []string
		wantErr string
	}{
		{
			name:  "built-in keeps both operands",
			flags: []string{"--profile", "$PANIX_PROFILE_PATH", "--list-generations"},
		},
		{
			name:  "built-in short operation form",
			flags: []string{"--profile", "$PANIX_PROFILE_PATH", "-G"},
		},
		{
			name:    "built-in without the profile",
			flags:   []string{"--list-generations"},
			wantErr: "nix_env_list_generations_default_flags dropped $PANIX_PROFILE_PATH",
		},
		{
			name:    "built-in without the operation",
			flags:   []string{"--profile", "$PANIX_PROFILE_PATH"},
			wantErr: "dropped --list-generations",
		},
		{
			name:    "replacement command owns its list",
			command: []string{"my-env"},
			flags:   []string{"--show"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			nixCfg := &nix.NixConfig{
				NixEnvCommand:                     tt.command,
				NixEnvListGenerationsDefaultFlags: tt.flags,
			}
			require.NoError(t, nixCfg.Init(nil))

			_, err := generationsArgv(
				newSkipTestMachine(t, false, ""),
				installable.Preset{ProfilePath: "/nix/var/nix/profiles/system"},
				"/nix/var/nix/profiles/system",
				"",
				nixCfg,
			)

			if tt.wantErr == "" {
				require.NoError(t, err)

				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// A replacement command's failure is not a fresh profile: only the built-in
// nix-env may be absent or have no profile yet.
func TestReadGenerations_ReplacementCommandFailurePropagates(t *testing.T) {
	t.Parallel()

	mach := newSkipTestMachine(t, false, "")
	nixCfg := &nix.NixConfig{NixEnvCommand: []string{"sh", "-c", "exit 1"}}
	require.NoError(t, nixCfg.Init(nil))

	err := readGenerations(
		newGenerationsRealExecutioner(t, mach),
		mach,
		installable.Preset{ProfilePath: "/nix/var/nix/profiles/system"},
		"/nix/var/nix/profiles/system",
		"",
		nixCfg,
	)

	require.Error(t, err)
}

// newGenerationsRealExecutioner runs readGenerations through the real local PTY
// transport, so the OnSuccess postcondition is exercised rather than mocked.
func newGenerationsRealExecutioner(t *testing.T, mach *machine.Machine) *executioner.Executioner {
	t.Helper()

	return executioner.NewExecutioner(executioner.ExecutionerConf{
		Ctx:          context.Background(),
		Timeout:      10 * time.Second,
		Xpath:        xpath.New("test"),
		Machine:      mach,
		Phase:        phase.Inspect,
		PhaseLog:     phaselogs.NewPhaseLog(),
		OnUpdateHook: func() {},
	})
}

// Exit 0 with output that is not a generation list means the configured
// nix_env_command is not a nix-env drop-in. Storing nil silently would leave
// auto-rollback disabled for the rest of the run, so it must fail loudly.
func TestReadGenerations_UnparseableOutputFails(t *testing.T) {
	t.Parallel()

	mach := newSkipTestMachine(t, false, "")
	nixCfg := &nix.NixConfig{NixEnvCommand: []string{"sh", "-c", "echo 'not a generation list'"}}
	require.NoError(t, nixCfg.Init(nil))

	err := readGenerations(
		newGenerationsRealExecutioner(t, mach),
		mach,
		installable.Preset{ProfilePath: "/nix/var/nix/profiles/system"},
		"/nix/var/nix/profiles/system",
		"",
		nixCfg,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no parseable generations")
}

// An empty profile is not a failure: there are simply no generations yet.
func TestReadGenerations_EmptyOutputIsFine(t *testing.T) {
	t.Parallel()

	mach := newSkipTestMachine(t, false, "")
	nixCfg := &nix.NixConfig{NixEnvCommand: []string{"true"}}
	require.NoError(t, nixCfg.Init(nil))

	require.NoError(t, readGenerations(
		newGenerationsRealExecutioner(t, mach),
		mach,
		installable.Preset{ProfilePath: "/nix/var/nix/profiles/system"},
		"/nix/var/nix/profiles/system",
		"",
		nixCfg,
	))

	meta := mach.MetaInspect.Load()
	assert.Nil(t, meta.Generations, "no generations recorded, and no error either")
}

// A working nix-env-shaped drop-in still populates generations.
func TestReadGenerations_ParsesOutput(t *testing.T) {
	t.Parallel()

	mach := newSkipTestMachine(t, false, "")
	nixCfg := &nix.NixConfig{
		NixEnvCommand: []string{"sh", "-c", `printf '1   2026-08-01 15:00:44\n2   2026-08-01 15:10:22   (current)\n'`},
	}
	require.NoError(t, nixCfg.Init(nil))

	require.NoError(t, readGenerations(
		newGenerationsRealExecutioner(t, mach),
		mach,
		installable.Preset{ProfilePath: "/nix/var/nix/profiles/system"},
		"/nix/var/nix/profiles/system",
		"",
		nixCfg,
	))

	gens := mach.MetaInspect.Load().Generations
	require.NotNil(t, gens)
	assert.Equal(t, uint(2), gens.Current)
	assert.Equal(t, []uint{1, 2}, gens.Available)
}
