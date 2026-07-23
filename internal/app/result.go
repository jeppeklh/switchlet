package app

// ProfileSource identifies how a profile is resolved for the UI.
type ProfileSource string

const (
	// ProfileSourceLiteral indicates that the profile value comes directly from configuration.
	ProfileSourceLiteral ProfileSource = "literal"
	// ProfileSourceEnvironment indicates that the profile value comes from an environment variable.
	ProfileSourceEnvironment ProfileSource = "environment"
)

// ProfileItem describes one configured profile for the terminal UI.
type ProfileItem struct {
	Name                    string
	Protected               bool
	Available               bool
	Source                  ProfileSource
	EnvironmentVariableName string
	MaskedValue             string
	UnavailableReason       string
}

// Result describes a successful profile application.
type Result struct {
	ProfileName string
	TargetPath  string
}
