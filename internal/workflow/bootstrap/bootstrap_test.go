package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTarArgs(t *testing.T) {
	t.Parallel()

	const tarballPath = "/tmp/panix-kexec-test0123456789/kexec.tar"

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"tar.gz extension", "https://example.com/file.tar.gz", []string{"-xzf", tarballPath}},
		{"tgz extension", "https://example.com/file.tgz", []string{"-xzf", tarballPath}},
		{"tar.xz extension", "https://example.com/file.tar.xz", []string{"-xJf", tarballPath}},
		{"tar.zst extension", "https://example.com/file.tar.zst", []string{"--use-compress-program=zstd", "-xf", tarballPath}},
		{"plain tar", "https://example.com/file.tar", []string{"-xf", tarballPath}},
		{"no extension", "https://example.com/file", []string{"-xf", tarballPath}},
		{"tar.bz2 falls to default", "https://example.com/file.tar.bz2", []string{"-xf", tarballPath}},
		{"local path tar.gz", "/local/path/file.tar.gz", []string{"-xzf", tarballPath}},
		{"local path tgz", "/local/path/file.tgz", []string{"-xzf", tarballPath}},
		{"local path tar.xz", "/local/path/file.tar.xz", []string{"-xJf", tarballPath}},
		{"local path tar.zst", "/local/path/file.tar.zst", []string{"--use-compress-program=zstd", "-xf", tarballPath}},
		{"uppercase extension", "https://example.com/file.TAR.GZ", []string{"-xf", tarballPath}},
		{"query params with tar.gz", "https://example.com/file.tar.gz?v=1", []string{"-xf", tarballPath}},
		{"fragment with tar.gz", "https://example.com/file.tar.gz#checksum", []string{"-xf", tarballPath}},
		{"double extension tar.gz.gz", "https://example.com/file.tar.gz.gz", []string{"-xf", tarballPath}},
		{"empty string", "", []string{"-xf", tarballPath}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.expected, getTarArgs(tt.input, tarballPath))
		})
	}
}
