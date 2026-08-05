package editor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jeppeklh/switchlet/internal/config"
)

func replaceStringValue(contents []byte, jsonPath string, replacementValue string) ([]byte, error) {
	rootObject, targetObject, targetKey, err := parseStringTarget(contents, jsonPath)
	if err != nil {
		return nil, err
	}

	updates := []jsonStringValueUpdate{{
		jsonPath:         jsonPath,
		targetObject:     targetObject,
		targetKey:        targetKey,
		replacementValue: replacementValue,
	}}
	if updatedContents, ok := replaceJSONStringTokens(contents, updates); ok {
		return updatedContents, nil
	}

	targetObject[targetKey] = replacementValue

	return serializeJSONRoot(rootObject)
}

func replaceJSONTargetValues(contents []byte, changes []TargetChange) ([]byte, error) {
	rootObject, err := parseRootObject(contents)
	if err != nil {
		return nil, targetError(changes[0].Target, err)
	}

	updates := make([]jsonStringValueUpdate, 0, len(changes))
	for _, change := range changes {
		targetObject, targetKey, err := findStringTarget(rootObject, change.Target.JSONPath)
		if err != nil {
			return nil, targetError(change.Target, err)
		}

		updates = append(updates, jsonStringValueUpdate{
			jsonPath:         change.Target.JSONPath,
			targetObject:     targetObject,
			targetKey:        targetKey,
			replacementValue: change.Value,
		})
	}

	if updatedContents, ok := replaceJSONStringTokens(contents, updates); ok {
		return updatedContents, nil
	}

	for _, update := range updates {
		update.targetObject[update.targetKey] = update.replacementValue
	}

	return serializeJSONRoot(rootObject)
}

type jsonStringValueUpdate struct {
	jsonPath         string
	targetObject     map[string]any
	targetKey        string
	replacementValue string
}

type jsonStringValueRange struct {
	start int
	end   int
}

type jsonStringTokenReplacement struct {
	valueRange       jsonStringValueRange
	replacementValue []byte
}

func readJSONTargetValues(contents []byte, targets []config.Target) (map[string]string, error) {
	values := make(map[string]string, len(targets))
	if len(targets) == 0 {
		return values, nil
	}

	rootObject, err := parseRootObject(contents)
	if err != nil {
		return nil, targetError(targets[0], err)
	}

	for _, target := range targets {
		targetObject, targetKey, err := findStringTarget(rootObject, target.JSONPath)
		if err != nil {
			return nil, targetError(target, err)
		}

		targetValue, ok := targetObject[targetKey].(string)
		if !ok {
			return nil, targetError(target, fmt.Errorf("JSON path %q must resolve to a string", target.JSONPath))
		}
		values[target.Name] = targetValue
	}

	return values, nil
}

func replaceJSONStringTokens(contents []byte, updates []jsonStringValueUpdate) ([]byte, bool) {
	rangesByPath, err := collectJSONStringValueRanges(contents)
	if err != nil {
		return nil, false
	}

	replacements := make([]jsonStringTokenReplacement, 0, len(updates))
	for _, update := range updates {
		pathSegments, err := config.ParseJSONPath(update.jsonPath)
		if err != nil {
			return nil, false
		}

		valueRange, ok := rangesByPath[selectorSegmentsKey(pathSegments)]
		if !ok {
			return nil, false
		}

		replacementValue, err := json.Marshal(update.replacementValue)
		if err != nil {
			return nil, false
		}
		replacements = append(replacements, jsonStringTokenReplacement{
			valueRange:       valueRange,
			replacementValue: replacementValue,
		})
	}

	sort.Slice(replacements, func(leftIndex int, rightIndex int) bool {
		return replacements[leftIndex].valueRange.start > replacements[rightIndex].valueRange.start
	})

	updatedContents := append([]byte(nil), contents...)
	for _, replacement := range replacements {
		start := replacement.valueRange.start
		end := replacement.valueRange.end
		if start < 0 || end < start || end > len(updatedContents) {
			return nil, false
		}

		replacedContents := make([]byte, 0, len(updatedContents)-(end-start)+len(replacement.replacementValue))
		replacedContents = append(replacedContents, updatedContents[:start]...)
		replacedContents = append(replacedContents, replacement.replacementValue...)
		replacedContents = append(replacedContents, updatedContents[end:]...)
		updatedContents = replacedContents
	}

	return updatedContents, true
}

