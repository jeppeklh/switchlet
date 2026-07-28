package main

import (
	"encoding/json"
	"io"

	"github.com/jeppeklh/switchlet/internal/app"
)

func writeListJSON(output io.Writer, profiles []app.ProfileItem) error {
	encodedProfiles := make([]profileJSON, 0, len(profiles))
	for _, profileItem := range profiles {
		encodedProfiles = append(encodedProfiles, profileJSONFromItem(profileItem))
	}

	return writeJSON(output, struct {
		Profiles []profileJSON `json:"profiles"`
	}{Profiles: encodedProfiles})
}

func writeInspectJSON(output io.Writer, profileItem app.ProfileItem) error {
	return writeJSON(output, struct {
		Profile profileJSON `json:"profile"`
	}{Profile: profileJSONFromItem(profileItem)})
}

func writeApplyJSON(output io.Writer, result app.Result) error {
	return writeJSON(output, struct {
		Result applyResultJSON `json:"result"`
	}{Result: applyResultJSON{
		ProfileName: result.ProfileName,
		Status:      applyResultStatus(result),
		TargetPath:  result.TargetPath,
		TargetFile:  result.TargetFile,
		TargetCount: len(result.Changes),
		Changes:     targetDescriptorJSONFromDescriptors(result.Changes),
		Protected:   result.Protected,
		DryRun:      result.DryRun,
	}})
}

func writeStatusJSON(output io.Writer, status app.StatusComparison) error {
	return writeJSON(output, struct {
		Result statusResultJSON `json:"result"`
	}{Result: statusResultJSON{
		Command:             "status",
		Status:              string(status.Status),
		CurrentProfile:      status.CurrentProfile,
		Matches:             profileMatchJSONFromMatches(status.Matches),
		MatchedTargets:      targetDescriptorJSONFromDescriptors(status.MatchedTargets),
		PartialMatches:      partialProfileMatchJSONFromMatches(status.PartialMatches),
		ClosestProfiles:     closestProfileMatchJSONFromMatches(status.ClosestProfiles),
		UnavailableProfiles: unavailableProfileJSONFromProfiles(status.UnavailableProfiles),
		TargetCount:         status.TargetCount,
		Complete:            status.Complete,
	}})
}

func writeDiffJSON(output io.Writer, diff app.ProfileDiff) error {
	return writeJSON(output, struct {
		Result diffResultJSON `json:"result"`
	}{Result: diffResultJSON{
		Command:        "diff",
		ProfileName:    diff.ProfileName,
		Protected:      diff.Protected,
		Complete:       diff.Complete,
		WouldUpdate:    targetDescriptorJSONFromDescriptors(diff.WouldUpdate),
		AlreadyMatches: targetDescriptorJSONFromDescriptors(diff.AlreadyMatches),
		Unavailable:    unavailableValueJSONFromValues(diff.Unavailable),
		OmittedTargets: targetDescriptorJSONFromDescriptors(diff.OmittedTargets),
	}})
}

func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

type profileJSON struct {
	Name                    string             `json:"name"`
	Status                  string             `json:"status"`
	Protected               bool               `json:"protected"`
	Available               bool               `json:"available"`
	Source                  app.ProfileSource  `json:"source"`
	EnvironmentVariableName string             `json:"environmentVariableName"`
	MaskedValue             string             `json:"maskedValue"`
	UnavailableReason       string             `json:"unavailableReason"`
	TargetCount             int                `json:"targetCount"`
	TotalTargets            int                `json:"totalTargets"`
	Partial                 bool               `json:"partial"`
	Values                  []profileValueJSON `json:"values"`
}

type applyResultJSON struct {
	ProfileName string                 `json:"profileName"`
	Status      string                 `json:"status"`
	TargetPath  string                 `json:"targetPath"`
	TargetFile  string                 `json:"targetFile"`
	TargetCount int                    `json:"targetCount"`
	Changes     []targetDescriptorJSON `json:"changes"`
	Protected   bool                   `json:"protected"`
	DryRun      bool                   `json:"dryRun"`
}

