package nixver

import (
	"fmt"
	"strings"
)

// parseNixVersion parses the output of "nix --version" into Info.
//
// Expected formats:
//
//	nix (Nix) 2.34.0              -> Nix, 2.34.0
//	nix (Lix, like Nix) 2.94.0    -> Lix, 2.94.0
//	nix (Lix, like Nix) 2.90-beta.0 -> Lix, 2.90.0
func parseNixVersion(input string) *Info {
	input = strings.TrimSpace(input)
	major, minor, patch := parseSemver(input)

	return &Info{
		Raw:    input,
		Flavor: detectFlavor(input),
		Major:  major,
		Minor:  minor,
		Patch:  patch,
	}
}

func detectFlavor(s string) Flavor {
	open, closeParen := strings.Index(s, "("), strings.Index(s, ")")
	if open >= 0 && closeParen > open && strings.Contains(s[open+1:closeParen], "Lix") {
		return FlavorLix
	}

	return FlavorNix
}

func parseSemver(s string) (int, int, int) {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return 0, 0, 0
	}

	ver := parts[len(parts)-1]
	if idx := strings.IndexByte(ver, '-'); idx >= 0 {
		ver = ver[:idx]
	}

	var major, minor, patch int

	_, _ = fmt.Sscanf(ver, "%d.%d.%d", &major, &minor, &patch)

	return major, minor, patch
}
