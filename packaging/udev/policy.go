// Package udev exposes the canonical release policy as immutable content.
package udev

import _ "embed"

//go:embed 60-attack-shark-x6-hidraw.rules
var policy string

// Policy returns the exact packaged policy, including comments and final newline.
func Policy() string { return policy }
