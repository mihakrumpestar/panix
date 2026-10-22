// Package redact replaces recognized secret token values with *** in command
// lines for display.
//
// Only the matched token substring is replaced; assignment names, keys and
// surrounding context are preserved. This is a display-only filter for
// panix-generated command lines, not a general secret scanner: values that do
// not match a known token format or credential-key pattern are left visible. It
// never modifies the argv used for execution.
package redact

import "regexp"

const redaction = "***"

type secretPattern struct {
	re          *regexp.Regexp
	replacement string
}

// secretPatterns are applied in order. Token formats come first; the
// credential-key patterns preserve the key and separator and redact only the
// value.
var secretPatterns = []secretPattern{
	// Private key blocks.
	{
		regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
		redaction,
	},
	// GitHub tokens: classic and fine-grained.
	{regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{10,}`), redaction},
	{regexp.MustCompile(`github_pat_[A-Za-z0-9_]{10,}`), redaction},
	// GitLab personal access tokens.
	{regexp.MustCompile(`glpat-[A-Za-z0-9_-]{10,}`), redaction},
	// AWS access key IDs.
	{regexp.MustCompile(`(?:AKIA|ASIA)[0-9A-Z]{16}`), redaction},
	// Google API keys.
	{regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`), redaction},
	// Slack tokens.
	{regexp.MustCompile(`xox[baprs]-[0-9A-Za-z-]{10,}`), redaction},
	// Stripe keys.
	{regexp.MustCompile(`(?:sk|rk)_(?:live|test)_[0-9A-Za-z]{10,}`), redaction},
	// npm and PyPI tokens.
	{regexp.MustCompile(`npm_[A-Za-z0-9]{10,}`), redaction},
	{regexp.MustCompile(`pypi-[A-Za-z0-9_-]{20,}`), redaction},
	// JWTs.
	{regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`), redaction},
	// Nix access-tokens with a host: preserve host= and redact the token.
	{
		regexp.MustCompile(`(?i)(access-tokens\s*=\s*[^\s"']+=)[^\s"']+`),
		"${1}" + redaction,
	},
	// Nix access-tokens without a host prefix.
	{
		regexp.MustCompile(`(?i)(access-tokens\s*=\s*)([^\s"'=]+)(\s|["']|$)`),
		"${1}" + redaction + "${3}",
	},
	// Credential-shaped key/value pairs: preserve the key and separator.
	{
		regexp.MustCompile(`(?i)((?:token|secret(?:[_-]?key)?|password|passwd|api[_-]?key|access[_-]?key|private[_-]?key)\s*[=:]\s*)([^\s"']+)(\s|["']|$)`),
		"${1}" + redaction + "${3}",
	},
	// Bearer tokens in headers or arguments.
	{
		regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]{10,}`),
		"${1}" + redaction,
	},
}

// Argv returns a display-only copy of argv with recognized secret tokens
// redacted.
func Argv(argv []string) []string {
	if argv == nil {
		return nil
	}

	redacted := make([]string, len(argv))
	for index, arg := range argv {
		redacted[index] = ShellString(arg)
	}

	return redacted
}

// ShellString redacts recognized secret tokens inside a shell-joined string.
func ShellString(shellCommand string) string {
	for _, pattern := range secretPatterns {
		shellCommand = pattern.re.ReplaceAllString(shellCommand, pattern.replacement)
	}

	return shellCommand
}
