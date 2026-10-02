package xyzshell

import "golang.org/x/crypto/ssh"

// Algorithms reports the algorithms available in the active SSH library.
// It describes support, not a promise that a server will negotiate each one.
func Algorithms() ssh.Algorithms { return ssh.SupportedAlgorithms() }

// InsecureAlgorithms lists implemented legacy algorithms that are excluded
// from secure defaults. Callers should enable them only for known legacy hosts.
func InsecureAlgorithms() ssh.Algorithms { return ssh.InsecureAlgorithms() }
