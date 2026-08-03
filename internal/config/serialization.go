package config

import (
	"bytes"
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

type yamlMappingEntry struct {
	key   *yaml.Node
	value *yaml.Node
}

func marshalReplacementConfigPreservingComments(currentContents []byte, projectRoot string, targets []Target, profiles []Profile) ([]byte, error) {
	document, err := parseConfigDocument(currentContents)
	if err != nil {
		return nil, fmt.Errorf("parse existing configuration for comment preservation: %w", err)
	}

	return marshalVersionThreeConfig(projectRoot, targets, profiles, document)
}

func marshalVersionThreeConfig(projectRoot string, targets []Target, profiles []Profile, existingDocument *yaml.Node) ([]byte, error) {
	configuredTargets := fileTargetsFromTargets(projectRoot, targets)
	configuredProfiles := fileProfilesFromProfiles(profiles)
	document := buildVersionThreeConfigDocument(configuredTargets, configuredProfiles, existingDocument)

	contents, err := serializeConfigDocument(document)
	if err != nil {
		return nil, err
	}
	if len(contents) == 0 || contents[len(contents)-1] != '\n' {
		contents = append(contents, '\n')
	}

	return contents, nil
}

func parseConfigDocument(contents []byte) (*yaml.Node, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return nil, err
	}

	return &document, nil
}

func serializeConfigDocument(document *yaml.Node) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		_ = encoder.Close()
		return nil, fmt.Errorf("serialize configuration file: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("serialize configuration file: %w", err)
	}

	return buffer.Bytes(), nil
}

func buildVersionThreeConfigDocument(targets []fileTarget, profiles []fileProfile, existingDocument *yaml.Node) *yaml.Node {
	document := &yaml.Node{Kind: yaml.DocumentNode}
	copyYAMLNodePresentation(document, existingDocument)

	existingRoot := configDocumentRoot(existingDocument)
	root := &yaml.Node{Kind: yaml.MappingNode}
	copyYAMLNodePresentation(root, existingRoot)

	existingFields := yamlMappingEntries(existingRoot)
	appendYAMLMappingEntry(root, "version", buildYAMLIntNode(namedTargetVersion, existingFields["version"].value), existingFields["version"].key)
	appendYAMLMappingEntry(root, "targets", buildTargetSequenceNode(targets, existingFields["targets"].value), existingFields["targets"].key)
	appendYAMLMappingEntry(root, "profiles", buildProfileSequenceNode(profiles, existingFields["profiles"].value), existingFields["profiles"].key)

	document.Content = []*yaml.Node{root}
	return document
}

func buildTargetSequenceNode(targets []fileTarget, existing *yaml.Node) *yaml.Node {
	sequence := &yaml.Node{Kind: yaml.SequenceNode}
	copyYAMLNodePresentation(sequence, existing)

	existingItems := yamlSequenceItemsByField(existing, "name")
	for _, target := range targets {
		sequence.Content = append(sequence.Content, buildTargetNode(target, existingItems[target.Name]))
	}

	return sequence
}

func buildTargetNode(target fileTarget, existing *yaml.Node) *yaml.Node {
	mapping := &yaml.Node{Kind: yaml.MappingNode}
	copyYAMLNodePresentation(mapping, existing)

	existingFields := yamlMappingEntries(existing)
	appendYAMLMappingEntry(mapping, "name", buildYAMLStringNode(target.Name, existingFields["name"].value), existingFields["name"].key)
	appendYAMLMappingEntry(mapping, "file", buildYAMLStringNode(target.File, existingFields["file"].value), existingFields["file"].key)
	appendYAMLMappingEntry(mapping, "type", buildYAMLStringNode(target.Type, existingFields["type"].value), existingFields["type"].key)
	if target.JSONPath != "" {
		appendYAMLMappingEntry(mapping, "jsonPath", buildYAMLStringNode(target.JSONPath, existingFields["jsonPath"].value), existingFields["jsonPath"].key)
	}
	if target.Key != "" {
		appendYAMLMappingEntry(mapping, "key", buildYAMLStringNode(target.Key, existingFields["key"].value), existingFields["key"].key)
	}
	if target.YAMLPath != "" {
		appendYAMLMappingEntry(mapping, "yamlPath", buildYAMLStringNode(target.YAMLPath, existingFields["yamlPath"].value), existingFields["yamlPath"].key)
	}
	if target.TOMLPath != "" {
		appendYAMLMappingEntry(mapping, "tomlPath", buildYAMLStringNode(target.TOMLPath, existingFields["tomlPath"].value), existingFields["tomlPath"].key)
	}

	return mapping
}

func buildProfileSequenceNode(profiles []fileProfile, existing *yaml.Node) *yaml.Node {
	sequence := &yaml.Node{Kind: yaml.SequenceNode}
	copyYAMLNodePresentation(sequence, existing)

	existingItems := yamlSequenceItemsByField(existing, "name")
	for _, profile := range profiles {
		sequence.Content = append(sequence.Content, buildProfileNode(profile, existingItems[profile.Name]))
	}

	return sequence
}

