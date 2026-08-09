package nixver

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNixVersion_Nix(t *testing.T) {
	t.Parallel()

	info := ParseNixVersion("nix (Nix) 2.34.0")
	assert.Equal(t, FlavorNix, info.flavor)
	assert.Equal(t, 2, info.major)
	assert.Equal(t, 34, info.minor)
	assert.Equal(t, 0, info.patch)
	assert.Equal(t, "2.34.0", info.Version())
}

func TestParseNixVersion_NixWithPatch(t *testing.T) {
	t.Parallel()

	info := ParseNixVersion("nix (Nix) 2.34.7")
	assert.Equal(t, FlavorNix, info.flavor)
	assert.Equal(t, 2, info.major)
	assert.Equal(t, 34, info.minor)
	assert.Equal(t, 7, info.patch)
	assert.Equal(t, "2.34.7", info.Version())
}

func TestParseNixVersion_Lix(t *testing.T) {
	t.Parallel()

	info := ParseNixVersion("nix (Lix, like Nix) 2.94.0")
	assert.Equal(t, FlavorLix, info.flavor)
	assert.Equal(t, 2, info.major)
	assert.Equal(t, 94, info.minor)
	assert.Equal(t, 0, info.patch)
	assert.Equal(t, "2.94.0", info.Version())
}

func TestParseNixVersion_LixBeta(t *testing.T) {
	t.Parallel()

	info := ParseNixVersion("nix (Lix, like Nix) 2.90-beta.0")
	assert.Equal(t, FlavorLix, info.flavor)
	assert.Equal(t, 2, info.major)
	assert.Equal(t, 90, info.minor)
	assert.Equal(t, 0, info.patch)
	assert.Equal(t, "2.90.0", info.Version())
}

func TestParseNixVersion_LixWithCommit(t *testing.T) {
	t.Parallel()

	info := ParseNixVersion("nix (Lix, like Nix) 2.90-beta.1-lixpre20240506-b6799ab")
	assert.Equal(t, FlavorLix, info.flavor)
	assert.Equal(t, 2, info.major)
	assert.Equal(t, 90, info.minor)
	assert.Equal(t, 0, info.patch)
}

func TestParseNixVersion_EmptyString(t *testing.T) {
	t.Parallel()

	info := ParseNixVersion("")
	assert.Equal(t, FlavorNix, info.flavor)
	assert.Equal(t, 0, info.major)
	assert.Equal(t, 0, info.minor)
	assert.Equal(t, 0, info.patch)
}

func TestParseNixVersion_Garbage(t *testing.T) {
	t.Parallel()

	info := ParseNixVersion("garbage output")
	assert.Equal(t, FlavorNix, info.flavor)
	assert.Equal(t, 0, info.major)
	assert.Equal(t, 0, info.minor)
	assert.Equal(t, 0, info.patch)
}

func TestMarshalJSON(t *testing.T) {
	t.Parallel()

	info := ParseNixVersion("nix (Lix, like Nix) 2.94.0")
	data, err := json.Marshal(info)
	require.NoError(t, err)
	assert.Equal(t, `"nix (Lix, like Nix) 2.94.0"`, string(data))
}

func TestMarshalJSON_Nix(t *testing.T) {
	t.Parallel()

	info := ParseNixVersion("nix (Nix) 2.34.0")
	data, err := json.Marshal(info)
	require.NoError(t, err)
	assert.Equal(t, `"nix (Nix) 2.34.0"`, string(data))
}

func TestUnmarshalJSON(t *testing.T) {
	t.Parallel()

	var info Info

	err := json.Unmarshal([]byte(`"nix (Lix, like Nix) 2.94.0"`), &info)
	require.NoError(t, err)
	assert.Equal(t, FlavorLix, info.flavor)
	assert.Equal(t, 2, info.major)
	assert.Equal(t, 94, info.minor)
	assert.Equal(t, "nix (Lix, like Nix) 2.94.0", info.raw)
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	original := ParseNixVersion("nix (Lix, like Nix) 2.94.0")
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored Info

	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, original.flavor, restored.flavor)
	assert.Equal(t, original.major, restored.major)
	assert.Equal(t, original.minor, restored.minor)
	assert.Equal(t, original.patch, restored.patch)
	assert.Equal(t, original.raw, restored.raw)
}

func TestString(t *testing.T) {
	t.Parallel()

	info := ParseNixVersion("nix (Lix, like Nix) 2.94.0")
	assert.Equal(t, "nix (Lix, like Nix) 2.94.0", info.String())
}
