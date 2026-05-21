package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"https url", "https://github.com/repo/file.tar.gz", true},
		{"http url", "http://example.com/file.tar.xz", true},
		{"https with port", "https://example.com:8080/file.tar", true},
		{"https with query", "https://example.com/file?version=1", true},
		{"absolute path", "/path/to/file.tar.gz", false},
		{"relative path", "file.tar.gz", false},
		{"relative path with dir", "some/dir/file.tar.gz", false},
		{"ftp scheme", "ftp://server/file.tar.gz", false},
		{"empty string", "", false},
		{"just scheme no host", "http://", false},
		{"scheme no host", "https:///path", false},
		{"file scheme", "file:///path/to/file.tar.gz", false},
		{"ssh url", "ssh://user@host/path", false},
		{"git url", "git://github.com/repo.git", false},
		{
			"complex https url",
			"https://github.com/nix-community/nixos-images/releases/" +
				"latest/download/nixos-kexec-installer-noninteractive-x86_64-linux.tar.gz",
			true,
		},
		{"url with fragment", "https://example.com/file.tar.gz#checksum", true},
		{"url with username", "https://user@example.com/file.tar.gz", true},
		{"url with credentials", "https://user:pass@example.com/file.tar.gz", true},
		{"ip address", "https://192.168.1.1/file.tar.gz", true},
		{"ipv6 address", "https://[::1]/file.tar.gz", true},
		{"localhost", "http://localhost/file.tar.gz", true},
		{"trailing slash", "https://example.com/", true},
		{"just domain", "https://example.com", true},
		{"unicode in path", "https://example.com/\u6587\u4EF6.tar.gz", true},
		{"space in url", "https://example.com/file name.tar.gz", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, isURL(tt.input))
		})
	}
}

func TestGetTarArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"tar.gz extension", "https://example.com/file.tar.gz", []string{"-xvzf", "/tmp/kexec/kexec.tar"}},
		{"tgz extension", "https://example.com/file.tgz", []string{"-xvzf", "/tmp/kexec/kexec.tar"}},
		{"tar.xz extension", "https://example.com/file.tar.xz", []string{"-xvJf", "/tmp/kexec/kexec.tar"}},
		{"tar.zst extension", "https://example.com/file.tar.zst", []string{"--use-compress-program=zstd", "-xvf", "/tmp/kexec/kexec.tar"}},
		{"plain tar", "https://example.com/file.tar", []string{"-xvf", "/tmp/kexec/kexec.tar"}},
		{"no extension", "https://example.com/file", []string{"-xvf", "/tmp/kexec/kexec.tar"}},
		{"tar.bz2 falls to default", "https://example.com/file.tar.bz2", []string{"-xvf", "/tmp/kexec/kexec.tar"}},
		{"local path tar.gz", "/local/path/file.tar.gz", []string{"-xvzf", "/tmp/kexec/kexec.tar"}},
		{"local path tgz", "/local/path/file.tgz", []string{"-xvzf", "/tmp/kexec/kexec.tar"}},
		{"local path tar.xz", "/local/path/file.tar.xz", []string{"-xvJf", "/tmp/kexec/kexec.tar"}},
		{"local path tar.zst", "/local/path/file.tar.zst", []string{"--use-compress-program=zstd", "-xvf", "/tmp/kexec/kexec.tar"}},
		{"uppercase extension", "https://example.com/file.TAR.GZ", []string{"-xvf", "/tmp/kexec/kexec.tar"}},
		{"query params with tar.gz", "https://example.com/file.tar.gz?v=1", []string{"-xvf", "/tmp/kexec/kexec.tar"}},
		{"fragment with tar.gz", "https://example.com/file.tar.gz#checksum", []string{"-xvf", "/tmp/kexec/kexec.tar"}},
		{"double extension tar.gz.gz", "https://example.com/file.tar.gz.gz", []string{"-xvf", "/tmp/kexec/kexec.tar"}},
		{"empty string", "", []string{"-xvf", "/tmp/kexec/kexec.tar"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.expected, getTarArgs(tt.input))
		})
	}
}
