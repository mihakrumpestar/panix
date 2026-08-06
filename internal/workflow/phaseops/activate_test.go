package phaseops

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAsUser_EmptyUserReturnsCommandUnchanged(t *testing.T) {
	t.Parallel()

	cmd := []string{"nix", "profile", "add", "/nix/store/abc"}
	result := AsUser("", cmd)

	assert.Equal(t, cmd, result)
}

func TestAsUser_SingleSafeArgNoQuoting(t *testing.T) {
	t.Parallel()

	// A single safe arg with no spaces: no quoting at all needed
	cmd := []string{"reboot"}
	result := AsUser("root", cmd)

	assert.Equal(t, []string{"su", "-l", "root", "-c", "reboot"}, result)
}

func TestAsUser_MultiWordCommandGetsDoubleQuotes(t *testing.T) {
	t.Parallel()

	// Multiple safe args: inner string has spaces, needs outer double quotes.
	// Individual args are safe, so no single quotes.
	cmd := []string{"echo", "hello"}
	result := AsUser("alice", cmd)

	assert.Equal(t, []string{"su", "-l", "alice", "-c", `"echo hello"`}, result)
}

// TestAsUser_PreservesSpaceContainingArgs verifies that arguments containing
// spaces (e.g. "nix-command flakes") are properly shell-quoted so the shell
// doesn't split them into separate arguments.
//
// This is the regression test for the bug where `nix profile add` failed
// with "'flakes' is not a recognised command" because "nix-command flakes"
// was split into "nix-command" and "flakes" by the shell.
func TestAsUser_PreservesSpaceContainingArgs(t *testing.T) {
	t.Parallel()

	cmd := []string{
		"nix",
		"--extra-experimental-features", "nix-command flakes",
		"profile", "add",
		"/nix/store/abc",
	}
	result := AsUser("root", cmd)

	suCmd := result[len(result)-1]

	// Outer double quotes (single shell word for SSH transport)
	assert.True(t, strings.HasPrefix(suCmd, `"`), "cmd must start with double quote")
	assert.True(t, strings.HasSuffix(suCmd, `"`), "cmd must end with double quote")

	inner := suCmd[1 : len(suCmd)-1]

	// Only "nix-command flakes" should be single-quoted (has space).
	// Safe args like nix, --extra-experimental-features, profile, add
	// should NOT be single-quoted.
	assert.Contains(t, inner, `'nix-command flakes'`,
		"space-containing arg must be single-quoted")
	assert.NotContains(t, inner, `'nix'`,
		"safe arg 'nix' should not be single-quoted")
	assert.NotContains(t, inner, `'profile'`,
		"safe arg 'profile' should not be single-quoted")
	assert.NotContains(t, inner, `'add'`,
		"safe arg 'add' should not be single-quoted")
	assert.NotContains(t, inner, `'/nix/store/abc'`,
		"safe path arg should not be single-quoted")

	// Verify expected output
	assert.Equal(t, `nix --extra-experimental-features 'nix-command flakes' profile add /nix/store/abc`, inner)
}

// TestAsUser_TildeInPathStaysUnquoted verifies that ~ in paths is NOT
// single-quoted, so the login shell can expand it to the target user's home
// directory. This is critical for paths like ~/.local/state/nix/profiles/...
// used by homeConfigurations and nixOnDroidConfigurations presets.
//
// Regression test: the initial quoting fix single-quoted ~, which silently
// broke readGenerations() for home-manager with a target user.
func TestAsUser_TildeInPathStaysUnquoted(t *testing.T) {
	t.Parallel()

	cmd := []string{"nix-env", "--profile", "~/.local/state/nix/profiles/home-manager", "--list-generations"}
	result := AsUser("alice", cmd)

	suCmd := result[len(result)-1]
	inner := suCmd[1 : len(suCmd)-1] // strip outer double quotes

	// Tilde path must NOT be single-quoted — the login shell needs ~ unquoted
	// to expand it to /home/alice.
	assert.Contains(t, inner, "~/.local/state/nix/profiles/home-manager",
		"tilde path must be unquoted for login shell expansion")
	assert.NotContains(t, inner, "'~/.local",
		"tilde path must not be single-quoted")
	// Safe args must also not be quoted
	assert.Contains(t, inner, "nix-env --profile ")
	assert.Contains(t, inner, " --list-generations")
}