type statusResultJSON struct {
	Command             string                    `json:"command"`
	Status              string                    `json:"status"`
	CurrentProfile      string                    `json:"currentProfile"`
	Matches             []profileMatchJSON        `json:"matches"`
	MatchedTargets      []targetDescriptorJSON    `json:"matchedTargets"`
	PartialMatches      []partialProfileMatchJSON `json:"partialMatches"`
	ClosestProfiles     []closestProfileMatchJSON `json:"closestProfiles"`
	UnavailableProfiles []unavailableProfileJSON  `json:"unavailableProfiles"`
	TargetCount         int                       `json:"targetCount"`
	Complete            bool                      `json:"complete"`
}

type diffResultJSON struct {
	Command        string                 `json:"command"`
	ProfileName    string                 `json:"profileName"`
	Protected      bool                   `json:"protected"`
	Complete       bool                   `json:"complete"`
	WouldUpdate    []targetDescriptorJSON `json:"wouldUpdate"`
	AlreadyMatches []targetDescriptorJSON `json:"alreadyMatches"`
	Unavailable    []unavailableValueJSON `json:"unavailable"`
	OmittedTargets []targetDescriptorJSON `json:"omittedTargets"`
}

type profileValueJSON struct {
	TargetName              string            `json:"targetName"`
	TargetFile              string            `json:"targetFile"`
	TargetType              string            `json:"targetType"`
	SelectorName            string            `json:"selectorName"`
	Selector                string            `json:"selector"`
	Status                  string            `json:"status"`
	Available               bool              `json:"available"`
	Source                  app.ProfileSource `json:"source"`
	EnvironmentVariableName string            `json:"environmentVariableName"`
	MaskedValue             string            `json:"maskedValue"`
	UnavailableReason       string            `json:"unavailableReason"`
}

type targetDescriptorJSON struct {
	TargetName   string `json:"targetName"`
	TargetFile   string `json:"targetFile"`
	TargetType   string `json:"targetType"`
	SelectorName string `json:"selectorName"`
	Selector     string `json:"selector"`
}

type profileMatchJSON struct {
	ProfileName string `json:"profileName"`
	Protected   bool   `json:"protected"`
}

type partialProfileMatchJSON struct {
	ProfileName     string `json:"profileName"`
	Protected       bool   `json:"protected"`
	MatchedTargets  int    `json:"matchedTargets"`
	IncludedTargets int    `json:"includedTargets"`
	OmittedTargets  int    `json:"omittedTargets"`
	TargetCount     int    `json:"targetCount"`
}

type closestProfileMatchJSON struct {
	ProfileName        string `json:"profileName"`
	Protected          bool   `json:"protected"`
	MatchedTargets     int    `json:"matchedTargets"`
	IncludedTargets    int    `json:"includedTargets"`
	UnavailableTargets int    `json:"unavailableTargets"`
	TargetCount        int    `json:"targetCount"`
}

type unavailableProfileJSON struct {
	ProfileName string                 `json:"profileName"`
	Protected   bool                   `json:"protected"`
	Reason      string                 `json:"reason"`
	Values      []unavailableValueJSON `json:"values"`
}

type unavailableValueJSON struct {
	TargetName              string `json:"targetName"`
	TargetFile              string `json:"targetFile"`
	TargetType              string `json:"targetType"`
	SelectorName            string `json:"selectorName"`
	Selector                string `json:"selector"`
	EnvironmentVariableName string `json:"environmentVariable"`
	Reason                  string `json:"reason"`
}

func profileJSONFromItem(profileItem app.ProfileItem) profileJSON {
	return profileJSON{
		Name:                    profileItem.Name,
		Status:                  availabilityStatus(profileItem.Available),
		Protected:               profileItem.Protected,
		Available:               profileItem.Available,
		Source:                  profileItem.Source,
		EnvironmentVariableName: profileItem.EnvironmentVariableName,
		MaskedValue:             profileItem.MaskedValue,
		UnavailableReason:       profileItem.UnavailableReason,
		TargetCount:             profileItem.TargetCount,
		TotalTargets:            profileItem.TotalTargets,
		Partial:                 profileItem.Partial,
		Values:                  profileValueJSONFromItems(profileItem.Values),
	}
}

