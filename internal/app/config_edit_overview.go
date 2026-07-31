package app

import "github.com/jeppeklh/switchlet/internal/config"

// ConfigEditOverview is the presentation-neutral read model for the interactive
// config editor shell.
type ConfigEditOverview struct {
	ProjectRoot            string
	ConfigPath             string
	OriginalVersion        int
	ConvertsToVersionThree bool
	Profiles               []ConfigEditProfileItem
	ManagedValues          []ConfigEditManagedValueItem
	Changes                []ConfigEditChange
	Dirty                  bool
	Saveable               bool
	ValidationError        string
}

// ConfigEditProfileItem describes one draft profile without exposing literal values.
type ConfigEditProfileItem struct {
	Name               string
	Protected          bool
	ValueCount         int
	TotalManagedValues int
	Partial            bool
	Values             []ConfigEditProfileValueItem
}

// ConfigEditProfileValueItem describes one included profile value safely.
type ConfigEditProfileValueItem struct {
	TargetDescriptor
	Source                  ProfileSource
	EnvironmentVariableName string
}

// ConfigEditManagedValueItem describes one configured managed value target.
type ConfigEditManagedValueItem struct {
	TargetDescriptor
	IncludedProfileCount int
}

// BuildConfigEditOverview derives safe display data for the config editor TUI.
func (workflow ConfigEditWorkflow) BuildConfigEditOverview(document ConfigEditDocument) ConfigEditOverview {
	changes := workflow.SummarizeChanges(document)
	validationError := ""
	if err := workflow.ValidateDraft(document); err != nil {
		validationError = err.Error()
	}

	overview := ConfigEditOverview{
		ProjectRoot:            document.ProjectRoot,
		ConfigPath:             document.ConfigPath,
		OriginalVersion:        document.OriginalVersion,
		ConvertsToVersionThree: document.ConvertsToVersionThree,
		Changes:                changes,
		Dirty:                  len(changes) > 0,
		Saveable:               len(changes) > 0 && validationError == "",
		ValidationError:        validationError,
	}

	targetsByName := make(map[string]TargetDescriptor, len(document.Targets))
	for _, target := range document.Targets {
		descriptor := targetDescriptor(target)
		targetsByName[target.Name] = descriptor
		overview.ManagedValues = append(overview.ManagedValues, ConfigEditManagedValueItem{
			TargetDescriptor:     descriptor,
			IncludedProfileCount: len(profilesReferencingTarget(document.Profiles, target.Name)),
		})
	}

	for _, profile := range document.Profiles {
		overview.Profiles = append(overview.Profiles, configEditProfileItem(profile, targetsByName, len(document.Targets)))
	}

	return overview
}

func configEditProfileItem(profile config.Profile, targetsByName map[string]TargetDescriptor, totalManagedValues int) ConfigEditProfileItem {
	item := ConfigEditProfileItem{
		Name:               profile.Name,
		Protected:          profile.Protected,
		ValueCount:         len(profile.Values),
		TotalManagedValues: totalManagedValues,
		Partial:            len(profile.Values) < totalManagedValues,
		Values:             make([]ConfigEditProfileValueItem, 0, len(profile.Values)),
	}

	for _, value := range profile.Values {
		valueItem := ConfigEditProfileValueItem{
			TargetDescriptor: TargetDescriptor{TargetName: value.Target},
		}
		if descriptor, exists := targetsByName[value.Target]; exists {
			valueItem.TargetDescriptor = descriptor
		}
		if value.ValueFromEnv != nil {
			valueItem.Source = ProfileSourceEnvironment
			valueItem.EnvironmentVariableName = *value.ValueFromEnv
		} else {
			valueItem.Source = ProfileSourceLiteral
		}

		item.Values = append(item.Values, valueItem)
	}

	return item
}
