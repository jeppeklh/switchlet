package editor

import (
	"fmt"
	"strings"
)

// ValidateDotenvTarget verifies that a dotenv file contains exactly one
// assignment for key.
func ValidateDotenvTarget(targetPath string, key string) error {
	contents, _, err := readTargetFile(targetPath)
	if err != nil {
		return err
	}

	if err := validateDotenvKey(key); err != nil {
		return fmt.Errorf("dotenv key %q is invalid: %w", key, err)
	}

	lines := splitDotenvLines(contents)
	assignments, err := parseDotenvAssignments(lines)
	if err != nil {
		return fmt.Errorf("validate dotenv file %q: %w", targetPath, err)
	}

	if err := validateDotenvKeyExistsOnce(assignments, key); err != nil {
		return fmt.Errorf("validate dotenv file %q key %q: %w", targetPath, key, err)
	}

	return nil
}

func replaceDotenvTargetValues(contents []byte, changes []TargetChange) ([]byte, error) {
	lines := splitDotenvLines(contents)
	assignments, err := parseDotenvAssignments(lines)
	if err != nil {
		return nil, targetError(changes[0].Target, err)
	}

	for _, change := range changes {
		if err := replaceDotenvTargetValue(lines, assignments, change); err != nil {
			return nil, err
		}
	}

	return serializeDotenvLines(lines), nil
}

func replaceDotenvTargetValue(lines []dotenvLine, assignments map[string][]int, change TargetChange) error {
	key := change.Target.Key
	if err := validateDotenvKey(key); err != nil {
		return targetError(change.Target, fmt.Errorf("dotenv key is invalid: %w", err))
	}
	if strings.ContainsAny(change.Value, "\r\n") {
		return targetError(change.Target, fmt.Errorf("replacement value must not contain newline characters"))
	}
	if err := validateDotenvKeyExistsOnce(assignments, key); err != nil {
		return targetError(change.Target, err)
	}

	lineIndex := assignments[key][0]
	lines[lineIndex].text = key + "=" + change.Value

	return nil
}

type dotenvLine struct {
	text    string
	newline string
}

func splitDotenvLines(contents []byte) []dotenvLine {
	text := string(contents)
	lines := make([]dotenvLine, 0)
	start := 0

	for index := 0; index < len(text); index++ {
		if text[index] != '\n' {
			continue
		}

		lineText := text[start:index]
		newline := "\n"
		if strings.HasSuffix(lineText, "\r") {
			lineText = strings.TrimSuffix(lineText, "\r")
			newline = "\r\n"
		}

		lines = append(lines, dotenvLine{text: lineText, newline: newline})
		start = index + 1
	}

	if start < len(text) {
		lines = append(lines, dotenvLine{text: text[start:]})
	}

	return lines
}

func parseDotenvAssignments(lines []dotenvLine) (map[string][]int, error) {
	assignments := make(map[string][]int)

	for index, line := range lines {
		trimmedLine := strings.TrimSpace(line.text)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		assignmentIndex := strings.Index(trimmedLine, "=")
		if assignmentIndex < 0 {
			return nil, fmt.Errorf("line %d is not a supported KEY=value assignment", index+1)
		}

		key := strings.TrimSpace(trimmedLine[:assignmentIndex])
		if err := validateDotenvKey(key); err != nil {
			return nil, fmt.Errorf("line %d has invalid key %q: %w", index+1, key, err)
		}

		assignments[key] = append(assignments[key], index)
	}

	return assignments, nil
}

func validateDotenvKeyExistsOnce(assignments map[string][]int, key string) error {
	lineIndexes := assignments[key]
	switch len(lineIndexes) {
	case 0:
		return fmt.Errorf("dotenv key does not exist")
	case 1:
		return nil
	default:
		return fmt.Errorf("dotenv key appears more than once")
	}
}

func serializeDotenvLines(lines []dotenvLine) []byte {
	var builder strings.Builder
	for _, line := range lines {
		builder.WriteString(line.text)
		builder.WriteString(line.newline)
	}

	return []byte(builder.String())
}

func validateDotenvKey(key string) error {
	if key == "" {
		return fmt.Errorf("must be set")
	}

	for index := 0; index < len(key); index++ {
		character := key[index]
		if index == 0 {
			if !isDotenvKeyStart(character) {
				return fmt.Errorf("must match [A-Za-z_][A-Za-z0-9_]*")
			}
			continue
		}

		if !isDotenvKeyPart(character) {
			return fmt.Errorf("must match [A-Za-z_][A-Za-z0-9_]*")
		}
	}

	return nil
}

func isDotenvKeyStart(character byte) bool {
	return character == '_' || character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
}

func isDotenvKeyPart(character byte) bool {
	return isDotenvKeyStart(character) || character >= '0' && character <= '9'
}