func profileValueJSONFromItems(values []app.ProfileValueItem) []profileValueJSON {
	encodedValues := make([]profileValueJSON, 0, len(values))
	for _, valueItem := range values {
		reason := valueItem.UnavailableReason
		encodedValues = append(encodedValues, profileValueJSON{
			TargetName:              valueItem.TargetName,
			TargetFile:              valueItem.TargetFile,
			TargetType:              string(valueItem.TargetType),
			SelectorName:            valueItem.SelectorName,
			Selector:                valueItem.Selector,
			Status:                  availabilityStatus(valueItem.Available),
			Available:               valueItem.Available,
			Source:                  valueItem.Source,
			EnvironmentVariableName: valueItem.EnvironmentVariableName,
			MaskedValue:             valueItem.MaskedValue,
			UnavailableReason:       reason,
		})
	}

	return encodedValues
}

func targetDescriptorJSONFromDescriptors(descriptors []app.TargetDescriptor) []targetDescriptorJSON {
	encodedDescriptors := make([]targetDescriptorJSON, 0, len(descriptors))
	for _, descriptor := range descriptors {
		encodedDescriptors = append(encodedDescriptors, targetDescriptorJSONFromDescriptor(descriptor))
	}

	return encodedDescriptors
}

func targetDescriptorJSONFromDescriptor(descriptor app.TargetDescriptor) targetDescriptorJSON {
	return targetDescriptorJSON{
		TargetName:   descriptor.TargetName,
		TargetFile:   descriptor.TargetFile,
		TargetType:   string(descriptor.TargetType),
		SelectorName: descriptor.SelectorName,
		Selector:     descriptor.Selector,
	}
}

func profileMatchJSONFromMatches(matches []app.ProfileMatch) []profileMatchJSON {
	encodedMatches := make([]profileMatchJSON, 0, len(matches))
	for _, match := range matches {
		encodedMatches = append(encodedMatches, profileMatchJSON{
			ProfileName: match.ProfileName,
			Protected:   match.Protected,
		})
	}

	return encodedMatches
}

func partialProfileMatchJSONFromMatches(matches []app.PartialProfileMatch) []partialProfileMatchJSON {
	encodedMatches := make([]partialProfileMatchJSON, 0, len(matches))
	for _, match := range matches {
		encodedMatches = append(encodedMatches, partialProfileMatchJSON{
			ProfileName:     match.ProfileName,
			Protected:       match.Protected,
			MatchedTargets:  match.MatchedTargets,
			IncludedTargets: match.IncludedTargets,
			OmittedTargets:  match.OmittedTargets,
			TargetCount:     match.TargetCount,
		})
	}

	return encodedMatches
}

func closestProfileMatchJSONFromMatches(matches []app.ClosestProfileMatch) []closestProfileMatchJSON {
	encodedMatches := make([]closestProfileMatchJSON, 0, len(matches))
	for _, match := range matches {
		encodedMatches = append(encodedMatches, closestProfileMatchJSON{
			ProfileName:        match.ProfileName,
			Protected:          match.Protected,
			MatchedTargets:     match.MatchedTargets,
			IncludedTargets:    match.IncludedTargets,
			UnavailableTargets: match.UnavailableTargets,
			TargetCount:        match.TargetCount,
		})
	}

	return encodedMatches
}

func unavailableProfileJSONFromProfiles(profiles []app.UnavailableProfile) []unavailableProfileJSON {
	encodedProfiles := make([]unavailableProfileJSON, 0, len(profiles))
	for _, profile := range profiles {
		encodedProfiles = append(encodedProfiles, unavailableProfileJSON{
			ProfileName: profile.ProfileName,
			Protected:   profile.Protected,
			Reason:      profile.Reason,
			Values:      unavailableValueJSONFromValues(profile.Values),
		})
	}

	return encodedProfiles
}

func unavailableValueJSONFromValues(values []app.UnavailableValue) []unavailableValueJSON {
	encodedValues := make([]unavailableValueJSON, 0, len(values))
	for _, value := range values {
		encodedValues = append(encodedValues, unavailableValueJSON{
			TargetName:              value.TargetName,
			TargetFile:              value.TargetFile,
			TargetType:              string(value.TargetType),
			SelectorName:            value.SelectorName,
			Selector:                value.Selector,
			EnvironmentVariableName: value.EnvironmentVariableName,
			Reason:                  value.Reason,
		})
	}

	return encodedValues
}
