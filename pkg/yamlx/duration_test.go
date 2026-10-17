package yamlx

import (
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDurationYAMLDecode(t *testing.T) {
	t.Parallel()

	type holder struct {
		Interval Duration `yaml:"interval"`
	}

	tests := []struct {
		name     string
		yaml     string
		expected time.Duration
		wantErr  bool
	}{
		{name: "duration string", yaml: "interval: 120ms\n", expected: 120 * time.Millisecond},
		{name: "quoted duration string", yaml: "interval: \"120ms\"\n", expected: 120 * time.Millisecond},
		{name: "nanoseconds integer", yaml: "interval: 120000000\n", expected: 120 * time.Millisecond},
		{name: "long duration string", yaml: "interval: 1h30m\n", expected: 90 * time.Minute},
		{name: "null is zero", yaml: "interval: ~\n", expected: 0},
		{name: "missing is zero", yaml: "{}\n", expected: 0},
		{name: "invalid string errors", yaml: "interval: banana\n", wantErr: true},
		{name: "unitless number errors", yaml: "interval: \"120\"\n", wantErr: true},
		{name: "boolean errors", yaml: "interval: true\n", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got holder

			err := Decode([]byte(tt.yaml), &got)

			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, Duration(tt.expected), got.Interval)
			assert.Equal(t, tt.expected, got.Interval.Duration())
		})
	}
}

func TestDurationYAMLRoundTrip(t *testing.T) {
	t.Parallel()

	type holder struct {
		Interval Duration `yaml:"interval"`
	}

	encoded, err := yaml.Marshal(holder{Interval: Duration(120 * time.Millisecond)})
	require.NoError(t, err)
	assert.Equal(t, "interval: 120ms\n", string(encoded))

	var decoded holder

	require.NoError(t, Decode(encoded, &decoded))
	assert.Equal(t, Duration(120*time.Millisecond), decoded.Interval)
}
