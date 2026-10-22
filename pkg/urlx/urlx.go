package urlx

import (
	"net/url"

	"github.com/pkg/errors"
)

// ErrUnsupportedURLScheme reports a source that is neither a local path nor
// an https URL.
var ErrUnsupportedURLScheme = errors.New("unsupported URL scheme")

// ErrInsecureURLScheme reports a plain http remote artifact source. Bootstrap
// artifacts are executed as root, so they must be fetched over https or come
// from a local path.
var ErrInsecureURLScheme = errors.New("plain http URLs are not allowed; use https or a local path")

// IsHTTPURL reports whether s parses as an http(s) URL with a non-empty host.
func IsHTTPURL(s string) bool {
	parsedURL, err := url.Parse(s)
	if err != nil {
		return false
	}

	return isHTTPURL(parsedURL)
}

// ValidateSource reports whether s is usable as an artifact source: a local
// path (no URL scheme) or an https URL. Plain http is rejected with
// ErrInsecureURLScheme, and any other scheme (ftp, file, s3, typos like htps)
// is rejected with ErrUnsupportedURLScheme naming the scheme, instead of
// silently falling through to the local-path branch. Input that fails to parse
// falls back to local-path treatment.
func ValidateSource(s string) error {
	parsedURL, _ := url.Parse(s)

	if parsedURL == nil || parsedURL.Scheme == "" {
		return nil
	}

	switch parsedURL.Scheme {
	case "https":
		if parsedURL.Host != "" {
			return nil
		}
	case "http":
		if parsedURL.Host != "" {
			return errors.Wrapf(ErrInsecureURLScheme, "%q", parsedURL.Scheme)
		}
	}

	return errors.Wrapf(ErrUnsupportedURLScheme, "%q", parsedURL.Scheme)
}

// Helpers

func isHTTPURL(parsedURL *url.URL) bool {
	return (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") && parsedURL.Host != ""
}
