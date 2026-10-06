package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTarArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"tar.gz extension", "https://example.com/file.tar.gz", []string{"-xzf", "/tmp/kexec/kexec.tar"}},
		{"tgz extension", "https://example.com/file.tgz", []string{"-xzf", "/tmp/kexec/kexec.tar"}},
		{"tar.xz extension", "https://example.com/file.tar.xz", []string{"-xJf", "/tmp/kexec/kexec.tar"}},
		{"tar.zst extension", "https://example.com/file.tar.zst", []string{"--use-compress-program=zstd", "-xf", "/tmp/kexec/kexec.tar"}},
		{"plain tar", "https://example.com/file.tar", []string{"-xf", "/tmp/kexec/kexec.tar"}},
		{"no extension", "https://example.com/file", []string{"-xf", "/tmp/kexec/kexec.tar"}},
		{"tar.bz2 falls to default", "https://example.com/file.tar.bz2", []string{"-xf", "/tmp/kexec/kexec.tar"}},
		{"local path tar.gz", "/local/path/file.tar.gz", []string{"-xzf", "/tmp/kexec/kexec.tar"}},
		{"local path tgz", "/local/path/file.tgz", []string{"-xzf", "/tmp/kexec/kexec.tar"}},
		{"local path tar.xz", "/local/path/file.tar.xz", []string{"-xJf", "/tmp/kexec/kexec.tar"}},
		{"local path tar.zst", "/local/path/file.tar.zst", []string{"--use-compress-program=zstd", "-xf", "/tmp/kexec/kexec.tar"}},
		{"uppercase extension", "https://example.com/file.TAR.GZ", []string{"-xf", "/tmp/kexec/kexec.tar"}},
		{"query params with tar.gz", "https://example.com/file.tar.gz?v=1", []string{"-xf", "/tmp/kexec/kexec.tar"}},
		{"fragment with tar.gz", "https://example.com/file.tar.gz#checksum", []string{"-xf", "/tmp/kexec/kexec.tar"}},
		{"double extension tar.gz.gz", "https://example.com/file.tar.gz.gz", []string{"-xf", "/tmp/kexec/kexec.tar"}},
		{"empty string", "", []string{"-xf", "/tmp/kexec/kexec.tar"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.expected, getTarArgs(tt.input))
		})
	}
}
