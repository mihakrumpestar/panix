package osrelease

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEmpty(t *testing.T) {
	t.Parallel()

	got := Parse([]byte(""))
	assert.Empty(t, got)
}

func TestParseCommentAndBlankLines(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("# comment\n\nID=linux\n"))
	assert.Equal(t, map[string]string{"ID": "linux"}, got)
}

func TestParseUnquotedValue(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("ID=linux\nVERSION=1.0\n"))
	assert.Equal(t, map[string]string{"ID": "linux", "VERSION": "1.0"}, got)
}

func TestParseDoubleQuotedValue(t *testing.T) {
	t.Parallel()

	got := Parse([]byte(`PRETTY_NAME="Ubuntu 22.04"` + "\n"))
	assert.Equal(t, map[string]string{"PRETTY_NAME": "Ubuntu 22.04"}, got)
}

func TestParseSingleQuotedValue(t *testing.T) {
	t.Parallel()

	got := Parse([]byte(`NAME='My OS'` + "\n"))
	assert.Equal(t, map[string]string{"NAME": "My OS"}, got)
}

func TestParseSingleQuotedNoEscape(t *testing.T) {
	t.Parallel()

	got := Parse([]byte(`NAME='has \" \$ \\ \` + "'" + "\n"))
	assert.Equal(t, map[string]string{"NAME": `has \" \$ \\ \`}, got)
}

func TestParseDoubleQuotedEscape(t *testing.T) {
	t.Parallel()

	got := Parse([]byte(`NAME="has \" \$ \\ ` + "`" + `end"` + "\n"))
	assert.Equal(t, map[string]string{"NAME": "has \" $ \\ `end"}, got)
}

func TestParseUnquotedEscape(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("NAME=has\\\"\\$end\n"))
	assert.Equal(t, map[string]string{"NAME": `has"$end`}, got)
}

func TestParseWhitespaceAroundEquals(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("  ID  =  linux  \n"))
	assert.Equal(t, map[string]string{"ID": "linux"}, got)
}

func TestParseNoNewlineAtEnd(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("ID=linux"))
	assert.Equal(t, map[string]string{"ID": "linux"}, got)
}

func TestParseEmptyValue(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("ID=\n"))
	assert.Equal(t, map[string]string{"ID": ""}, got)
}

func TestParseLineWithoutEquals(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("NOEQUALS\nID=linux\n"))
	assert.Equal(t, map[string]string{"ID": "linux"}, got)
}

func TestParseDuplicateKey(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("ID=first\nID=second\n"))
	assert.Equal(t, map[string]string{"ID": "second"}, got)
}

func TestParseTypicalOsRelease(t *testing.T) {
	t.Parallel()

	input := `NAME="Ubuntu"
VERSION="22.04.3 LTS (Jammy Jellyfish)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 22.04.3 LTS"
VERSION_ID="22.04"
HOME_URL="https://www.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
` + "# comment line\n"

	want := map[string]string{
		"NAME":           "Ubuntu",
		"VERSION":        "22.04.3 LTS (Jammy Jellyfish)",
		"ID":             "ubuntu",
		"ID_LIKE":        "debian",
		"PRETTY_NAME":    "Ubuntu 22.04.3 LTS",
		"VERSION_ID":     "22.04",
		"HOME_URL":       "https://www.ubuntu.com/",
		"BUG_REPORT_URL": "https://bugs.launchpad.net/ubuntu/",
	}

	assert.Equal(t, want, Parse([]byte(input)))
}

func TestParseNoEscapingFastPath(t *testing.T) {
	t.Parallel()

	assert.Equal(t, map[string]string{"ID": "linux"}, Parse([]byte("ID=linux\n")))
}

func TestParseBackslashNotSpecialChar(t *testing.T) {
	t.Parallel()

	got := Parse([]byte(`PATH=/usr\local` + "\n"))
	assert.Equal(t, map[string]string{"PATH": `/usr\local`}, got)
}

func TestReadFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "os-release")

	content := `NAME="TestOS"
ID=test
`

	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	result, err := ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "TestOS", result["NAME"])
	assert.Equal(t, "test", result["ID"])
}

func TestReadFileNotFound(t *testing.T) {
	t.Parallel()

	_, err := ReadFile("/nonexistent/path/os-release")
	assert.Error(t, err)
}

func TestReadFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	etcPath := filepath.Join(dir, "etc-os-release")
	usrPath := filepath.Join(dir, "usr-os-release")

	require.NoError(t, os.WriteFile(usrPath, []byte("ID=fallback\n"), 0o600))

	_, err := ReadFile(etcPath)
	require.Error(t, err, "expected error for missing /etc file")

	result, err := ReadFile(usrPath)
	require.NoError(t, err)
	assert.Equal(t, "fallback", result["ID"])
}
