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

	targetObject[targetKey] = replacementValue

	return serializeJSONRoot(rootObject)
}

func readJSONStringTargetValue(contents []byte, jsonPath string) (string, error) {
	_, targetObject, targetKey, err := parseStringTarget(contents, jsonPath)
	if err != nil {
		return "", err
	}

	targetValue, ok := targetObject[targetKey].(string)
	if !ok {
		return "", fmt.Errorf("JSON path %q must resolve to a string", jsonPath)
	}

	return targetValue, nil
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
			targetObject:     targetObject,
			targetKey:        targetKey,
			replacementValue: change.Value,
		})
	}

	for _, update := range updates {
		update.targetObject[update.targetKey] = update.replacementValue
	}

	return serializeJSONRoot(rootObject)
}

type jsonStringValueUpdate struct {
	targetObject     map[string]any
	targetKey        string
	replacementValue string
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
			return nil, "", fmt.Errorf("JSON path %q cannot continue through %q because it is not an object", jsonPath, strings.Join(pathSegments[:index+1], "."))
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
		jsonPath := strings.Join(pathSegments, ".")

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
