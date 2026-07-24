package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
)

func writeListText(output io.Writer, profiles []app.ProfileItem) error {
	for _, profileItem := range profiles {
		indicators := make([]string, 0, 2)
		if profileItem.Protected {
			indicators = append(indicators, "protected")
		}
		if !profileItem.Available {
			indicators = append(indicators, "unavailable")
		}

		line := profileItem.Name
		if len(indicators) > 0 {
			line += " [" + strings.Join(indicators, ", ") + "]"
		}

		if _, err := fmt.Fprintln(output, line); err != nil {
			return err
		}
		if profileItem.UnavailableReason != "" {
			if _, err := fmt.Fprintf(output, "  reason: %s\n", profileItem.UnavailableReason); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeInspectText(output io.Writer, profileItem app.ProfileItem) error {
	if _, err := fmt.Fprintf(output, "Profile: %s\n", profileItem.Name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "Availability: %s\n", availabilityLabel(profileItem)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "Source: %s\n", sourceLabel(profileItem.Source)); err != nil {
		return err
	}
	if profileItem.EnvironmentVariableName != "" {
		if _, err := fmt.Fprintf(output, "Environment variable: %s\n", profileItem.EnvironmentVariableName); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(output, "Protection: %s\n\n", protectionLabel(profileItem)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "Masked value:\n%s\n", maskedValueLabel(profileItem)); err != nil {
		return err
	}
	if profileItem.UnavailableReason != "" {
		if _, err := fmt.Fprintf(output, "\nResolution error:\n%s\n", profileItem.UnavailableReason); err != nil {
			return err
		}
	}

	return nil
}

func writeApplyText(output io.Writer, result app.Result) error {
	if result.DryRun {
		if _, err := fmt.Fprintf(output, "Dry run successful: %s\n\nTarget file:\n%s\n\nTarget JSON path:\n%s\n\nNo changes were written.\n", result.ProfileName, result.TargetFile, result.TargetPath); err != nil {
			return err
		}

		return nil
	}

	if _, err := fmt.Fprintf(output, "Applied profile: %s\n\nTarget file:\n%s\n\nUpdated target:\n%s\n", result.ProfileName, result.TargetFile, result.TargetPath); err != nil {
		return err
	}

	return nil
}

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
		TargetPath:  result.TargetPath,
		TargetFile:  result.TargetFile,
		Protected:   result.Protected,
		DryRun:      result.DryRun,
	}})
}

func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func availabilityLabel(profileItem app.ProfileItem) string {
	if profileItem.Available {
		return "Available"
	}

	return "Unavailable"
}

func sourceLabel(source app.ProfileSource) string {
	switch source {
	case app.ProfileSourceEnvironment:
		return "Environment variable"
	case app.ProfileSourceLiteral:
		return "Literal"
	default:
		return "Unknown"
	}
}

func protectionLabel(profileItem app.ProfileItem) string {
	if profileItem.Protected {
		return "Protected"
	}

	return "Not protected"
}

func maskedValueLabel(profileItem app.ProfileItem) string {
	if !profileItem.Available {
		return "Unavailable"
	}
	if profileItem.MaskedValue == "" {
		return "<empty>"
	}

	return profileItem.MaskedValue
}

type profileJSON struct {
	Name                    string            `json:"name"`
	Protected               bool              `json:"protected"`
	Available               bool              `json:"available"`
	Source                  app.ProfileSource `json:"source"`
	EnvironmentVariableName string            `json:"environmentVariableName"`
	MaskedValue             string            `json:"maskedValue"`
	UnavailableReason       string            `json:"unavailableReason"`
}

type applyResultJSON struct {
	ProfileName string `json:"profileName"`
	TargetPath  string `json:"targetPath"`
	TargetFile  string `json:"targetFile"`
	Protected   bool   `json:"protected"`
	DryRun      bool   `json:"dryRun"`
}

func profileJSONFromItem(profileItem app.ProfileItem) profileJSON {
	return profileJSON{
		Name:                    profileItem.Name,
		Protected:               profileItem.Protected,
		Available:               profileItem.Available,
		Source:                  profileItem.Source,
		EnvironmentVariableName: profileItem.EnvironmentVariableName,
		MaskedValue:             profileItem.MaskedValue,
		UnavailableReason:       profileItem.UnavailableReason,
	}
}
