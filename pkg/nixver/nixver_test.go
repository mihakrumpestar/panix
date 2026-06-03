package nixver

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNixVersion_Nix(t *testing.T) {
	t.Parallel()

	info := parseNixVersion("nix (Nix) 2.34.0")
	assert.Equal(t, FlavorNix, info.Flavor)
	assert.Equal(t, 2, info.Major)
	assert.Equal(t, 34, info.Minor)
	assert.Equal(t, 0, info.Patch)
	assert.Equal(t, "2.34.0", info.Version())
}

func TestParseNixVersion_NixWithPatch(t *testing.T) {
	t.Parallel()

	info := parseNixVersion("nix (Nix) 2.34.7")
	assert.Equal(t, FlavorNix, info.Flavor)
	assert.Equal(t, 2, info.Major)
	assert.Equal(t, 34, info.Minor)
	assert.Equal(t, 7, info.Patch)
	assert.Equal(t, "2.34.7", info.Version())
}

func TestParseNixVersion_Lix(t *testing.T) {
	t.Parallel()

	info := parseNixVersion("nix (Lix, like Nix) 2.94.0")
	assert.Equal(t, FlavorLix, info.Flavor)
	assert.Equal(t, 2, info.Major)
	assert.Equal(t, 94, info.Minor)
	assert.Equal(t, 0, info.Patch)
	assert.Equal(t, "2.94.0", info.Version())
}

func TestParseNixVersion_LixBeta(t *testing.T) {
	t.Parallel()

	info := parseNixVersion("nix (Lix, like Nix) 2.90-beta.0")
	assert.Equal(t, FlavorLix, info.Flavor)
	assert.Equal(t, 2, info.Major)
	assert.Equal(t, 90, info.Minor)
	assert.Equal(t, 0, info.Patch)
	assert.Equal(t, "2.90.0", info.Version())
}

func TestParseNixVersion_LixWithCommit(t *testing.T) {
	t.Parallel()

	info := parseNixVersion("nix (Lix, like Nix) 2.90-beta.1-lixpre20240506-b6799ab")
	assert.Equal(t, FlavorLix, info.Flavor)
	assert.Equal(t, 2, info.Major)
	assert.Equal(t, 90, info.Minor)
	assert.Equal(t, 0, info.Patch)
}

func TestParseNixVersion_EmptyString(t *testing.T) {
	t.Parallel()

	info := parseNixVersion("")
	assert.Equal(t, FlavorNix, info.Flavor)
	assert.Equal(t, 0, info.Major)
	assert.Equal(t, 0, info.Minor)
	assert.Equal(t, 0, info.Patch)
}

func TestParseNixVersion_Garbage(t *testing.T) {
	t.Parallel()

	info := parseNixVersion("garbage output")
	assert.Equal(t, FlavorNix, info.Flavor)
	assert.Equal(t, 0, info.Major)
	assert.Equal(t, 0, info.Minor)
	assert.Equal(t, 0, info.Patch)
}

func TestMarshalJSON(t *testing.T) {
	t.Parallel()

	info := parseNixVersion("nix (Lix, like Nix) 2.94.0")
	data, err := json.Marshal(info)
	require.NoError(t, err)
	assert.Equal(t, `"nix (Lix, like Nix) 2.94.0"`, string(data))
}

func TestMarshalJSON_Nix(t *testing.T) {
	t.Parallel()

	info := parseNixVersion("nix (Nix) 2.34.0")
	data, err := json.Marshal(info)
	require.NoError(t, err)
	assert.Equal(t, `"nix (Nix) 2.34.0"`, string(data))
}

func TestUnmarshalJSON(t *testing.T) {
	t.Parallel()

	var info Info

	err := json.Unmarshal([]byte(`"nix (Lix, like Nix) 2.94.0"`), &info)
	require.NoError(t, err)
	assert.Equal(t, FlavorLix, info.Flavor)
	assert.Equal(t, 2, info.Major)
	assert.Equal(t, 94, info.Minor)
	assert.Equal(t, "nix (Lix, like Nix) 2.94.0", info.Raw)
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	original := parseNixVersion("nix (Lix, like Nix) 2.94.0")
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored Info

	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, original.Flavor, restored.Flavor)
	assert.Equal(t, original.Major, restored.Major)
	assert.Equal(t, original.Minor, restored.Minor)
	assert.Equal(t, original.Patch, restored.Patch)
	assert.Equal(t, original.Raw, restored.Raw)
}

func TestString(t *testing.T) {
	t.Parallel()

	info := parseNixVersion("nix (Lix, like Nix) 2.94.0")
	assert.Equal(t, "nix (Lix, like Nix) 2.94.0", info.String())
}
