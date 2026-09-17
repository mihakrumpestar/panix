package phaseops

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/stretchr/testify/assert"
)

func TestAsUser_EmptyUserReturnsCommandUnchanged(t *testing.T) {
	t.Parallel()

	cmd := []string{"nix", "profile", "add", "/nix/store/abc"}
	result := asUser("", cmd)

	assert.Equal(t, cmd, result)
}

func TestAsUser_SingleSafeArg(t *testing.T) {
	t.Parallel()

	// Even a safe arg is quoted: quoting decisions live in pkg/shellquote, not
	// at call sites.
	cmd := []string{"reboot"}
	result := asUser("root", cmd)

	assert.Equal(t, []string{"su", "-l", "root", "-c", `XDG_RUNTIME_DIR=/run/user/$(id -u) 'reboot'`}, result)
}

func TestAsUser_MultiWordCommand(t *testing.T) {
	t.Parallel()

	// Each arg becomes one quoted word in the single -c argv element, so su -c
	// parses the identical string on local and remote transports.
	cmd := []string{"echo", "hello"}
	result := asUser("alice", cmd)

	assert.Equal(t, []string{"su", "-l", "alice", "-c", `XDG_RUNTIME_DIR=/run/user/$(id -u) 'echo' 'hello'`}, result)
}

// TestAsUser_PreservesSpaceContainingArgs pins that an argument with spaces
// ("nix-command flakes") is one quoted word, so the login shell does not split
// it into separate arguments.
func TestAsUser_PreservesSpaceContainingArgs(t *testing.T) {
	t.Parallel()

	cmd := []string{
		"nix",
		"--extra-experimental-features", "nix-command flakes",
		"profile", "add",
		"/nix/store/abc",
	}
	result := asUser("root", cmd)

	assert.Equal(t, []string{
		"su", "-l", "root", "-c",
		`XDG_RUNTIME_DIR=/run/user/$(id -u) 'nix' '--extra-experimental-features' 'nix-command flakes' 'profile' 'add' '/nix/store/abc'`,
	}, result)
}

// TestAsUser_TildeInPathStaysUnquoted pins that ~ paths stay unquoted so the
// login shell expands them to the target user's home, as homeConfigurations and
// nixOnDroidConfigurations presets require.
func TestAsUser_TildeInPathStaysUnquoted(t *testing.T) {
	t.Parallel()

	cmd := []string{"nix-env", "--profile", "~/.local/state/nix/profiles/home-manager", "--list-generations"}
	result := asUser("alice", cmd)

	assert.Equal(t, []string{
		"su", "-l", "alice", "-c",
		`XDG_RUNTIME_DIR=/run/user/$(id -u) 'nix-env' '--profile' ~/.local/state/nix/profiles/home-manager '--list-generations'`,
	}, result)
}

// TestAsUser_TildeWithUnsafeRestStillQuoted pins the boundary: only inert ~/
// paths stay unquoted; a tilde arg with shell-active characters is quoted like
// anything else and thus stays literal, the safe failure mode.
func TestAsUser_TildeWithUnsafeRestStillQuoted(t *testing.T) {
	t.Parallel()

	result := asUser("root", []string{"echo", `~'x`, `~/a b`})

	assert.Equal(t, `XDG_RUNTIME_DIR=/run/user/$(id -u) 'echo' '~'\''x' '~/a b'`, result[4])
}

func TestAsUser_EscapesSingleQuotesInArgs(t *testing.T) {
	t.Parallel()

	cmd := []string{"echo", "it's working"}
	result := asUser("root", cmd)

	assert.Equal(t, `XDG_RUNTIME_DIR=/run/user/$(id -u) 'echo' 'it'\''s working'`, result[4])
}

func TestAsUser_EscapesDoubleQuotesInArgs(t *testing.T) {
	t.Parallel()

	cmd := []string{"echo", `say "hello"`}
	result := asUser("root", cmd)

	assert.Equal(t, `XDG_RUNTIME_DIR=/run/user/$(id -u) 'echo' 'say "hello"'`, result[4])
}

func TestAsUser_EscapesDollarInArgs(t *testing.T) {
	t.Parallel()

	cmd := []string{"echo", "$HOME"}
	result := asUser("root", cmd)

	assert.Equal(t, `XDG_RUNTIME_DIR=/run/user/$(id -u) 'echo' '$HOME'`, result[4])
}

