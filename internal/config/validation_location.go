package config

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type validationLocationError struct {
	configPath string
	line       int
	column     int
	err        error
}

func (err validationLocationError) Error() string {
	return fmt.Sprintf("%s:%d:%d: %v", err.configPath, err.line, err.column, err.err)
}

func (err validationLocationError) Unwrap() error {
	return err.err
}

type yamlPathSegment struct {
	name  string
	index int
}

func validationErrorWithLocation(configPath string, root *yaml.Node, validationErr error) error {
	if validationErr == nil {
		return nil
	}

	node, ok := validationErrorNode(root, validationErr.Error())
	if !ok || node.Line <= 0 || node.Column <= 0 {
		return validationErr
	}

	return validationLocationError{
		configPath: configPath,
		line:       node.Line,
		column:     node.Column,
		err:        validationErr,
	}
}

func validationErrorNode(root *yaml.Node, message string) (*yaml.Node, bool) {
	if fieldPath, ok := leadingValidationFieldPath(message); ok {
		return yamlNodeForPath(root, parseYAMLPathSegments(fieldPath))
	}

	return profileValidationNode(root, message)
}

func leadingValidationFieldPath(message string) (string, bool) {
	switch {
	case strings.HasPrefix(message, "target.") || strings.HasPrefix(message, "targets[") || strings.HasPrefix(message, "profiles["):
		fieldPath, _, _ := strings.Cut(message, " ")
		return fieldPath, fieldPath != ""
	case strings.HasPrefix(message, "unsupported version"):
		return "version", true
	case strings.HasPrefix(message, "at least one target"):
		return "targets", true
	case strings.HasPrefix(message, "at least one profile"):
		return "profiles", true
	default:
		return "", false
	}
}

func parseYAMLPathSegments(fieldPath string) []yamlPathSegment {
	rawSegments := strings.Split(fieldPath, ".")
	segments := make([]yamlPathSegment, 0, len(rawSegments))
	for _, rawSegment := range rawSegments {
		segment := yamlPathSegment{name: rawSegment, index: -1}
		if name, rawIndex, ok := strings.Cut(rawSegment, "["); ok && strings.HasSuffix(rawIndex, "]") {
			indexText := strings.TrimSuffix(rawIndex, "]")
			index, err := strconv.Atoi(indexText)
			if err == nil {
				segment.name = name
				segment.index = index
			}
		}
		segments = append(segments, segment)
	}

	return segments
}

func yamlNodeForPath(root *yaml.Node, segments []yamlPathSegment) (*yaml.Node, bool) {
	node := yamlDocumentNode(root)
	if node == nil {
		return nil, false
	}
	lastLocated := node

	for _, segment := range segments {
		valueNode, ok := yamlMappingValue(node, segment.name)
		if !ok {
			return lastLocated, lastLocated != nil
		}
		node = valueNode
		lastLocated = node

		if segment.index < 0 {
			continue
		}
		if node.Kind != yaml.SequenceNode || segment.index >= len(node.Content) {
			return lastLocated, true
		}
		node = node.Content[segment.index]
		lastLocated = node
	}

	return node, true
}

func profileValidationNode(root *yaml.Node, message string) (*yaml.Node, bool) {
	profileName, rest, ok := profileValidationMessage(message)
	if !ok {
		return nil, false
	}

	profileNode, ok := yamlProfileNode(root, profileName)
	if !ok {
		return nil, false
	}

	trimmedRest := strings.TrimSpace(rest)
	switch {
	case strings.HasPrefix(trimmedRest, "values["):
		fieldPath, _, _ := strings.Cut(trimmedRest, " ")
		return yamlNodeForPathFromNode(profileNode, parseYAMLPathSegments(fieldPath))
	case strings.HasPrefix(trimmedRest, "valueFromEnv"):
		return yamlNodeForPathFromNode(profileNode, []yamlPathSegment{{name: "valueFromEnv", index: -1}})
	case strings.HasPrefix(trimmedRest, "value for target "):
		return profileValueValidationNode(profileNode, trimmedRest)
	default:
		return profileNode, true
	}
}

func profileValidationMessage(message string) (string, string, bool) {
	if !strings.HasPrefix(message, "profile ") {
		return "", "", false
	}

	remainder := strings.TrimPrefix(message, "profile ")
	if !strings.HasPrefix(remainder, "\"") {
		return "", "", false
	}
	remainder = strings.TrimPrefix(remainder, "\"")
	profileName, rest, found := strings.Cut(remainder, "\"")
	if !found || profileName == "" {
		return "", "", false
	}

	return profileName, rest, true
}

func yamlProfileNode(root *yaml.Node, profileName string) (*yaml.Node, bool) {
	profilesNode, ok := yamlNodeForPath(root, []yamlPathSegment{{name: "profiles", index: -1}})
	if !ok || profilesNode.Kind != yaml.SequenceNode {
		return nil, false
	}

	for _, profileNode := range profilesNode.Content {
		nameNode, ok := yamlMappingValue(profileNode, "name")
		if ok && nameNode.Value == profileName {
			return profileNode, true
		}
	}

	return nil, false
}

func profileValueValidationNode(profileNode *yaml.Node, message string) (*yaml.Node, bool) {
	targetName, rest, ok := quotedValueAfterPrefix(message, "value for target ")
	if !ok {
		return profileNode, true
	}

	valueNode, ok := yamlProfileValueNode(profileNode, targetName)
	if !ok {
		return profileNode, true
	}
	if strings.Contains(rest, "valueFromEnv") {
		return yamlNodeForPathFromNode(valueNode, []yamlPathSegment{{name: "valueFromEnv", index: -1}})
	}

	return valueNode, true
}

func quotedValueAfterPrefix(message string, prefix string) (string, string, bool) {
	remainder := strings.TrimPrefix(message, prefix)
	if remainder == message || !strings.HasPrefix(remainder, "\"") {
		return "", "", false
	}
	remainder = strings.TrimPrefix(remainder, "\"")
	value, rest, found := strings.Cut(remainder, "\"")
	if !found || value == "" {
		return "", "", false
	}

	return value, rest, true
}

func yamlProfileValueNode(profileNode *yaml.Node, targetName string) (*yaml.Node, bool) {
	valuesNode, ok := yamlMappingValue(profileNode, "values")
	if !ok || valuesNode.Kind != yaml.SequenceNode {
		return nil, false
	}

	for _, valueNode := range valuesNode.Content {
		targetNode, ok := yamlMappingValue(valueNode, "target")
		if ok && targetNode.Value == targetName {
			return valueNode, true
		}
	}

	return nil, false
}

func yamlNodeForPathFromNode(root *yaml.Node, segments []yamlPathSegment) (*yaml.Node, bool) {
	lastLocated := root
	node := root
	for _, segment := range segments {
		valueNode, ok := yamlMappingValue(node, segment.name)
		if !ok {
			return lastLocated, lastLocated != nil
		}
		node = valueNode
		lastLocated = node

		if segment.index < 0 {
			continue
		}
		if node.Kind != yaml.SequenceNode || segment.index >= len(node.Content) {
			return lastLocated, true
		}
		node = node.Content[segment.index]
		lastLocated = node
	}

	return node, true
}

func yamlDocumentNode(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		return root.Content[0]
	}

	return root
}

func yamlMappingValue(node *yaml.Node, name string) (*yaml.Node, bool) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, false
	}

	for index := 0; index+1 < len(node.Content); index += 2 {
		keyNode := node.Content[index]
		if keyNode.Value == name {
			return node.Content[index+1], true
		}
	}

	return nil, false
}
