package app

import "github.com/jeppeklh/switchlet/internal/profile"

// Result describes a successful profile application.
type Result struct {
	ProfileName    string
	Protected      bool
	Source         profile.ValueSource
	TargetPath     string
	ConnectionName string
}
