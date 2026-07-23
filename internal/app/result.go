package app

import "github.com/jeppeklh/switchlet/internal/profile"

// ProfileItem describes one configured profile for list rendering.
type ProfileItem struct {
	Name              string
	Protected         bool
	Available         bool
	UnavailableReason string
}

// Result describes a successful profile application.
type Result struct {
	ProfileName    string
	Protected      bool
	Source         profile.ValueSource
	TargetPath     string
	ConnectionName string
}
