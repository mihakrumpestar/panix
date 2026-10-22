package attributes

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/pkg/filemode"
)

type fileModeWrapper struct {
	Mode FileMode `yaml:"mode"`
}

func TestFileMode_UnmarshalYAML_CanonicalForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  FileMode
	}{
		{"default", "0700", 0o700},
		{"owner only", "0600", 0o600},
		{"group read", "0640", 0o640},
		{"owner rwx", "0755", 0o755},
		{"other execute", "0001", 0o0001},
		{"sticky", "1777", 0o1777},
		{"setuid", "4755", 0o4755},
		{"maximum", "7777", 0o7777},
		{"quoted", `"0700"`, 0o700},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var wrapper fileModeWrapper

			err := yaml.Unmarshal([]byte("mode: "+tt.value+"\n"), &wrapper)
			require.NoError(t, err)

			assert.Equal(t, tt.want, wrapper.Mode)
		})
	}
}

func TestFileMode_UnmarshalYAML_RejectsNonCanonical(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"600", "0o600", "0O600", "0000",
		"999", "0o999", "800", "409", "448",
		"-600", "abc", "10000", "0o10000",
	} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			var wrapper fileModeWrapper

			err := yaml.Unmarshal([]byte("mode: "+value+"\n"), &wrapper)
			require.Error(t, err)
		})
	}
}

func TestFileMode_String_IsOctal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode FileMode
		want string
	}{
		{"explicit mode", 0o600, "0600"},
		{"zero falls back to default", 0, "0700"},
		{"sticky", 0o1777, "1777"},
		{"maximum mode", 0o7777, "7777"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.mode.String())
		})
	}
}

func TestFileMode_StatString_IsUnpadded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode FileMode
		want string
	}{
		{0o600, "600"},
		{0, "700"},
		{0o1777, "1777"},
		{0o7777, "7777"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.mode.StatString())
		})
	}
}

func TestFileMode_DefaultMatchesCanonical(t *testing.T) {
	t.Parallel()

	assert.Equal(t, filemode.Default, filemode.Format(uint32(FileModeDefault)))
}

func TestDefaultRsyncFlags_EnforcePermissions(t *testing.T) {
	t.Parallel()

	assert.Contains(t, DefaultRsyncFlags, "--perms")

	for _, flag := range DefaultRsyncFlags {
		assert.NotEqual(t, "-P", flag, "partial-transfer flag must not be a default")
		assert.NotEqual(t, "--partial", flag, "partial-transfer flag must not be a default")
		assert.NotContains(t, flag, "P", "combined flags must not enable partial transfers: %q", flag)
	}
}

func TestDefaultCurlFlags_DoNotResume(t *testing.T) {
	t.Parallel()

	assert.Contains(t, DefaultCurlFlags, "-L", "curl must keep following redirects")

	for _, flag := range DefaultCurlFlags {
		assert.NotContains(t, flag, "-C", "resume flag must not be a default: %q", flag)
		assert.NotEqual(t, "--continue", flag, "resume flag must not be a default")
		assert.NotEqual(t, "--continue-at", flag, "resume flag must not be a default")
	}
}
