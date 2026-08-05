package config

import (
	"fmt"
	"strings"
)

// ParseJSONPath validates and splits a JSON object-property path.
//
// Dots separate path segments. A backslash escapes the next character inside a
// segment, so `ConnectionStrings.Default\.Connection` selects the literal key
// `Default.Connection` under `ConnectionStrings`.
func ParseJSONPath(jsonPath string) ([]string, error) {
	return parseEscapedPath(jsonPath)
}

// ParseYAMLPath validates and splits a YAML mapping-key path.
//
// It uses the same escaped segment syntax as JSON paths.
func ParseYAMLPath(yamlPath string) ([]string, error) {
	return parseEscapedPath(yamlPath)
}

// ParseTOMLPath validates and splits a TOML table/key path.
//
// Escaping is parsed consistently with JSON and YAML, but TOML remains limited
// to bare-key segments that the editor can update safely.
func ParseTOMLPath(tomlPath string) ([]string, error) {
	segments, err := parseEscapedPath(tomlPath)
	if err != nil {
		return nil, err
	}

	for _, segment := range segments {
		if strings.Contains(segment, "*") {
			return nil, fmt.Errorf("wildcard selectors are not supported")
		}
		if strings.ContainsAny(segment, "[]") {
			return nil, fmt.Errorf("array selectors are not supported")
		}
		if !isTOMLBareKeySegment(segment) {
			return nil, fmt.Errorf("segment %q must use unquoted TOML bare-key syntax [A-Za-z0-9_-]+", segment)
		}
	}

	return segments, nil
}

// FormatJSONPath formats path segments using the documented JSON selector
// escaping rules.
func FormatJSONPath(pathSegments []string) (string, error) {
	return formatEscapedPath(pathSegments)
}

// FormatYAMLPath formats path segments using the documented YAML selector
// escaping rules.
func FormatYAMLPath(pathSegments []string) (string, error) {
	return formatEscapedPath(pathSegments)
}

func parseEscapedPath(path string) ([]string, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return nil, fmt.Errorf("path must be set")
	}

	segments := make([]string, 0)
	var segment strings.Builder
	escaping := false
	for _, character := range trimmedPath {
		if escaping {
			segment.WriteRune(character)
			escaping = false
			continue
		}

		switch character {
		case '\\':
			escaping = true
		case '.':
			parsedSegment := segment.String()
			if err := validatePathSegment(parsedSegment); err != nil {
				return nil, err
			}
			segments = append(segments, parsedSegment)
			segment.Reset()
		default:
			segment.WriteRune(character)
		}
	}

	if escaping {
		return nil, fmt.Errorf("path escape must be followed by a character")
	}

	parsedSegment := segment.String()
	if err := validatePathSegment(parsedSegment); err != nil {
		return nil, err
	}
	segments = append(segments, parsedSegment)

	return segments, nil
}

func formatEscapedPath(pathSegments []string) (string, error) {
	if len(pathSegments) == 0 {
		return "", fmt.Errorf("path must contain at least one segment")
	}

	formattedSegments := make([]string, 0, len(pathSegments))
	for _, segment := range pathSegments {
		if err := validatePathSegment(segment); err != nil {
			return "", err
		}
		formattedSegments = append(formattedSegments, escapePathSegment(segment))
	}

	return strings.Join(formattedSegments, "."), nil
}

func validatePathSegment(segment string) error {
	if segment == "" {
		return fmt.Errorf("path must contain non-empty dot-separated segments")
	}
	if strings.TrimSpace(segment) != segment {
		return fmt.Errorf("segment %q must not contain leading or trailing whitespace", segment)
	}

	return nil
}

func escapePathSegment(segment string) string {
	var escaped strings.Builder
	for _, character := range segment {
		if character == '.' || character == '\\' {
			escaped.WriteByte('\\')
		}
		escaped.WriteRune(character)
	}

	return escaped.String()
}

func isTOMLBareKeySegment(segment string) bool {
	if segment == "" {
		return false
	}

	for index := 0; index < len(segment); index++ {
		character := segment[index]
		if !(character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '_' || character == '-') {
			return false
		}
	}

	return true
}
