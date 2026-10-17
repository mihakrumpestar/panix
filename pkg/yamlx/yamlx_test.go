package yamlx

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncodeIndentsSequences pins the panix YAML sequence style: items under a
// mapping key are indented two spaces (yaml.IndentSequence(true) plus
// yaml.Indent(2)), never flushed with the key. Every panix command that
// persists YAML goes through Encode, so this guard keeps schemas, docs blocks
// and "panix eval" output stable across dependency bumps.
func TestEncodeIndentsSequences(t *testing.T) {
	t.Parallel()

	type inner struct {
		Name  string   `yaml:"name"`
		Items []string `yaml:"items"`
	}

	type outer struct {
		Key  string  `yaml:"key"`
		List []inner `yaml:"list"`
	}

	tests := []struct {
		name     string
		value    any
		expected string
	}{
		{
			name:     "slice of scalars under a mapping key",
			value:    inner{Name: "alpha", Items: []string{"one", "two"}},
			expected: "name: alpha\nitems:\n  - one\n  - two\n",
		},
		{
			name:     "slice of maps under a mapping key",
			value:    outer{Key: "top", List: []inner{{Name: "a", Items: []string{"x", "z"}}}},
			expected: "key: top\nlist:\n  - name: a\n    items:\n      - x\n      - z\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			require.NoError(t, Encode(tt.value, &buf))
			assert.Equal(t, tt.expected, buf.String())
			assert.Contains(t, buf.String(), "\n  - ",
				"sequence items must be indented under their mapping key")
		})
	}
}