func TestAsUser_EscapesSingleQuotesInArgs(t *testing.T) {
	t.Parallel()

	cmd := []string{"echo", "it's working"}
	result := AsUser("root", cmd)

	suCmd := result[len(result)-1]
	inner := suCmd[1 : len(suCmd)-1] // strip outer double quotes

	// "it's working" has space and single quote → single-quoted with escaped quote
	// 'it'\''s working' → after backslash escaping for double quotes: 'it'\\''s working'
	assert.Equal(t, `echo 'it'\\''s working'`, inner)
}

func TestAsUser_EscapesDoubleQuotesInArgs(t *testing.T) {
	t.Parallel()

	cmd := []string{"echo", `say "hello"`}
	result := AsUser("root", cmd)

	suCmd := result[len(result)-1]

	// The double quotes in the arg must be escaped as \"
	assert.Contains(t, suCmd, `\"hello\"`)
}

func TestAsUser_EscapesDollarInArgs(t *testing.T) {
	t.Parallel()

	cmd := []string{"echo", "$HOME"}
	result := AsUser("root", cmd)

	suCmd := result[len(result)-1]

	// $ must be escaped as \$ inside double quotes
	assert.Contains(t, suCmd, `\$HOME`)
}

// TestAsUser_NoUnnecessaryQuoting verifies the minimal quoting principle:
// safe args are never quoted, only unsafe ones are.
func TestAsUser_NoUnnecessaryQuoting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		expected string // expected -c argument (including outer quotes if applicable)
	}{
		{
			name:     "all safe args",
			args:     []string{"nix", "profile", "add", "/nix/store/abc"},
			expected: `"nix profile add /nix/store/abc"`,
		},
		{
			name:     "flags with dashes",
			args:     []string{"nix-env", "--profile", "/nix/var/nix/profiles/system", "--list-generations"},
			expected: `"nix-env --profile /nix/var/nix/profiles/system --list-generations"`,
		},
		{
			name:     "single safe arg no outer quotes",
			args:     []string{"reboot"},
			expected: "reboot",
		},
		{
			name:     "arg with space gets single-quoted",
			args:     []string{"nix", "--extra-experimental-features", "nix-command flakes", "build"},
			expected: `"nix --extra-experimental-features 'nix-command flakes' build"`,
		},
		{
			name:     "tilde path stays unquoted for expansion",
			args:     []string{"cat", "~/.bashrc"},
			expected: `"cat ~/.bashrc"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := AsUser("root", tt.args)
			assert.Equal(t, "su", result[0])
			assert.Equal(t, "-l", result[1])
			assert.Equal(t, "root", result[2])
			assert.Equal(t, "-c", result[3])
			assert.Equal(t, tt.expected, result[4],
				"quoted command does not match expected minimal quoting")
		})
	}
}

// TestShellQuote verifies the shellQuote helper only quotes when needed.
func TestShellQuote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"", "''"},
		{"hello", "hello"},
		{"nix", "nix"},
		{"/nix/store/abc", "/nix/store/abc"},
		{"--extra-experimental-features", "--extra-experimental-features"},
		{"nix-command flakes", "'nix-command flakes'"},
		{"~/.bashrc", "~/.bashrc"}, // ~ is safe (login shell expands it)
		{"--flag=value", "--flag=value"}, // = is safe
		{"$HOME", "'$HOME'"},
		{`say "hi"`, `'say "hi"'`},
		{"it's", `'it'\''s'`},
		{"a:b@c.d", "a:b@c.d"}, // safe special chars
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			result := shellQuote(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
