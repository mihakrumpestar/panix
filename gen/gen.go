package gen

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var versionRaw string

func Version() string {
	return strings.TrimSpace(versionRaw)
}

//go:generate go run ../cmd/panix/main.go schema
