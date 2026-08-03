package editor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jeppeklh/switchlet/internal/config"
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

func readDotenvTargetValues(contents []byte, targets []config.Target) (map[string]string, error) {
	values := make(map[string]string, len(targets))
	if len(targets) == 0 {
		return values, nil
	}

	lines := splitDotenvLines(contents)
	assignments, err := parseDotenvAssignments(lines)
	if err != nil {
		return nil, targetError(targets[0], err)
	}

	for _, target := range targets {
		if err := validateDotenvKey(target.Key); err != nil {
			return nil, targetError(target, fmt.Errorf("dotenv key is invalid: %w", err))
		}

		value, err := readDotenvTargetValueFromAssignments(assignments, target.Key)
		if err != nil {
			return nil, targetError(target, err)
		}
		values[target.Name] = value
	}

	return values, nil
}

func readDotenvTargetValueFromAssignments(assignments map[string][]dotenvAssignment, key string) (string, error) {
	if err := validateDotenvKeyExistsOnce(assignments, key); err != nil {
		return "", err
	}

	return assignments[key][0].value, nil
}

func replaceDotenvTargetValue(lines []dotenvLine, assignments map[string][]dotenvAssignment, change TargetChange) error {
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

	assignment := assignments[key][0]
	replacement, err := encodeDotenvValue(assignment, change.Value)
	if err != nil {
		return targetError(change.Target, err)
	}

	lines[assignment.lineIndex].text = assignment.prefix + replacement + assignment.suffix

	return nil
}

type dotenvLine struct {
	text    string
	newline string
}

type dotenvQuoteStyle int

const (
	dotenvUnquotedStyle dotenvQuoteStyle = iota
	dotenvSingleQuotedStyle
	dotenvDoubleQuotedStyle
)

type dotenvAssignment struct {
	lineIndex  int
	key        string
	value      string
	prefix     string
	suffix     string
	quoteStyle dotenvQuoteStyle
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

func parseDotenvAssignments(lines []dotenvLine) (map[string][]dotenvAssignment, error) {
	assignments := make(map[string][]dotenvAssignment)

	for index, line := range lines {
		assignment, ok, err := parseDotenvAssignment(line.text, index+1)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		assignment.lineIndex = index
		assignments[assignment.key] = append(assignments[assignment.key], assignment)
	}

	return assignments, nil
}

func parseDotenvAssignment(lineText string, lineNumber int) (dotenvAssignment, bool, error) {
	trimmedLine := strings.TrimSpace(lineText)
	if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
		return dotenvAssignment{}, false, nil
	}

	keyStart := 0
	for keyStart < len(lineText) && isDotenvSpacing(lineText[keyStart]) {
		keyStart++
	}

	assignmentIndex := strings.IndexByte(lineText[keyStart:], '=')
	if assignmentIndex < 0 {
		return dotenvAssignment{}, false, fmt.Errorf("line %d is not a supported KEY=value assignment", lineNumber)
	}
	assignmentIndex += keyStart

	key := strings.TrimSpace(lineText[keyStart:assignmentIndex])
	if err := validateDotenvKey(key); err != nil {
		return dotenvAssignment{}, false, fmt.Errorf("line %d has invalid key %q: %w", lineNumber, key, err)
	}

	valueStart := assignmentIndex + 1
	for valueStart < len(lineText) && isDotenvSpacing(lineText[valueStart]) {
		valueStart++
	}

	assignment, err := parseDotenvAssignmentValue(lineText, assignmentIndex+1, valueStart)
	if err != nil {
		return dotenvAssignment{}, false, fmt.Errorf("line %d %w", lineNumber, err)
	}

	assignment.key = key
	assignment.prefix = lineText[:valueStart]
	return assignment, true, nil
}

func parseDotenvAssignmentValue(lineText string, rawValueStart int, valueStart int) (dotenvAssignment, error) {
	if valueStart >= len(lineText) {
		return dotenvAssignment{quoteStyle: dotenvUnquotedStyle}, nil
	}

	hadWhitespaceAfterEquals := valueStart > rawValueStart
	switch lineText[valueStart] {
	case '\'':
		value, suffix, err := parseSingleQuotedDotenvValue(lineText, valueStart)
		if err != nil {
			return dotenvAssignment{}, err
		}
		return dotenvAssignment{value: value, suffix: suffix, quoteStyle: dotenvSingleQuotedStyle}, nil
	case '"':
		value, suffix, err := parseDoubleQuotedDotenvValue(lineText, valueStart)
		if err != nil {
			return dotenvAssignment{}, err
		}
		return dotenvAssignment{value: value, suffix: suffix, quoteStyle: dotenvDoubleQuotedStyle}, nil
	default:
		value, suffix := parseUnquotedDotenvValue(lineText, valueStart, hadWhitespaceAfterEquals)
		return dotenvAssignment{value: value, suffix: suffix, quoteStyle: dotenvUnquotedStyle}, nil
	}
}

