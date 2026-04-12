package xpath

import (
	"encoding/json"
	"strings"
)

// Note: all methods are immutable.

type Xpath struct {
	path string
}

func New(xpath ...string) Xpath {
	return Xpath{
		path: strings.Join(xpath, "/"),
	}
}

func (x Xpath) Depth() int {
	if x.path == "" {
		return 0
	}

	return strings.Count(x.path, "/") + 1
}

// NewXpathWithAppend creates a new Xpath based on current Xpath as base and appends appendXpath.
// If current Xpath is nil, resulting Xpath consists only of appendXpath.
func (x Xpath) NewXpathWithAppend(appendXpath ...string) Xpath {
	if x.path == "" {
		return New(appendXpath...)
	}

	var builder strings.Builder

	builder.WriteString(x.path)
	builder.WriteByte('/')
	builder.WriteString(strings.Join(appendXpath, "/"))

	return Xpath{path: builder.String()}
}

// IsChild checks if provided xpath is a child of called Xpath.
func (x Xpath) IsChild(xpath Xpath) bool {
	// Check if x.path is a prefix of xpath.path
	// Must be prefix AND child must be deeper (longer string)
	return strings.HasPrefix(xpath.path, x.path+"/") && len(xpath.path) > len(x.path)
}

// IsParent checks if provided xpath is a parent of called Xpath.
func (x Xpath) IsParent(xpath Xpath) bool {
	// Check if xpath.path is a prefix of x.path
	// Must be prefix AND current path must be deeper than the potential parent
	return strings.HasPrefix(x.path, xpath.path+"/") && len(x.path) > len(xpath.path)
}

// Element returns the xpath element at the given index.
// 0 returns the first element, 1 returns the second, etc.
// -1 returns the last element, -2 returns the second-to-last, etc.
// Returns empty string if the index is out of range.
func (x Xpath) Element(index int) string {
	if x.path == "" {
		return ""
	}

	parts := strings.Split(x.path, "/")

	if index < 0 {
		index += len(parts)
	}

	if index < 0 || index >= len(parts) {
		return ""
	}

	return parts[index]
}

// FleetLeaf returns the last 3 elements of the path as strings.
func (x Xpath) FleetLeaf() (string, string, string) {
	if x.path == "" {
		return "", "", ""
	}

	parts := strings.Split(x.path, "/")
	n := len(parts)

	switch n {
	case 1:
		return "", "", parts[0]
	case 2:
		return "", parts[0], parts[1]
	default:
		return parts[n-3], parts[n-2], parts[n-1]
	}
}

func (x Xpath) String() string {
	return x.path
}

func (x Xpath) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.path)
}

func (x *Xpath) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err //nolint:wrapcheck
	}

	x.path = s

	return nil
}