func collectJSONStringValueRanges(contents []byte) (map[string]jsonStringValueRange, error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()

	rootToken, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	rootDelimiter, ok := rootToken.(json.Delim)
	if !ok || rootDelimiter != '{' {
		return nil, fmt.Errorf("must contain a JSON object at the root")
	}

	rangesByPath := make(map[string]jsonStringValueRange)
	if err := collectJSONObjectStringValueRanges(decoder, contents, nil, true, rangesByPath); err != nil {
		return nil, err
	}

	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values are not allowed")
		}
		return nil, err
	}

	return rangesByPath, nil
}

func collectJSONObjectStringValueRanges(decoder *json.Decoder, contents []byte, parentSegments []string, recordValues bool, rangesByPath map[string]jsonStringValueRange) error {
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}

		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("object keys must be strings")
		}

		pathSegments := appendPathSegment(parentSegments, key)
		if err := collectJSONValueStringRanges(decoder, contents, pathSegments, recordValues, rangesByPath); err != nil {
			return err
		}
	}

	endToken, err := decoder.Token()
	if err != nil {
		return err
	}
	endDelimiter, ok := endToken.(json.Delim)
	if !ok || endDelimiter != '}' {
		return fmt.Errorf("expected JSON object to end")
	}

	return nil
}

func collectJSONArrayStringValueRanges(decoder *json.Decoder, contents []byte, rangesByPath map[string]jsonStringValueRange) error {
	for decoder.More() {
		if err := collectJSONValueStringRanges(decoder, contents, nil, false, rangesByPath); err != nil {
			return err
		}
	}

	endToken, err := decoder.Token()
	if err != nil {
		return err
	}
	endDelimiter, ok := endToken.(json.Delim)
	if !ok || endDelimiter != ']' {
		return fmt.Errorf("expected JSON array to end")
	}

	return nil
}

func collectJSONValueStringRanges(decoder *json.Decoder, contents []byte, pathSegments []string, recordValue bool, rangesByPath map[string]jsonStringValueRange) error {
	valueToken, err := decoder.Token()
	if err != nil {
		return err
	}

	switch value := valueToken.(type) {
	case json.Delim:
		switch value {
		case '{':
			return collectJSONObjectStringValueRanges(decoder, contents, pathSegments, recordValue, rangesByPath)
		case '[':
			return collectJSONArrayStringValueRanges(decoder, contents, rangesByPath)
		default:
			return fmt.Errorf("unexpected JSON delimiter %q", value)
		}
	case string:
		if !recordValue {
			return nil
		}

		valueRange, ok := jsonStringTokenRange(contents, int(decoder.InputOffset()))
		if !ok {
			return fmt.Errorf("could not locate JSON string token")
		}
		rangesByPath[selectorSegmentsKey(pathSegments)] = valueRange
	}

	return nil
}

func jsonStringTokenRange(contents []byte, endOffset int) (jsonStringValueRange, bool) {
	closingQuoteIndex := endOffset - 1
	for closingQuoteIndex >= 0 && isJSONWhitespace(contents[closingQuoteIndex]) {
		closingQuoteIndex--
	}
	if closingQuoteIndex < 0 || contents[closingQuoteIndex] != '"' {
		return jsonStringValueRange{}, false
	}

	for startIndex := closingQuoteIndex - 1; startIndex >= 0; startIndex-- {
		if contents[startIndex] != '"' || isEscapedJSONStringQuote(contents, startIndex) {
			continue
		}

		return jsonStringValueRange{start: startIndex, end: closingQuoteIndex + 1}, true
	}

	return jsonStringValueRange{}, false
}

func isEscapedJSONStringQuote(contents []byte, quoteIndex int) bool {
	backslashCount := 0
	for index := quoteIndex - 1; index >= 0 && contents[index] == '\\'; index-- {
		backslashCount++
	}

	return backslashCount%2 == 1
}

func isJSONWhitespace(character byte) bool {
	return character == ' ' || character == '\n' || character == '\r' || character == '\t'
}

func serializeJSONRoot(rootObject map[string]any) ([]byte, error) {
	updatedContents, err := json.MarshalIndent(rootObject, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize updated JSON: %w", err)
	}

	return append(updatedContents, '\n'), nil
}

func parseStringTarget(contents []byte, jsonPath string) (map[string]any, map[string]any, string, error) {
	rootObject, err := parseRootObject(contents)
	if err != nil {
		return nil, nil, "", err
	}

	targetObject, targetKey, err := findStringTarget(rootObject, jsonPath)
	if err != nil {
		return nil, nil, "", err
	}

	return rootObject, targetObject, targetKey, nil
}

