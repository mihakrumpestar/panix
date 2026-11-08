package phaseops

import (
	"path/filepath"
	"slices"

	"github.com/pkg/errors"
)

// CommandInvokes reports whether command runs the named executable at any
// position, through a wrapper such as env(1) or an absolute path.
func CommandInvokes(command []string, name string) bool {
	return slices.ContainsFunc(command, func(element string) bool {
		return filepath.Base(element) == name
	})
}

// RequireExpandedOperand rejects a flag list that dropped an operand panix
// expanded into it, so a built-in command never silently acts on a default
// profile or an empty installable.
func RequireExpandedOperand(field, placeholder string, flags []string, value string) error {
	if slices.Contains(flags, value) {
		return nil
	}

	return errors.Errorf("%s dropped %s: the built-in command needs that operand", field, placeholder)
}
