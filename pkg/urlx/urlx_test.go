package urlx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsHTTPURL(t *testing.T) {
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
		{"s3 scheme", "s3://bucket/key", false},
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
			assertion.Equal(tt.want, IsHTTPURL(tt.input))
		})
	}
}

func TestValidateSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantErr    bool
		wantScheme string
	}{
		{"https url", "https://host/x", false, ""},
		{"http url", "http://host/x", false, ""},
		{"http scheme without host", "http://", true, "http"},
		{"ftp scheme", "ftp://host/x", true, "ftp"},
		{"file scheme", "file:///x", true, "file"},
		{"s3 scheme", "s3://bucket/key", true, "s3"},
		{"scheme typo", "htps://host/x", true, "htps"},
		{"colon in local name", "my:file.tar.gz", true, "my"},
		{"absolute path", "/tmp/x.tar.gz", false, ""},
		{"relative path", "./rel/x.tar.gz", false, ""},
		{"bare name", "plain-name.tar.gz", false, ""},
		{"empty string", "", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSource(tt.input)

			if !tt.wantErr {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, ErrUnsupportedURLScheme)
			assert.Contains(t, err.Error(), `"`+tt.wantScheme+`"`)
		})
	}
}
