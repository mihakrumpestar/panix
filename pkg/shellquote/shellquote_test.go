package shellquote

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain word", "reboot", `'reboot'`},
		{"path", "/nix/store/abc", `'/nix/store/abc'`},
		{"spaces", "nix-command flakes", `'nix-command flakes'`},
		{"dollar", "$HOME", `'$HOME'`},
		{"double quotes", `say "hello"`, `'say "hello"'`},
		{"backtick", "a`b", "'a`b'"},
		{"backslash", `a\b`, `'a\b'`},
		{"equals and flag", "--flag=value", `'--flag=value'`},
		{"tilde stays literal in transit", "~/.bashrc", `'~/.bashrc'`},
		{"empty string", "", `''`},
		{"single quote escaped", "it's", `'it'\''s'`},
		{"shell metacharacter", ">", `'>'`},
		{"ampersand chain", "a && b", `'a && b'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, Quote(tt.input))
		})
	}
}

func TestQuoteWord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"tilde path stays bare for home expansion", "~/.local/state/nix/profiles/home-manager", "~/.local/state/nix/profiles/home-manager"},
		{"bare tilde", "~", "~"},
		{"named-user tilde", "~root/.bashrc", "~root/.bashrc"},
		{"tilde with space falls back to quoted", "~/a b", `'~/a b'`},
		{"tilde with quote falls back to quoted", "~'x", `'~'\''x'`},
		{"tilde with dollar falls back to quoted", "~/$HOME", `'~/$HOME'`},
		{"no tilde: full quote", "/nix/store/abc", `'/nix/store/abc'`},
		{"plain word", "nix", `'nix'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, QuoteWord(tt.input))
		})
	}
}
