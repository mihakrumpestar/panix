package buffer

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLineBuf_MarshalJSON(t *testing.T) {
	t.Parallel()

	buf := NewLineBuf()
	buf.Set([]byte("hello world"))

	data, err := buf.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `"hello world"`, string(data))
}

func TestLineBuf_MarshalJSONEmpty(t *testing.T) {
	t.Parallel()

	buf := NewLineBuf()

	data, err := buf.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `""`, string(data))
}

func TestLineBuf_MarshalJSONSpecialChars(t *testing.T) {
	t.Parallel()

	buf := NewLineBuf()
	buf.Set([]byte("ssh -q -l root -p 10022 -i /home/user/.ssh/id_ed25519 127.0.0.1 echo OK"))

	data, err := buf.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `"ssh -q -l root -p 10022 -i /home/user/.ssh/id_ed25519 127.0.0.1 echo OK"`, string(data))
}

func TestLineBuf_MarshalJSONWithQuotes(t *testing.T) {
	t.Parallel()

	buf := NewLineBuf()
	buf.Set([]byte(`NIX_SSHOPTS='-o StrictHostKeyChecking=no' nix build --store "ssh-ng://host"`))

	data, err := buf.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `"NIX_SSHOPTS='-o StrictHostKeyChecking=no' nix build --store \"ssh-ng://host\""`, string(data))
}

func TestLineBuf_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	buf := NewLineBuf()

	err := buf.UnmarshalJSON([]byte(`"hello world"`))
	require.NoError(t, err)

	assert.Equal(t, "hello world", string(buf.Bytes()))
}

func TestLineBuf_UnmarshalJSONEmpty(t *testing.T) {
	t.Parallel()

	buf := NewLineBuf()
	buf.Set([]byte("stale"))

	err := buf.UnmarshalJSON([]byte(`""`))
	require.NoError(t, err)

	assert.Zero(t, buf.Len())
}

func TestLineBuf_UnmarshalJSONInvalid(t *testing.T) {
	t.Parallel()

	buf := NewLineBuf()

	err := buf.UnmarshalJSON([]byte(`not json`))
	assert.Error(t, err, "invalid JSON should return an error")
}

func TestLineBuf_UnmarshalJSONObject(t *testing.T) {
	t.Parallel()

	buf := NewLineBuf()

	err := buf.UnmarshalJSON([]byte(`{}`))
	assert.Error(t, err, "object is not a string")
}

func TestLineBuf_MarshalUnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	original := NewLineBuf()
	original.Set([]byte("ssh -q -l root -p 10022 some-host echo OK"))

	data, err := original.MarshalJSON()
	require.NoError(t, err)

	restored := NewLineBuf()
	err = restored.UnmarshalJSON(data)
	require.NoError(t, err)

	assert.Equal(t, string(original.Bytes()), string(restored.Bytes()))
}

func TestLineBuf_MarshalUnmarshalRoundTripSpecialChars(t *testing.T) {
	t.Parallel()

	tests := []string{
		"simple command",
		"ssh -o StrictHostKeyChecking=no user@host",
		`NIX_SSHOPTS='-o IdentitiesOnly=yes' nix copy --to ssh-ng://host`,
		"path:/home/user/project#nixosConfigurations.test.config.system.build.toplevel",
		"",
		"a",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			t.Parallel()

			buf := NewLineBuf()
			buf.Set([]byte(test))

			data, err := buf.MarshalJSON()
			require.NoError(t, err)

			bufRestored := NewLineBuf()
			err = bufRestored.UnmarshalJSON(data)
			require.NoError(t, err)

			assert.Equal(t, test, string(bufRestored.Bytes()))
		})
	}
}

func TestLineBuf_MarshalEmbeddedInStruct(t *testing.T) {
	t.Parallel()

	type testStruct struct {
		Command *LineBuf `json:"command,omitempty"`
	}

	buf := NewLineBuf()
	buf.Set([]byte("ssh user@host echo OK"))

	s := testStruct{Command: buf}

	data, err := json.Marshal(s)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"command":"ssh user@host echo OK"`)
}

func TestLineBuf_MarshalNilEmbeddedInStruct(t *testing.T) {
	t.Parallel()

	type testStruct struct {
		Command *LineBuf `json:"command,omitempty"`
	}

	s := testStruct{}

	data, err := json.Marshal(s)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "command")
}
