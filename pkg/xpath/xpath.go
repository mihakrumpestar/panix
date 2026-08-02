package xpath

import (
	"strings"

	"github.com/mihakrumpestar/panix/pkg/stringbyte"
)

// Xpath is a string type for entity paths (e.g. "flake0/cfg0/m0").
type Xpath struct {
	stringbyte.StringByte
}

func New(xpathParts ...string) Xpath {
	return Xpath{stringbyte.StringByte(strings.Join(xpathParts, "/"))}
}

func (x Xpath) Depth() int {
	xS := x.String()
	if xS == "" {
		return 0
	}

	return strings.Count(xS, "/") + 1
}

// NewXpathWithAppend creates a new Xpath based on current Xpath as base and appends appendXpath.
// If current Xpath is nil, resulting Xpath consists only of appendXpath.
func (x Xpath) NewXpathWithAppend(appendXpath ...string) Xpath {
	xpathS := x.String()
	if xpathS == "" {
		return New(appendXpath...)
	}

	// Fast path: single arg (most common case), one allocation, no builder.
	if len(appendXpath) == 1 {
		return Xpath{stringbyte.StringByte(xpathS + "/" + appendXpath[0])}
	}

	return Xpath{stringbyte.StringByte(xpathS + "/" + strings.Join(appendXpath, "/"))}
}

// FleetLeaf returns the last 4 elements of the path as strings:
// (flake, outputType, outputName, machine).
// Uses strings.LastIndex to avoid allocating a []string from strings.Split.
func (x Xpath) FleetLeaf() (string, string, string, string) {
	path := x.String()
	if path == "" {
		return "", "", "", ""
	}

	// Segment 4 (machine)
	last := strings.LastIndex(path, "/")
	if last < 0 {
		return "", "", "", path
	}

	machine := path[last+1:]
	rest := path[:last]

	// Segment 3 (outputName)
	mid := strings.LastIndex(rest, "/")
	if mid < 0 {
		return "", "", rest, machine
	}

	outputName := rest[mid+1:]
	rest = rest[:mid]

	// Segment 2 (outputType)
	first := strings.LastIndex(rest, "/")
	if first < 0 {
		return "", rest, outputName, machine
	}

	outputType := rest[first+1:]
	rest = rest[:first]

	// Segment 1 (flake)
	flakeStart := strings.LastIndex(rest, "/")
	if flakeStart < 0 {
		return rest, outputType, outputName, machine
	}

	return rest[flakeStart+1:], outputType, outputName, machine
}
