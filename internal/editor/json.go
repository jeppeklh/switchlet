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
		targetObject, targetKey, err := findScalarTarget(rootObject, change.Target.JSONPath, change.Target.ValueType)
		if err != nil {
			return nil, targetError(change.Target, err)
		}

		updates = append(updates, jsonStringValueUpdate{
			jsonPath:         change.Target.JSONPath,
			valueType:        change.Target.ValueType,
			targetObject:     targetObject,
			targetKey:        targetKey,
			replacementValue: change.Value,
		})
	}

	if updatedContents, ok := replaceJSONStringTokens(contents, updates); ok {
		return updatedContents, nil
	}

	for _, update := range updates {
		if config.EffectiveValueType(update.valueType) == config.ValueTypeString {
			update.targetObject[update.targetKey] = update.replacementValue
		} else {
			raw, err := encodedScalar(update.replacementValue, update.valueType)
			if err != nil {
				return nil, fmt.Errorf("encode JSON scalar: %w", err)
			}
			update.targetObject[update.targetKey] = json.RawMessage(raw)
		}
	}

	return serializeJSONRoot(rootObject)
}

type jsonStringValueUpdate struct {
	valueType        config.ValueType
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
		if err := validateUniqueJSONTargetPath(contents, target.JSONPath); err != nil {
			return nil, targetError(target, err)
		}
		targetObject, targetKey, err := findScalarTarget(rootObject, target.JSONPath, target.ValueType)
		if err != nil {
			return nil, targetError(target, err)
		}

		targetValue, err := nativeScalar(targetObject[targetKey], target.ValueType)
		if err != nil {
			return nil, targetError(target, err)
		}
		values[target.Name] = targetValue.Text()
	}

	return values, nil
}

// Validate only keys along the configured selector. Decoding into a map alone
// silently chooses the last duplicate, including duplicate ancestor objects.
func validateUniqueJSONTargetPath(contents []byte, path string) error {
	segments, err := config.ParseJSONPath(path)
	if err != nil {
		return err
	}
	for _, segment := range segments {
		decoder := json.NewDecoder(bytes.NewReader(contents))
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		if token != json.Delim('{') {
			return nil
		} // The normal target lookup reports type errors.
		var matched json.RawMessage
		found := false
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			var value json.RawMessage
			if err := decoder.Decode(&value); err != nil {
				return err
			}
			if key != segment {
				continue
			}
			if found {
				return fmt.Errorf("JSON path is ambiguous because an object key is repeated")
			}
			found, matched = true, value
		}
		if !found {
			return nil
		} // The normal target lookup reports missing keys.
		contents = matched
	}
	return nil
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

		replacementValue, err := encodedScalar(update.replacementValue, update.valueType)
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
	case json.Number, bool:
		if recordValue {
			raw := fmt.Sprint(value)
			end := int(decoder.InputOffset())
			rangesByPath[selectorSegmentsKey(pathSegments)] = jsonStringValueRange{start: end - len(raw), end: end}
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
	return findScalarTarget(rootObject, jsonPath, config.ValueTypeString)
}
func findScalarTarget(rootObject map[string]any, jsonPath string, kind config.ValueType) (map[string]any, string, error) {
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
	if _, err := nativeScalar(targetValue, kind); err != nil {
		return nil, "", fmt.Errorf("JSON path %q must resolve to a %s: %w", jsonPath, config.EffectiveValueType(kind), err)
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
	return filterJSONStringNodes(buildScalarTargetNodes(currentObject, parentSegments))
}
func buildScalarTargetNodes(currentObject map[string]any, parentSegments []string) []StringTargetNode {
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
		case string, bool, json.Number:
			if _, err := nativeScalar(propertyValue, nativeValueType(propertyValue)); err != nil {
				continue
			}
			stringNodes = append(stringNodes, StringTargetNode{
				Name:       propertyName,
				ValueType:  nativeValueType(propertyValue),
				JSONPath:   jsonPath,
				Selectable: true,
			})
		case map[string]any:
			children := buildScalarTargetNodes(propertyValue, pathSegments)
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

func filterJSONStringNodes(nodes []StringTargetNode) []StringTargetNode {
	if len(nodes) == 0 {
		return nil
	}
	result := make([]StringTargetNode, 0)
	for _, node := range nodes {
		node.Children = filterJSONStringNodes(node.Children)
		if node.Selectable && node.ValueType == "" || len(node.Children) > 0 {
			result = append(result, node)
		}
	}
	return result
}

// InspectJSONScalarTargets includes supported boolean and numeric selectors.
func InspectJSONScalarTargets(path string) ([]StringTargetNode, error) {
	contents, _, err := readTargetFile(path)
	if err != nil {
		return nil, err
	}
	return inspectJSONScalarContents(contents)
}
func inspectJSONScalarContents(contents []byte) ([]StringTargetNode, error) {
	root, err := parseRootObject(contents)
	if err != nil {
		return nil, err
	}
	nodes := buildScalarTargetNodes(root, nil)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("does not contain any existing scalar JSON paths")
	}
	return nodes, nil
}