func findStringTarget(rootObject map[string]any, jsonPath string) (map[string]any, string, error) {
	pathSegments, err := config.ParseJSONPath(jsonPath)
	if err != nil {
		return nil, "", fmt.Errorf("invalid JSON path %q: %w", jsonPath, err)
	}

	currentObject := rootObject
	for index, segment := range pathSegments[:len(pathSegments)-1] {
		nextValue, ok := currentObject[segment]
		if !ok {
			return nil, "", fmt.Errorf("does not contain JSON path %q: missing segment %q", jsonPath, segment)
		}

		nextObject, ok := nextValue.(map[string]any)
		if !ok {
			return nil, "", fmt.Errorf("JSON path %q cannot continue through %q because it is not an object", jsonPath, formatJSONPathForError(pathSegments[:index+1]))
		}

		currentObject = nextObject
	}

	targetKey := pathSegments[len(pathSegments)-1]
	targetValue, ok := currentObject[targetKey]
	if !ok {
		return nil, "", fmt.Errorf("does not contain JSON path %q: missing segment %q", jsonPath, targetKey)
	}
	if _, ok := targetValue.(string); !ok {
		return nil, "", fmt.Errorf("JSON path %q must resolve to a string", jsonPath)
	}

	return currentObject, targetKey, nil
}

func parseConnectionStringsObject(contents []byte) (map[string]any, map[string]any, error) {
	rootObject, err := parseRootObject(contents)
	if err != nil {
		return nil, nil, err
	}

	connectionStringsValue, ok := rootObject["ConnectionStrings"]
	if !ok {
		return nil, nil, fmt.Errorf("must contain a ConnectionStrings object")
	}

	connectionStringsObject, ok := connectionStringsValue.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("ConnectionStrings must be an object")
	}

	return rootObject, connectionStringsObject, nil
}

func parseRootObject(contents []byte) (map[string]any, error) {
	decodedDocument, err := parseJSONDocument(contents)
	if err != nil {
		return nil, fmt.Errorf("contains invalid JSON: %w", err)
	}

	rootObject, ok := decodedDocument.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("must contain a JSON object at the root")
	}

	return rootObject, nil
}

func parseJSONDocument(contents []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()

	var decodedDocument any
	if err := decoder.Decode(&decodedDocument); err != nil {
		return nil, err
	}

	var extraValue any
	if err := decoder.Decode(&extraValue); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values are not allowed")
		}

		return nil, err
	}

	return decodedDocument, nil
}

func buildStringTargetNodes(currentObject map[string]any, parentSegments []string) []StringTargetNode {
	propertyNames := make([]string, 0, len(currentObject))
	for propertyName := range currentObject {
		propertyNames = append(propertyNames, propertyName)
	}
	sort.Strings(propertyNames)

	objectNodes := make([]StringTargetNode, 0)
	stringNodes := make([]StringTargetNode, 0)
	for _, propertyName := range propertyNames {
		pathSegments := appendPathSegment(parentSegments, propertyName)
		jsonPath, ok := formatJSONPath(pathSegments)
		if !ok {
			continue
		}

		switch propertyValue := currentObject[propertyName].(type) {
		case string:
			stringNodes = append(stringNodes, StringTargetNode{
				Name:       propertyName,
				JSONPath:   jsonPath,
				Selectable: true,
			})
		case map[string]any:
			children := buildStringTargetNodes(propertyValue, pathSegments)
			if len(children) == 0 {
				continue
			}

			objectNodes = append(objectNodes, StringTargetNode{
				Name:     propertyName,
				JSONPath: jsonPath,
				Children: children,
			})
		}
	}

	return append(objectNodes, stringNodes...)
}

func appendPathSegment(pathSegments []string, segment string) []string {
	nextPathSegments := make([]string, len(pathSegments)+1)
	copy(nextPathSegments, pathSegments)
	nextPathSegments[len(pathSegments)] = segment
	return nextPathSegments
}

func formatJSONPath(pathSegments []string) (string, bool) {
	jsonPath, err := config.FormatJSONPath(pathSegments)
	if err != nil {
		return "", false
	}

	return jsonPath, true
}

func formatJSONPathForError(pathSegments []string) string {
	jsonPath, ok := formatJSONPath(pathSegments)
	if ok {
		return jsonPath
	}

	return strings.Join(pathSegments, ".")
}
