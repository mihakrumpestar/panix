package guard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrNoProfilePath marks an empty profile path where one is required.
var ErrNoProfilePath = errors.New("no profile path")

// ProfileTarget resolves the profile chain to its store path and the current generation
// number (profile -> system-N-link -> store path). A direct store path target yields
// gen 0 (no generation link: callers must handle that). Shared by the panix deployer
// and the guardian binary (spec 6.5 runtime rules).
func ProfileTarget(profile string) (string, int64, error) {
	if profile == "" {
		return "", 0, ErrNoProfilePath
	}

	cur := profile
	gen := int64(0)

	for range 5 {
		next, err := os.Readlink(cur)
		if err != nil {
			return "", 0, fmt.Errorf("readlink %s: %w", cur, err)
		}

		if !filepath.IsAbs(next) {
			next = filepath.Join(filepath.Dir(cur), next)
		}

		if gen == 0 {
			if g, ok := ParseGenLink(filepath.Base(next)); ok {
				gen = g
			}
		}

		cur = next
		if strings.HasPrefix(cur, "/nix/store/") {
			return cur, gen, nil
		}
	}

	return cur, gen, nil
}

// ParseGenLink extracts N from a profile generation link name (system-N-link,
// home-manager-N-link, ...).
func ParseGenLink(name string) (int64, bool) {
	if !strings.HasSuffix(name, "-link") {
		return 0, false
	}

	rest := strings.TrimSuffix(name, "-link")

	idx := strings.LastIndexByte(rest, '-')
	if idx < 0 {
		return 0, false
	}

	g, err := strconv.ParseInt(rest[idx+1:], 10, 64)
	if err != nil || g < 1 {
		return 0, false
	}

	return g, true
}
