package runtimevars

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expandTests covers every reference form: expansion, hard errors, and byte
// faithful pass-through of foreign or malformed references.
//
//nolint:gochecknoglobals // deterministic test table
var expandTests = []struct {
	name    string
	vars    Vars
	input   string
	want    string
	wantErr string
}{
	{
		name:  "present name expanded with dollar form",
		vars:  Vars{Arch: "x86_64"},
		input: "kexec-$PANIX_ARCH.tar.gz",
		want:  "kexec-x86_64.tar.gz",
	},
	{
		name:  "present name expanded with brace form",
		vars:  Vars{Arch: "aarch64"},
		input: "kexec-${PANIX_ARCH}.tar.gz",
		want:  "kexec-aarch64.tar.gz",
	},
	{
		name:  "secret local path expanded",
		vars:  Vars{SecretLocalPath: "/tmp/panix/staged"}, //nolint:gosec // variable name, not a credential
		input: "cat ${PANIX_SECRET_LOCAL_PATH}",
		want:  "cat /tmp/panix/staged",
	},
	{
		name:  "multiple references",
		vars:  Vars{Arch: "x86_64", SecretLocalPath: "/tmp/s"},
		input: "$PANIX_ARCH:${PANIX_ARCH}:$PANIX_SECRET_LOCAL_PATH",
		want:  "x86_64:x86_64:/tmp/s",
	},
	{
		name:    "known but absent is an error",
		vars:    Vars{},
		input:   "$PANIX_ARCH",
		wantErr: "panix runtime variable PANIX_ARCH is not available in this context",
	},
	{
		name:    "known but absent with brace form is an error",
		vars:    Vars{Arch: "x86_64"},
		input:   "${PANIX_SECRET_LOCAL_PATH}",
		wantErr: "panix runtime variable PANIX_SECRET_LOCAL_PATH is not available in this context",
	},
	{
		name:    "unknown panix name is an error",
		vars:    Vars{Arch: "x86_64"},
		input:   "$PANIX_NOPE",
		wantErr: "unknown panix runtime variable PANIX_NOPE",
	},
	{
		name:    "unknown panix brace name is an error",
		vars:    Vars{Arch: "x86_64"},
		input:   "${PANIX_NOPE}",
		wantErr: "unknown panix runtime variable PANIX_NOPE",
	},
	{
		name:  "foreign variable untouched",
		vars:  Vars{Arch: "x86_64"},
		input: "echo $HOME and ${USER} and $VAR_1",
		want:  "echo $HOME and ${USER} and $VAR_1",
	},
	{
		name:  "lowercase panix prefix untouched",
		vars:  Vars{Arch: "x86_64"},
		input: "$panix_arch ${panix_arch}",
		want:  "$panix_arch ${panix_arch}",
	},
	{
		name:  "bare dollar untouched",
		vars:  Vars{Arch: "x86_64"},
		input: "costs $",
		want:  "costs $",
	},
	{
		name:  "positional and special shell references untouched",
		vars:  Vars{Arch: "x86_64"},
		input: "$1 $? $@ $$ $-",
		want:  "$1 $? $@ $$ $-",
	},
	{
		name:  "malformed brace references untouched",
		vars:  Vars{Arch: "x86_64"},
		input: "${} ${1ARCH} ${PANIX_ARCH ${PANIX-ARCH}",
		want:  "${} ${1ARCH} ${PANIX_ARCH ${PANIX-ARCH}",
	},
	{
		name:  "unterminated brace untouched",
		vars:  Vars{Arch: "x86_64"},
		input: "${PANIX_ARCH",
		want:  "${PANIX_ARCH",
	},
	{
		name:  "empty string untouched",
		vars:  Vars{Arch: "x86_64"},
		input: "",
		want:  "",
	},
	{
		name:  "no references untouched",
		vars:  Vars{Arch: "x86_64"},
		input: "plain value with no references",
		want:  "plain value with no references",
	},
	{
		name:  "present empty value expands to empty",
		vars:  Vars{Arch: ""},
		input: "[$PANIX_ARCH]",
		want:  "[]",
	},
}

func TestExpand(t *testing.T) {
	t.Parallel()

	for _, tt := range expandTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.vars.Expand(tt.input)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestExpand_ErrorLeavesNoPartialResult guards the contract that a failed
// expansion returns the zero value, never a partially expanded string.
func TestExpand_ErrorLeavesNoPartialResult(t *testing.T) {
	t.Parallel()

	got, err := Vars{Arch: "x86_64"}.Expand("prefix-$PANIX_ARCH-$PANIX_NOPE")

	require.Error(t, err)
	assert.Empty(t, got)
}

// TestExpand_ByteFaithfulnessNoReference verifies that a string with no panix
// references is returned byte for byte, including NUL and high bytes.
func TestExpand_ByteFaithfulnessNoReference(t *testing.T) {
	t.Parallel()

	input := "a\x00b\xff$HOME$-"

	got, err := Vars{}.Expand(input)

	require.NoError(t, err)
	assert.Equal(t, input, got)
}
