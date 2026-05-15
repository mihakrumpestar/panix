package osrelease

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEmpty(t *testing.T) {
	t.Parallel()

	got := Parse([]byte(""))
	if len(got) != 0 {
		t.Fatalf("got %d keys, want 0", len(got))
	}
}

func TestParseCommentAndBlankLines(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("# comment\n\nID=linux\n"))
	assertMap(t, got, map[string]string{"ID": "linux"})
}

func TestParseUnquotedValue(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("ID=linux\nVERSION=1.0\n"))
	assertMap(t, got, map[string]string{"ID": "linux", "VERSION": "1.0"})
}

func TestParseDoubleQuotedValue(t *testing.T) {
	t.Parallel()

	got := Parse([]byte(`PRETTY_NAME="Ubuntu 22.04"` + "\n"))
	assertMap(t, got, map[string]string{"PRETTY_NAME": "Ubuntu 22.04"})
}

func TestParseSingleQuotedValue(t *testing.T) {
	t.Parallel()

	got := Parse([]byte(`NAME='My OS'` + "\n"))
	assertMap(t, got, map[string]string{"NAME": "My OS"})
}

func TestParseSingleQuotedNoEscape(t *testing.T) {
	t.Parallel()

	got := Parse([]byte(`NAME='has \" \$ \\ \` + "'" + "\n"))
	assertMap(t, got, map[string]string{"NAME": `has \" \$ \\ \`})
}

func TestParseDoubleQuotedEscape(t *testing.T) {
	t.Parallel()

	got := Parse([]byte(`NAME="has \" \$ \\ ` + "`" + `end"` + "\n"))
	assertMap(t, got, map[string]string{"NAME": "has \" $ \\ `end"})
}

func TestParseUnquotedEscape(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("NAME=has\\\"\\$end\n"))
	assertMap(t, got, map[string]string{"NAME": `has"$end`})
}

func TestParseWhitespaceAroundEquals(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("  ID  =  linux  \n"))
	assertMap(t, got, map[string]string{"ID": "linux"})
}

func TestParseNoNewlineAtEnd(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("ID=linux"))
	assertMap(t, got, map[string]string{"ID": "linux"})
}

func TestParseEmptyValue(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("ID=\n"))
	assertMap(t, got, map[string]string{"ID": ""})
}

func TestParseLineWithoutEquals(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("NOEQUALS\nID=linux\n"))
	assertMap(t, got, map[string]string{"ID": "linux"})
}

func TestParseDuplicateKey(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("ID=first\nID=second\n"))
	assertMap(t, got, map[string]string{"ID": "second"})
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

	got := Parse([]byte(input))
	assertMap(t, got, want)
}

func TestParseNoEscapingFastPath(t *testing.T) {
	t.Parallel()

	got := Parse([]byte("ID=linux\n"))
	assertMap(t, got, map[string]string{"ID": "linux"})
}

func TestParseBackslashNotSpecialChar(t *testing.T) {
	t.Parallel()

	got := Parse([]byte(`PATH=/usr\local` + "\n"))
	assertMap(t, got, map[string]string{"PATH": `/usr\local`})
}

func TestReadFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "os-release")

	content := `NAME="TestOS"
ID=test
`

	err := os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	result, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if result["NAME"] != "TestOS" {
		t.Errorf("NAME: got %q, want %q", result["NAME"], "TestOS")
	}

	if result["ID"] != "test" {
		t.Errorf("ID: got %q, want %q", result["ID"], "test")
	}
}

func TestReadFileNotFound(t *testing.T) {
	t.Parallel()

	_, err := ReadFile("/nonexistent/path/os-release")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	etcPath := filepath.Join(dir, "etc-os-release")
	usrPath := filepath.Join(dir, "usr-os-release")

	err := os.WriteFile(usrPath, []byte("ID=fallback\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ReadFile(etcPath)
	if err == nil {
		t.Error("expected error for missing /etc file")
	}

	result, err := ReadFile(usrPath)
	if err != nil {
		t.Fatalf("ReadFile fallback: %v", err)
	}

	if result["ID"] != "fallback" {
		t.Errorf("ID: got %q, want %q", result["ID"], "fallback")
	}
}

func assertMap(t *testing.T, got, want map[string]string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d keys, want %d keys: %+v", len(got), len(want), got)
	}

	for key, wantVal := range want {
		gotVal, ok := got[key]
		if !ok {
			t.Errorf("missing key %q", key)
		} else if gotVal != wantVal {
			t.Errorf("key %q: got %q, want %q", key, gotVal, wantVal)
		}
	}
}
