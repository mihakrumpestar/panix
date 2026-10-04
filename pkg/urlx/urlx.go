package urlx

import (
	"net/url"

	"github.com/pkg/errors"
)

// ErrUnsupportedURLScheme reports a source that is neither a local path nor
// an http(s) URL.
var ErrUnsupportedURLScheme = errors.New("unsupported URL scheme")

// IsHTTPURL reports whether s parses as an http(s) URL with a non-empty host.
func IsHTTPURL(s string) bool {
	parsedURL, err := url.Parse(s)
	if err != nil {
		return false
	}

	return isHTTPURL(parsedURL)
}

// ValidateSource reports whether s is usable as a transfer source: a local
// path (no URL scheme) or an http(s) URL. Any other scheme (ftp, file, s3,
// typos like htps) is rejected with ErrUnsupportedURLScheme naming the scheme,
// instead of silently falling through to the local-path branch. Unparseable
// input cannot carry a scheme and counts as a local path.
func ValidateSource(s string) error {
	parsedURL, _ := url.Parse(s)

	if parsedURL == nil || parsedURL.Scheme == "" || isHTTPURL(parsedURL) {
		return nil
	}

	return errors.Wrapf(ErrUnsupportedURLScheme, "%q", parsedURL.Scheme)
}

// Helpers

func isHTTPURL(parsedURL *url.URL) bool {
	return (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") && parsedURL.Host != ""
}
