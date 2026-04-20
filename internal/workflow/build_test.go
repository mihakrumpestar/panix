package workflow

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLastNonEmptyLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{"empty input", []byte{}, []byte{}},
		{"nil input", nil, []byte{}},
		{"single line", []byte("single line"), []byte("single line")},
		{
			"multiple lines last is json",
			[]byte("building...\nwarning...\n" +
				`[{"outputs":{"out":"/nix/store/abc"}}]`),
			[]byte(`[{"outputs":{"out":"/nix/store/abc"}}]`),
		},
		{"trailing newlines", []byte("output\n\n\n"), []byte("output")},
		{"multiple trailing newlines", []byte("first\nsecond\n\n\n"), []byte("second")},
		{"only whitespace", []byte("  \n  \n  "), []byte{}},
		{"only newlines", []byte("\n\n\n"), []byte{}},
		{"content with leading whitespace", []byte("\n\n  content  \n"), []byte("content")},
		{
			"realistic nix build output",
			[]byte("warning: Git tree is dirty\nevaluating flake...\n" +
				`[{"outputs":{"out":"/nix/store/xyz"}}]` + "\n"),
			[]byte(`[{"outputs":{"out":"/nix/store/xyz"}}]`),
		},
		{"line with tabs", []byte("line1\n\t  line2 with spaces  \t\n"), []byte("line2 with spaces")},
		{"windows line endings", []byte("line1\r\nline2\r\n"), []byte("line2")},
		{"single trailing newline", []byte("single\n"), []byte("single")},
		{
			"carriage return only treated as single line",
			[]byte("line1\rline2"),
			[]byte("line1\rline2"),
		},
		{
			"multiple build outputs",
			[]byte(`[{"outputs":{"out":"/nix/store/a"}},` +
				`{"outputs":{"out":"/nix/store/b"}}]`),
			[]byte(`[{"outputs":{"out":"/nix/store/a"}},` +
				`{"outputs":{"out":"/nix/store/b"}}]`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, lastNonEmptyLine(tt.input))
		})
	}
}

func TestBuildOutputJSONUnmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantLen int
		wantOut string
		wantErr bool
	}{
		{"valid single output", `[{"outputs":{"out":"/nix/store/abc-system"}}]`, 1, "/nix/store/abc-system", false},
		{"valid with extra fields", `[{"outputs":{"out":"/nix/store/xyz"},"drvPath":"/nix/store/drv"}]`, 1, "/nix/store/xyz", false},
		{"empty array", `[]`, 0, "", false},
		{"invalid json", `not json`, 0, "", true},
		{"missing outputs", `[{"drvPath":"/nix/store/drv"}]`, 1, "", false},
		{"multiple outputs", `[{"outputs":{"out":"/nix/store/a"}},{"outputs":{"out":"/nix/store/b"}}]`, 2, "/nix/store/a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)

			var output BuildOutputJSON

			err := json.Unmarshal([]byte(tt.input), &output)

			if tt.wantErr {
				assertion.Error(err)

				return
			}

			require.NoError(t, err)
			assertion.Len(output, tt.wantLen)

			if tt.wantLen > 0 {
				assertion.Equal(tt.wantOut, output[0].Outputs.Out)
			}
		})
	}
}

func TestBuildOutputJSONRoundtrip(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)
	must := require.New(t)

	original := BuildOutputJSON{{Outputs: struct {
		Out string `json:"out"`
	}{Out: "/nix/store/test"}}}

	data, err := json.Marshal(original)
	must.NoError(err)

	var parsed BuildOutputJSON

	err = json.Unmarshal(data, &parsed)
	must.NoError(err)
	require.NotEmpty(t, parsed)
	assertion.Equal(original[0].Outputs.Out, parsed[0].Outputs.Out)
}

func TestBuildOutputJSONMultipleOutputsRoundtrip(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)
	must := require.New(t)

	original := BuildOutputJSON{
		{Outputs: struct {
			Out string `json:"out"`
		}{Out: "/nix/store/a"}},
		{Outputs: struct {
			Out string `json:"out"`
		}{Out: "/nix/store/b"}},
	}

	data, err := json.Marshal(original)
	must.NoError(err)

	var parsed BuildOutputJSON

	err = json.Unmarshal(data, &parsed)
	must.NoError(err)
	assertion.Len(parsed, 2)
	assertion.Equal("/nix/store/a", parsed[0].Outputs.Out)
	assertion.Equal("/nix/store/b", parsed[1].Outputs.Out)
}
