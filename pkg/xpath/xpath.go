package xpath

import (
	"strings"
)

// Note: all methods are immutable.

type Xpath string

func New(xpath ...string) Xpath {
	return Xpath(strings.Join(xpath, "/"))
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

	// Fast path: single arg (most common case) — one allocation, no builder.
	if len(appendXpath) == 1 {
		return Xpath(xpathS + "/" + appendXpath[0])
	}

	return Xpath(xpathS + "/" + strings.Join(appendXpath, "/"))
}

// FleetLeaf returns the last 3 elements of the path as strings.
// Uses strings.LastIndex to avoid allocating a []string from strings.Split.
func (x Xpath) FleetLeaf() (string, string, string) {
	path := x.String()
	if path == "" {
		return "", "", ""
	}

	// Find last 3 segments by scanning backwards.
	last := strings.LastIndex(path, "/")
	if last < 0 {
		return "", "", path
	}

	machine := path[last+1:]
	rest := path[:last]

	mid := strings.LastIndex(rest, "/")
	if mid < 0 {
		return "", rest, machine
	}

	config := rest[mid+1:]
	rest = rest[:mid]

	first := strings.LastIndex(rest, "/")
	if first < 0 {
		return rest, config, machine
	}

	return rest[first+1:], config, machine
}

func (x Xpath) String() string {
	return string(x)
}