// TestAsUser_QuotingMatrix covers representative argv shapes: every arg is one
// quoted word, except inert ~/ paths which stay bare for tilde expansion.
func TestAsUser_QuotingMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		expected string // expected -c string
	}{
		{
			name:     "all safe args",
			args:     []string{"nix", "profile", "add", "/nix/store/abc"},
			expected: `XDG_RUNTIME_DIR=/run/user/$(id -u) 'nix' 'profile' 'add' '/nix/store/abc'`,
		},
		{
			name:     "flags with dashes",
			args:     []string{"nix-env", "--profile", "/nix/var/nix/profiles/system", "--list-generations"},
			expected: `XDG_RUNTIME_DIR=/run/user/$(id -u) 'nix-env' '--profile' '/nix/var/nix/profiles/system' '--list-generations'`,
		},
		{
			name:     "arg with space",
			args:     []string{"nix", "--extra-experimental-features", "nix-command flakes", "build"},
			expected: `XDG_RUNTIME_DIR=/run/user/$(id -u) 'nix' '--extra-experimental-features' 'nix-command flakes' 'build'`,
		},
		{
			name:     "tilde path stays unquoted for expansion",
			args:     []string{"cat", "~/.bashrc"},
			expected: `XDG_RUNTIME_DIR=/run/user/$(id -u) 'cat' ~/.bashrc`,
		},
		{
			name:     "env assignment argv element",
			args:     []string{"env", "NIX_PAGER=cat", "nix-env"},
			expected: `XDG_RUNTIME_DIR=/run/user/$(id -u) 'env' 'NIX_PAGER=cat' 'nix-env'`,
		},
		{
			name:     "empty arg",
			args:     []string{"echo", ""},
			expected: `XDG_RUNTIME_DIR=/run/user/$(id -u) 'echo' ''`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := asUser("root", tt.args)
			assert.Equal(t, "su", result[0])
			assert.Equal(t, "-l", result[1])
			assert.Equal(t, "root", result[2])
			assert.Equal(t, "-c", result[3])
			assert.Equal(t, tt.expected, result[4],
				"-c string does not match expected quoting")
		})
	}
}

// TestWrapAsTargetUser_NoTargetUser pins the no-target-user arms: system-level
// types get the sudo program prefixed unless the SSH user is root, user-level
// types run as the SSH user.
func TestWrapAsTargetUser_NoTargetUser(t *testing.T) {
	t.Parallel()

	cmd := []string{"readlink", "/nix/var/nix/profiles/system-3-link"}

	tests := []struct {
		name        string
		isRoot      bool
		sudoProgram attributes.SudoProgram
		systemLevel bool
		want        []string
	}{
		{
			name:        "system-level, root SSH user: no sudo",
			isRoot:      true,
			systemLevel: true,
			want:        cmd,
		},
		{
			name:        "system-level, non-root SSH user: sudo prefixed",
			systemLevel: true,
			want:        []string{"sudo", "readlink", "/nix/var/nix/profiles/system-3-link"},
		},
		{
			name:        "system-level, non-root SSH user, custom sudo program",
			systemLevel: true,
			sudoProgram: attributes.SudoProgram("doas"),
			want:        []string{"doas", "readlink", "/nix/var/nix/profiles/system-3-link"},
		},
		{
			name: "user-level: runs as the SSH user",
			want: cmd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newWrapTestMachine(tt.isRoot, tt.sudoProgram)
			preset := installable.Preset{IsSystemLevel: new(tt.systemLevel)}

			assert.Equal(t, tt.want, WrapAsTargetUser(mach, preset, "", cmd))
		})
	}
}

// TestWrapAsTargetUser_WithTargetUser pins the su -l wrapping: user-level wraps
// plainly; system-level elevates inside via MaybeSudoFor, custom program
// honored. One non-root case pins that elevation is independent of SSH rootness.
func TestWrapAsTargetUser_WithTargetUser(t *testing.T) {
	t.Parallel()

	for _, tt := range wrapWithTargetUserCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newWrapTestMachine(tt.isRoot, tt.sudoProgram)
			preset := installable.Preset{IsSystemLevel: new(tt.systemLevel)}

			assert.Equal(t, tt.want, WrapAsTargetUser(mach, preset, tt.targetUser, wrapTestCmd))
		})
	}
}

