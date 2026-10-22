package filemode

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  uint32
	}{
		{"default", "0700", 0o700},
		{"owner only", "0600", 0o600},
		{"group read", "0640", 0o640},
		{"owner rwx", "0755", 0o755},
		{"other execute", "0001", 0o0001},
		{"group execute", "0010", 0o0010},
		{"owner execute", "0100", 0o0100},
		{"sticky", "1777", 0o1777},
		{"setuid", "4755", 0o4755},
		{"maximum", "7777", 0o7777},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			mode, err := Parse(testCase.value)
			require.NoError(t, err)
			assert.Equal(t, testCase.want, mode)
		})
	}
}

func TestParseRejectsNonCanonical(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"600", "0o600", "0O600", "0000",
		"999", "0o999", "800", "409", "448",
		"-600", "abc", "10000", "0o10000", "",
	} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(value)
			require.Error(t, err)
		})
	}
}

func TestFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode uint32
		want string
	}{
		{0o700, "0700"},
		{0o600, "0600"},
		{0o640, "0640"},
		{0o755, "0755"},
		{0o0001, "0001"},
		{0o1777, "1777"},
		{0o4755, "4755"},
		{0o7777, "7777"},
	}

	for _, testCase := range tests {
		t.Run(testCase.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, Format(testCase.mode))
		})
	}
}

func TestPatternMatchesParse(t *testing.T) {
	t.Parallel()

	pattern := regexp.MustCompile(Pattern)

	for _, value := range []string{
		"0700", "0600", "0640", "0755", "0001", "0010", "0100", "1777", "4755", "7777",
	} {
		assert.True(t, pattern.MatchString(value), "pattern must accept %q", value)
	}

	for _, value := range []string{
		"600", "0o600", "0O600", "0000",
		"999", "0o999", "800", "409", "448",
		"-600", "abc", "10000", "0o10000", "",
	} {
		assert.False(t, pattern.MatchString(value), "pattern must reject %q", value)
	}
}