func buildProfileNode(profile fileProfile, existing *yaml.Node) *yaml.Node {
	mapping := &yaml.Node{Kind: yaml.MappingNode}
	copyYAMLNodePresentation(mapping, existing)

	existingFields := yamlMappingEntries(existing)
	appendYAMLMappingEntry(mapping, "name", buildYAMLStringNode(profile.Name, existingFields["name"].value), existingFields["name"].key)
	if profile.Protected {
		appendYAMLMappingEntry(mapping, "protected", buildYAMLBoolNode(true, existingFields["protected"].value), existingFields["protected"].key)
	}
	appendYAMLMappingEntry(mapping, "values", buildProfileValueSequenceNode(profile.Values, existingFields["values"].value), existingFields["values"].key)

	return mapping
}

func buildProfileValueSequenceNode(values []fileProfileValue, existing *yaml.Node) *yaml.Node {
	sequence := &yaml.Node{Kind: yaml.SequenceNode}
	copyYAMLNodePresentation(sequence, existing)

	existingItems := yamlSequenceItemsByField(existing, "target")
	for _, value := range values {
		sequence.Content = append(sequence.Content, buildProfileValueNode(value, existingItems[value.Target]))
	}

	return sequence
}

func buildProfileValueNode(value fileProfileValue, existing *yaml.Node) *yaml.Node {
	mapping := &yaml.Node{Kind: yaml.MappingNode}
	copyYAMLNodePresentation(mapping, existing)

	existingFields := yamlMappingEntries(existing)
	appendYAMLMappingEntry(mapping, "target", buildYAMLStringNode(value.Target, existingFields["target"].value), existingFields["target"].key)
	if value.Value != nil {
		appendYAMLMappingEntry(mapping, "value", buildYAMLStringNode(*value.Value, existingFields["value"].value), existingFields["value"].key)
	}
	if value.ValueFromEnv != nil {
		appendYAMLMappingEntry(mapping, "valueFromEnv", buildYAMLStringNode(*value.ValueFromEnv, existingFields["valueFromEnv"].value), existingFields["valueFromEnv"].key)
	}

	return mapping
}

func appendYAMLMappingEntry(mapping *yaml.Node, key string, value *yaml.Node, existingKey *yaml.Node) {
	mapping.Content = append(mapping.Content, buildYAMLKeyNode(key, existingKey), value)
}

func buildYAMLKeyNode(key string, existing *yaml.Node) *yaml.Node {
	return buildYAMLScalarNode("!!str", key, existing)
}

func buildYAMLStringNode(value string, existing *yaml.Node) *yaml.Node {
	return buildYAMLScalarNode("!!str", value, existing)
}

func buildYAMLIntNode(value int, existing *yaml.Node) *yaml.Node {
	return buildYAMLScalarNode("!!int", strconv.Itoa(value), existing)
}

func buildYAMLBoolNode(value bool, existing *yaml.Node) *yaml.Node {
	return buildYAMLScalarNode("!!bool", strconv.FormatBool(value), existing)
}

func buildYAMLScalarNode(tag string, value string, existing *yaml.Node) *yaml.Node {
	node := &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
	copyYAMLNodePresentation(node, existing)
	return node
}

func configDocumentRoot(document *yaml.Node) *yaml.Node {
	if document == nil {
		return nil
	}
	if document.Kind == yaml.MappingNode {
		return document
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) == 0 {
		return nil
	}
	root := document.Content[0]
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}

	return root
}

func yamlMappingEntries(mapping *yaml.Node) map[string]yamlMappingEntry {
	entries := make(map[string]yamlMappingEntry)
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return entries
	}

	for index := 0; index+1 < len(mapping.Content); index += 2 {
		keyNode := mapping.Content[index]
		valueNode := mapping.Content[index+1]
		if keyNode == nil || keyNode.Kind != yaml.ScalarNode {
			continue
		}

		entries[keyNode.Value] = yamlMappingEntry{key: keyNode, value: valueNode}
	}

	return entries
}

func yamlSequenceItemsByField(sequence *yaml.Node, fieldName string) map[string]*yaml.Node {
	items := make(map[string]*yaml.Node)
	if sequence == nil || sequence.Kind != yaml.SequenceNode {
		return items
	}

	for _, item := range sequence.Content {
		entry, ok := yamlMappingEntries(item)[fieldName]
		if !ok || entry.value == nil || entry.value.Kind != yaml.ScalarNode {
			continue
		}
		if _, exists := items[entry.value.Value]; exists {
			continue
		}

		items[entry.value.Value] = item
	}

	return items
}

func copyYAMLNodePresentation(destination *yaml.Node, source *yaml.Node) {
	if destination == nil || source == nil {
		return
	}

	destination.Style = source.Style
	destination.HeadComment = source.HeadComment
	destination.LineComment = source.LineComment
	destination.FootComment = source.FootComment
}