var wrapTestCmd = []string{"readlink", "/nix/var/nix/profiles/system-3-link"}

// suWrap builds the expected asUser argv: su -l with the XDG_RUNTIME_DIR
// prefix, every inner word quoted.
func suWrap(user string, inner ...string) []string {
	quoted := make([]string, len(inner))
	for i, arg := range inner {
		quoted[i] = "'" + arg + "'"
	}

	return []string{"su", "-l", user, "-c", `XDG_RUNTIME_DIR=/run/user/$(id -u) ` + strings.Join(quoted, " ")}
}

var wrapWithTargetUserCases = []struct {
	name        string
	isRoot      bool
	systemLevel bool
	targetUser  string
	sudoProgram attributes.SudoProgram
	want        []string
}{
	{
		name:       "user set, user-level: plain su -l wrap",
		targetUser: "alice",
		want:       suWrap("alice", "readlink", "/nix/var/nix/profiles/system-3-link"),
	},
	{
		name:        "user set, system-level, root SSH user (the realistic su path): sudo inside",
		isRoot:      true,
		systemLevel: true,
		targetUser:  "bob",
		want:        suWrap("bob", "sudo", "readlink", "/nix/var/nix/profiles/system-3-link"),
	},
	{
		name:        "user set, system-level, non-root SSH user: elevation independent of SSH rootness",
		systemLevel: true,
		targetUser:  "bob",
		want:        suWrap("bob", "sudo", "readlink", "/nix/var/nix/profiles/system-3-link"),
	},
	{
		name:        "root target user, system-level, root SSH: normalized to bare command",
		systemLevel: true,
		targetUser:  "root",
		isRoot:      true,
		want:        wrapTestCmd,
	},
	{
		name:        "root target user, system-level, non-root SSH user: normalized to MaybeSudo",
		systemLevel: true,
		targetUser:  "root",
		want:        []string{"sudo", "readlink", "/nix/var/nix/profiles/system-3-link"},
	},
	{
		name:        "user set, system-level, custom sudo program: sudo program inside su -l",
		systemLevel: true,
		targetUser:  "carol",
		sudoProgram: attributes.SudoProgram("doas"),
		want:        suWrap("carol", "doas", "readlink", "/nix/var/nix/profiles/system-3-link"),
	},
}

// TestNormalizedTargetUser pins the root rule: system-level "root" becomes the
// default no-user sudo path, user-level "root" stays a legitimate login-shell
// target.
func TestNormalizedTargetUser(t *testing.T) {
	t.Parallel()

	system := installable.Preset{IsSystemLevel: new(true)}
	user := installable.Preset{IsSystemLevel: new(false)}

	assert.Empty(t, NormalizedTargetUser(system, "root"))
	assert.Equal(t, "bob", NormalizedTargetUser(system, "bob"))
	assert.Empty(t, NormalizedTargetUser(system, ""))

	assert.Equal(t, "root", NormalizedTargetUser(user, "root"))
	assert.Equal(t, "alice", NormalizedTargetUser(user, "alice"))
}

// newWrapTestMachine builds a machine whose MaybeSudo/IsRoot behavior is fully
// caller-controlled (MaybeSudo reads MetaInspect.IsRoot).
func newWrapTestMachine(isRoot bool, sudoProgram attributes.SudoProgram) *machine.Machine {
	mach := &machine.Machine{
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	mach.SudoProgram = sudoProgram
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: isRoot})

	return mach
}

// TestProfileSubcmdForFlavor pins the flavor split: Lix needs "install" because
// it never adopted the Nix 2.30 rename to "add".
func TestProfileSubcmdForFlavor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		flavor nixver.Flavor
		want   string
	}{
		{
			name:   "Nix uses add",
			flavor: nixver.FlavorNix,
			want:   "add",
		},
		{
			name:   "Lix uses install",
			flavor: nixver.FlavorLix,
			want:   "install",
		},
		{
			name:   "empty flavor defaults to add",
			flavor: "",
			want:   "add",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, profileSubcmdForFlavor(tt.flavor))
		})
	}
}
