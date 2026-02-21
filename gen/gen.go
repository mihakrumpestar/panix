package gen

import (
	_ "embed"
)

//go:embed VERSION
var VersionRaw string

//go:generate go run ../cmd/panix/main.go schema
