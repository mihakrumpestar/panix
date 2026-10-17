package yamlx

import (
	"time"

	"github.com/pkg/errors"
)

// Duration is a serializable time.Duration. YAML accepts either a duration
// string ("120ms") or an integer of nanoseconds and always encodes as a
// duration string.
type Duration time.Duration

// Duration returns the value as a time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// UnmarshalYAML decodes a duration string ("120ms") or an integer of
// nanoseconds.
func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var raw any

	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parsed, err := parseDuration(raw)
	if err != nil {
		return err
	}

	*d = Duration(parsed)

	return nil
}

// MarshalYAML encodes the duration as a string ("120ms").
func (d Duration) MarshalYAML() ([]byte, error) {
	return []byte(d.Duration().String()), nil
}

// parseDuration accepts the decoded YAML shapes of a duration: a duration
// string ("120ms"), an integer of nanoseconds or null (zero).
func parseDuration(raw any) (time.Duration, error) {
	switch value := raw.(type) {
	case string:
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return 0, errors.Wrapf(err, "invalid duration %q", value)
		}

		return parsed, nil
	case uint64:
		return time.Duration(value), nil //nolint:gosec // G115: user-provided nanoseconds; overflow degrades to a wrong interval, not memory unsafety
	case int64:
		return time.Duration(value), nil
	case nil:
		return 0, nil
	default:
		return 0, errors.Errorf("invalid duration %v: expected a duration string like 120ms or an integer of nanoseconds", raw)
	}
}