func parseSingleQuotedDotenvValue(lineText string, valueStart int) (string, string, error) {
	closingIndex := strings.IndexByte(lineText[valueStart+1:], '\'')
	if closingIndex < 0 {
		return "", "", fmt.Errorf("contains a single-quoted value without a closing quote")
	}
	closingIndex += valueStart + 1

	suffix, err := parseQuotedDotenvSuffix(lineText, closingIndex+1)
	if err != nil {
		return "", "", err
	}

	return lineText[valueStart+1 : closingIndex], suffix, nil
}

func parseDoubleQuotedDotenvValue(lineText string, valueStart int) (string, string, error) {
	closingIndex := findDotenvDoubleQuoteEnd(lineText, valueStart)
	if closingIndex < 0 {
		return "", "", fmt.Errorf("contains a double-quoted value without a closing quote")
	}

	value, err := strconv.Unquote(lineText[valueStart : closingIndex+1])
	if err != nil {
		return "", "", fmt.Errorf("contains a double-quoted value with unsupported escaping")
	}

	suffix, err := parseQuotedDotenvSuffix(lineText, closingIndex+1)
	if err != nil {
		return "", "", err
	}

	return value, suffix, nil
}

func parseQuotedDotenvSuffix(lineText string, suffixStart int) (string, error) {
	for index := suffixStart; index < len(lineText); index++ {
		if isDotenvSpacing(lineText[index]) {
			continue
		}
		if lineText[index] == '#' {
			return lineText[suffixStart:], nil
		}

		return "", fmt.Errorf("has unsupported trailing content after the quoted value")
	}

	return lineText[suffixStart:], nil
}

func parseUnquotedDotenvValue(lineText string, valueStart int, hadWhitespaceAfterEquals bool) (string, string) {
	if hadWhitespaceAfterEquals && lineText[valueStart] == '#' {
		return "", lineText[valueStart:]
	}

	commentStart := -1
	for index := valueStart + 1; index < len(lineText); index++ {
		if lineText[index] != '#' || !isDotenvSpacing(lineText[index-1]) {
			continue
		}

		commentStart = index
		for commentStart > valueStart && isDotenvSpacing(lineText[commentStart-1]) {
			commentStart--
		}
		break
	}

	valueEnd := len(lineText)
	suffixStart := len(lineText)
	if commentStart >= 0 {
		valueEnd = commentStart
		suffixStart = commentStart
	} else {
		valueEnd = trimTrailingDotenvSpacingIndex(lineText, valueStart)
		suffixStart = valueEnd
	}

	return lineText[valueStart:valueEnd], lineText[suffixStart:]
}

func encodeDotenvValue(assignment dotenvAssignment, value string) (string, error) {
	switch assignment.quoteStyle {
	case dotenvSingleQuotedStyle:
		if strings.Contains(value, "'") {
			return "", fmt.Errorf("replacement value cannot preserve the existing single-quoted dotenv style safely")
		}
		return "'" + value + "'", nil
	case dotenvDoubleQuotedStyle:
		return strconv.Quote(value), nil
	default:
		if strings.TrimSpace(value) != value {
			return "", fmt.Errorf("replacement value cannot preserve the existing unquoted dotenv style safely because it has leading or trailing whitespace")
		}
		if containsDotenvCommentPattern(value) {
			return "", fmt.Errorf("replacement value cannot preserve the existing unquoted dotenv style safely because it would be parsed as a comment")
		}
		return value, nil
	}
}

func findDotenvDoubleQuoteEnd(lineText string, valueStart int) int {
	for index := valueStart + 1; index < len(lineText); index++ {
		if lineText[index] == '"' && !isEscapedDotenvDoubleQuote(lineText, index) {
			return index
		}
	}

	return -1
}

func isEscapedDotenvDoubleQuote(lineText string, quoteIndex int) bool {
	backslashCount := 0
	for index := quoteIndex - 1; index >= 0 && lineText[index] == '\\'; index-- {
		backslashCount++
	}

	return backslashCount%2 == 1
}

func trimTrailingDotenvSpacingIndex(lineText string, valueStart int) int {
	valueEnd := len(lineText)
	for valueEnd > valueStart && isDotenvSpacing(lineText[valueEnd-1]) {
		valueEnd--
	}

	return valueEnd
}

func containsDotenvCommentPattern(value string) bool {
	for index := 1; index < len(value); index++ {
		if value[index] == '#' && isDotenvSpacing(value[index-1]) {
			return true
		}
	}

	return false
}

func validateDotenvKeyExistsOnce(assignments map[string][]dotenvAssignment, key string) error {
	matchingAssignments := assignments[key]
	switch len(matchingAssignments) {
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

func isDotenvSpacing(character byte) bool {
	return character == ' ' || character == '\t'
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
