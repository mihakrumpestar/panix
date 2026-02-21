package version

import _ "embed"

//go:embed VERSION
var raw string

var Version = raw
