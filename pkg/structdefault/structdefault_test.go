package structdefault

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// tagged declares a list default and a scalar one so the readers can be
// exercised without a configuration type.
type tagged struct {
	Flags    []string `default:"[-rcEx, --mkpath, --perms]"`
	Command  []string `default:"[nix, build]"`
	Scalar   string   `default:"panix.yml"`
	Untagged []string
}

// Apply only walks configuration packages, so its behaviour is pinned through
// the real config types in internal/config/attributes and internal/config/nix.

func TestString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
		want  string
	}{
		{name: "list value", field: "Flags", want: "[-rcEx, --mkpath, --perms]"},
		{name: "scalar value", field: "Scalar", want: "panix.yml"},
		{name: "another list value", field: "Command", want: "[nix, build]"},
		{name: "untagged", field: "Untagged"},
		{name: "unknown field", field: "NoSuchField"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, String[tagged](tt.field))
		})
	}
}

func TestList(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"-rcEx", "--mkpath", "--perms"}, List[tagged]("Flags"))
	assert.Equal(t, []string{"nix", "build"}, List[tagged]("Command"), "a list default is readable")
	assert.Nil(t, List[tagged]("Scalar"), "a scalar default is not a list")
	assert.Nil(t, List[tagged]("NoSuchField"))
}

func TestParseList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tag  string
		want []string
		ok   bool
	}{
		{name: "single", tag: "[rsync]", want: []string{"rsync"}, ok: true},
		{name: "several", tag: "[nix, build]", want: []string{"nix", "build"}, ok: true},
		{name: "empty list", tag: "[]", want: []string{}, ok: true},
		{name: "padded", tag: " [ --fail ,  -# , -L ] ", want: []string{"--fail", "-#", "-L"}, ok: true},
		{name: "inner spaces", tag: "[nix-command flakes]", want: []string{"nix-command flakes"}, ok: true},
		{name: "no brackets", tag: "rsync", ok: false},
		{name: "open bracket only", tag: "[rsync", ok: false},
		{name: "empty tag", tag: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ParseList(tt.tag)

			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
