// Package shellquote is the single source of truth for POSIX shell quoting
// everywhere panix must pass an exact string through a shell boundary: the
// SSH transport (the remote shell re-parses the command line), su -c
// command strings, and sh -c scripts.
package shellquote

import "strings"

// Quote wraps s in single quotes so a POSIX shell parses it as exactly one
// literal word. It always quotes, even safe-looking strings: quoting decisions
// belong here, not at call sites.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// QuoteWord quotes like Quote, except that a shell-inert tilde path stays
// unquoted so the receiving login shell expands ~ to the invoking user's home
// (quoting it would resolve a literal ~ against the working directory). Tilde
// paths with shell-active characters are quoted and fail loudly instead.
func QuoteWord(s string) string {
	if strings.HasPrefix(s, "~") && isInertWord(s[1:]) {
		return s
	}

	return Quote(s)
}

// QuoteIfNeeded single-quotes s only when it is not already one inert word.
// Use it where a string is re-tokenized by something that splits on whitespace
// (e.g. rsync's --rsync-path, which the remote rsync parses itself): the common
// simple form stays readable and unchanged, while a value with spaces or quotes
// cannot split into extra arguments.
func QuoteIfNeeded(s string) string {
	if s != "" && isInertWord(s) {
		return s
	}

	return Quote(s)
}

// isInertWord reports whether s contains only characters that are inert in a
// POSIX shell word: no whitespace, quoting, expansion or redirection.
func isInertWord(s string) bool {
	for i := range len(s) {
		if !isInertRune(s[i]) {
			return false
		}
	}

	return true
}

// isInertRune is the single inert-character set shared by every quoting rule
// in this package.
func isInertRune(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case strings.ContainsRune("_-./,:@+=", rune(c)):
		return true
	default:
		return false
	}
}
