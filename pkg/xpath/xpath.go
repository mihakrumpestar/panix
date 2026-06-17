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
func (x Xpath) FleetLeaf() (string, string, string) {
	xpathS := x.String()

	if xpathS == "" {
		return "", "", ""
	}

	parts := strings.Split(xpathS, "/")
	numOfParts := len(parts)

	switch numOfParts {
	case 1:
		return "", "", parts[0]
	case 2:
		return "", parts[0], parts[1]
	default:
		return parts[numOfParts-3], parts[numOfParts-2], parts[numOfParts-1]
	}
}

func (x Xpath) String() string {
	return string(x)
}
