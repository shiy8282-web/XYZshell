package xyzshell

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var versionFile string

// Version is the application version declared in VERSION. Release builds are
// made from a tag whose version must match this file.
var Version = strings.TrimSpace(versionFile)

