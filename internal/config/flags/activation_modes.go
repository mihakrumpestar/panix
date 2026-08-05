package flags

import (
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
)

// ActivationMode holds per-installable-type activation mode overrides.
// Supports two input formats:
//   - Bare value: "test" applies to all installable types
//   - Key=value pairs: "nixosConfigurations=test;homeConfigurations=switch"
//
// When a bare value is set, it overrides all types.
// When key=value pairs are set, only specified types are overridden.
type ActivationMode struct {
	AllTypes string
	PerType  map[string]string
}

// Decode implements kong.MapperValue for CLI flag parsing.
func (a *ActivationMode) Decode(ctx *kong.DecodeContext) error {
	var value string

	err := ctx.Scan.PopValueInto("activation-mode", &value)
	if err != nil {
		return errors.Wrap(err, "failed to decode activation mode")
	}

	a.AllTypes = ""
	a.PerType = nil

	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	// Check if it contains "="; if not, it's a bare value for all types
	if !strings.Contains(value, "=") {
		a.AllTypes = value

		return nil
	}

	a.PerType = make(map[string]string)

	for pair := range strings.SplitSeq(value, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		key, val, found := strings.Cut(pair, "=")
		if !found {
			continue
		}

		a.PerType[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}

	return nil
}

// Get returns the activation mode for the given installable type.
// If AllTypes is set, it takes precedence over PerType.
// Returns empty string if no override is set.
func (a *ActivationMode) Get(installableType string) string {
	if a == nil {
		return ""
	}

	if a.AllTypes != "" {
		return a.AllTypes
	}

	return a.PerType[installableType]
}

// IsZero returns true if no modes are set.
func (a *ActivationMode) IsZero() bool {
	return a == nil || (a.AllTypes == "" && len(a.PerType) == 0)
}
