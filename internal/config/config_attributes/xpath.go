package config_attributes

import (
	"strings"
)

// Note: all methods are immutable

type Xpath struct {
	path string
}

func NewXpath(xpath ...string) Xpath {
	return Xpath{
		path: strings.Join(xpath, "/"),
	}
}

func (x *Xpath) Depth() int {
	if x.path == "" {
		return 0
	}
	return strings.Count(x.path, "/") + 1
}

// Creates a new Xpath based on current Xpath as base and appends appendXpath
// If current Xpath is nil, resulting Xpath consists only of appendXpath
func (x Xpath) NewXpathWithAppend(appendXpath ...string) Xpath {
	if x.path == "" {
		return NewXpath(appendXpath...)
	}

	return Xpath{
		path: x.path + "/" + strings.Join(appendXpath, "/"),
	}
}

// Check if provided xpath is a child of called Xpath
func (x Xpath) IsChild(xpath Xpath) bool {
	// Check if x.path is a prefix of xpath.path
	// Must be prefix AND child must be deeper
	return strings.HasPrefix(xpath.path, x.path+"/") && xpath.Depth() > x.Depth()
}

// Check if provided xpath is a parent of called Xpath
func (x Xpath) IsParent(xpath Xpath) bool {
	// Check if xpath.path is a prefix of x.path
	// Must be prefix AND current path must be deeper than the potential parent
	return strings.HasPrefix(x.path, xpath.path+"/") && x.Depth() > xpath.Depth()
}

func (x Xpath) String() string {
	return x.path
}
