package redact

import (
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
)

const canary = "PANIX_REDACT_CANARY_6f1c0d9e"

func TestShellString_RedactsNixTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"nix access token keeps the assignment and host",
			`export NIX_CONFIG="access-tokens = github.com=ghp_YOUR_TOKEN_HERE"`,
			`export NIX_CONFIG="access-tokens = github.com=***"`,
		},
		{
			"nix access token without host",
			"NIX_CONFIG=access-tokens=ghp_YOUR_TOKEN_HERE",
			"NIX_CONFIG=access-tokens=***",
		},
		{
			"arbitrary nix access token without host",
			"NIX_CONFIG=access-tokens=abc",
			"NIX_CONFIG=access-tokens=***",
		},
		{
			"arbitrary nix access token keeps the host",
			"NIX_CONFIG=access-tokens=github.com=abc",
			"NIX_CONFIG=access-tokens=github.com=***",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, ShellString(testCase.input))
		})
	}
}

func TestShellString_RedactsCredentialTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"github token keeps the key",
			"GITHUB_TOKEN=ghp_YOUR_TOKEN_HERE",
			"GITHUB_TOKEN=***",
		},
		{
			"credential key keeps the key",
			"DATABASE_PASSWORD=hunter2 cmd",
			"DATABASE_PASSWORD=*** cmd",
		},
		{
			"secret key keeps the key",
			"SECRET_KEY=abc",
			"SECRET_KEY=***",
		},
		{
			"bearer token keeps the scheme",
			"Authorization: Bearer abcdefghijklmnop",
			"Authorization: Bearer ***",
		},
		{
			"aws access key",
			"AKIAIOSFODNN7EXAMPLE",
			"***",
		},
		{
			"multiple tokens",
			"ghp_AAAABBBBCCCCDDDD ghp_EEEEFFFFGGGGHHHH",
			"*** ***",
		},
		{
			"private key block",
			"-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----",
			"***",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, ShellString(testCase.input))
		})
	}
}

func TestShellString_PreservesNonSecrets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"non-secret assignment stays visible",
			"NIX_CONFIG=max-jobs = 2",
			"NIX_CONFIG=max-jobs = 2",
		},
		{
			"non-secret hook variable stays visible",
			"PANIX_CLOSURE=/nix/store/abc",
			"PANIX_CLOSURE=/nix/store/abc",
		},
		{
			"non-secret install variable stays visible",
			"NIXOS_INSTALL_ENV=test",
			"NIXOS_INSTALL_ENV=test",
		},
		{
			"plain command stays visible",
			"nix build .#foo",
			"nix build .#foo",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, ShellString(testCase.input))
		})
	}
}

func TestArgv(t *testing.T) {
	t.Parallel()

	argv := []string{"export", `NIX_CONFIG="access-tokens = github.com=ghp_YOUR_TOKEN_HERE"`}
	want := []string{"export", `NIX_CONFIG="access-tokens = github.com=***"`}

	got := Argv(argv)

	assert.Equal(t, want, got)
	assert.Equal(t, []string{"export", `NIX_CONFIG="access-tokens = github.com=ghp_YOUR_TOKEN_HERE"`}, argv)
	assert.Nil(t, Argv(nil))

	got[1] = "changed"

	assert.Equal(t, []string{"export", `NIX_CONFIG="access-tokens = github.com=ghp_YOUR_TOKEN_HERE"`}, argv,
		"returned slice must not alias the input")
}

func FuzzRedactionNeverLeaksCanary(f *testing.F) {
	for _, suffix := range []string{"", "tail", "_X1"} {
		f.Add(suffix)
	}

	f.Fuzz(func(t *testing.T, suffix string) {
		suffix = sanitizeSuffix(suffix)
		token := "ghp_" + canary + suffix

		inputs := []string{
			token,
			`export NIX_CONFIG="access-tokens = github.com=` + token + `"`,
			"NIX_CONFIG=access-tokens=" + token,
			"Authorization: Bearer " + token,
		}

		for _, input := range inputs {
			if got := ShellString(input); strings.Contains(got, canary) {
				t.Fatalf("canary leaked from %q into %q", input, got)
			}
		}
	})
}

// sanitizeSuffix keeps fuzz input inside one token so the generated shape is
// always a token rather than a new word.
func sanitizeSuffix(suffix string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsSpace(character) || character == '\'' || character == '"' || character == '\\' {
			return -1
		}

		return character
	}, suffix)
}
