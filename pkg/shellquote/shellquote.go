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
	if strings.HasPrefix(s, "~") && tildePathIsSafe(s[1:]) {
		return s
	}

	return Quote(s)
}

// tildePathIsSafe reports whether every character after a leading ~ is
// inert in a POSIX shell word, so the whole ~/. (or ~user/) form can stay
// unquoted.
func tildePathIsSafe(s string) bool {
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.ContainsRune("_-./,:@+=", rune(c)):
		default:
			return false
		}
	}

	return true
}
