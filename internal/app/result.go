package app

import "github.com/jeppeklh/switchlet/internal/profile"

// ProfileItem describes one configured profile for the terminal UI.
type ProfileItem struct {
	Name                    string
	Protected               bool
	Available               bool
	Source                  profile.ValueSource
	EnvironmentVariableName string
	MaskedValue             string
	UnavailableReason       string
}

// Result describes a successful profile application.
type Result struct {
	ProfileName    string
	Protected      bool
	Source         profile.ValueSource
	TargetPath     string
	ConnectionName string
}
