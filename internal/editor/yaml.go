package editor

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/jeppeklh/switchlet/internal/config"
	"gopkg.in/yaml.v3"
)

// ValidateYAMLTarget verifies that a YAML file contains a string scalar at the
// configured YAML path.
func ValidateYAMLTarget(targetPath string, yamlPath string) error {
	if yamlPath == "" {
		return fmt.Errorf("YAML path must be set")
	}

	contents, _, err := readTargetFile(targetPath)
	if err != nil {
		return err
	}

	if _, err := parseYAMLStringTarget(contents, yamlPath); err != nil {
		return fmt.Errorf("validate YAML target file %q: %w", targetPath, err)
	}

	return nil
}

// InspectYAMLStringTargets returns a hierarchical view of selectable existing
// string-valued YAML paths inside targetPath.
func InspectYAMLStringTargets(targetPath string) ([]YAMLStringTargetNode, error) {
	contents, _, err := readTargetFile(targetPath)
	if err != nil {
		return nil, err
	}

	nodes, err := inspectYAMLStringTargetsContents(contents)
	if err != nil {
		return nil, fmt.Errorf("inspect target file %q: %w", targetPath, err)
	}

	return nodes, nil
}

func replaceYAMLTargetValues(contents []byte, changes []TargetChange) ([]byte, error) {
	document, err := parseYAMLDocument(contents)
	if err != nil {
		return nil, targetError(changes[0].Target, err)
	}

	updates := make([]yamlStringValueUpdate, 0, len(changes))
	for _, change := range changes {
		targetNode, err := findYAMLStringTarget(document, change.Target.YAMLPath)
		if err != nil {
			return nil, targetError(change.Target, err)
		}

		updates = append(updates, yamlStringValueUpdate{
			targetNode:       targetNode,
			replacementValue: change.Value,
		})
	}

	for _, update := range updates {
		update.targetNode.Value = update.replacementValue
		update.targetNode.Tag = "!!str"
	}

	return serializeYAMLDocument(document)
}

func readYAMLStringTargetValue(contents []byte, yamlPath string) (string, error) {
	targetNode, err := parseYAMLStringTarget(contents, yamlPath)
	if err != nil {
		return "", err
	}

	return targetNode.Value, nil
}

type yamlStringValueUpdate struct {
	targetNode       *yaml.Node
	replacementValue string
}

func parseYAMLStringTarget(contents []byte, yamlPath string) (*yaml.Node, error) {
	document, err := parseYAMLDocument(contents)
	if err != nil {
		return nil, err
	}

	return findYAMLStringTarget(document, yamlPath)
}

func parseYAMLDocument(contents []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(contents))

	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("must contain a YAML mapping at the root")
		}

		return nil, fmt.Errorf("contains invalid YAML: %w", err)
	}

	var extraDocument yaml.Node
	switch err := decoder.Decode(&extraDocument); err {
	case io.EOF:
	case nil:
		return nil, fmt.Errorf("multiple YAML documents are not supported")
	default:
		return nil, fmt.Errorf("contains invalid YAML: %w", err)
	}

	return &document, nil
}

func findYAMLStringTarget(document *yaml.Node, yamlPath string) (*yaml.Node, error) {
	pathSegments, err := config.ParseYAMLPath(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("invalid YAML path %q: %w", yamlPath, err)
	}

	currentNode, err := yamlRootMapping(document, yamlPath)
	if err != nil {
		return nil, err
	}

	for index, segment := range pathSegments {
		if currentNode.Kind != yaml.MappingNode {
			traversedPath := strings.Join(pathSegments[:index], ".")
			return nil, fmt.Errorf("YAML path %q cannot continue through %q because it is not a mapping", yamlPath, traversedPath)
		}

		nextNode, err := findYAMLMappingValue(currentNode, yamlPath, pathSegments[:index], segment)
		if err != nil {
			return nil, err
		}

		if err := validateYAMLPathNode(nextNode, yamlPath); err != nil {
			return nil, err
		}

		if index == len(pathSegments)-1 {
			if !isYAMLStringScalar(nextNode) {
				return nil, fmt.Errorf("YAML path %q must resolve to a scalar string", yamlPath)
			}

			return nextNode, nil
		}

		if nextNode.Kind != yaml.MappingNode {
			traversedPath := strings.Join(pathSegments[:index+1], ".")
			return nil, fmt.Errorf("YAML path %q cannot continue through %q because it is not a mapping", yamlPath, traversedPath)
		}

		currentNode = nextNode
	}

	return nil, fmt.Errorf("YAML path %q must contain at least one segment", yamlPath)
}

func yamlRootMapping(document *yaml.Node, yamlPath string) (*yaml.Node, error) {
	if document.Kind != yaml.DocumentNode || len(document.Content) == 0 || document.Content[0].Kind == 0 {
		return nil, fmt.Errorf("must contain a YAML mapping at the root")
	}

	rootNode := document.Content[0]
	if err := validateYAMLPathNode(rootNode, yamlPath); err != nil {
		return nil, err
	}
	if rootNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("must contain a YAML mapping at the root")
	}

	return rootNode, nil
}

func findYAMLMappingValue(mappingNode *yaml.Node, yamlPath string, parentSegments []string, segment string) (*yaml.Node, error) {
	if err := validateYAMLMappingKeys(mappingNode, yamlPath, parentSegments); err != nil {
		return nil, err
	}

	var foundNode *yaml.Node
	for index := 0; index < len(mappingNode.Content); index += 2 {
		keyNode := mappingNode.Content[index]
		valueNode := mappingNode.Content[index+1]

		if isYAMLMergeKey(keyNode) {
			continue
		}
		if !isYAMLStringMappingKey(keyNode) {
			continue
		}
		if keyNode.Value == segment {
			foundNode = valueNode
			break
		}
	}

	if foundNode != nil {
		return foundNode, nil
	}
	if yamlMappingHasMergeKey(mappingNode) {
		return nil, fmt.Errorf("YAML path %q depends on unsupported YAML merge key at %s", yamlPath, yamlPathContext(parentSegments))
	}

	return nil, fmt.Errorf("does not contain YAML path %q: missing segment %q", yamlPath, segment)
}

func validateYAMLPathNode(node *yaml.Node, yamlPath string) error {
	if node.Kind == yaml.AliasNode {
		return fmt.Errorf("YAML path %q cannot use aliases", yamlPath)
	}
	if node.Anchor != "" {
		return fmt.Errorf("YAML path %q cannot use anchored nodes", yamlPath)
	}

	return nil
}

func validateYAMLMappingKeys(mappingNode *yaml.Node, yamlPath string, parentSegments []string) error {
	seenKeys := make(map[string]struct{})
	for index := 0; index < len(mappingNode.Content); index += 2 {
		keyNode := mappingNode.Content[index]
		if keyNode.Kind == yaml.AliasNode {
			return fmt.Errorf("YAML path %q cannot use aliases", yamlPath)
		}
		if keyNode.Anchor != "" {
			return fmt.Errorf("YAML path %q cannot use anchored nodes", yamlPath)
		}
		if isYAMLMergeKey(keyNode) || !isYAMLStringMappingKey(keyNode) {
			continue
		}

		if _, exists := seenKeys[keyNode.Value]; exists {
			return fmt.Errorf("YAML path %q is ambiguous because mapping at %s contains duplicate key %q", yamlPath, yamlPathContext(parentSegments), keyNode.Value)
		}
		seenKeys[keyNode.Value] = struct{}{}
	}

	return nil
}

func serializeYAMLDocument(document *yaml.Node) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		_ = encoder.Close()
		return nil, fmt.Errorf("serialize updated YAML: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("serialize updated YAML: %w", err)
	}

	return buffer.Bytes(), nil
}

func inspectYAMLStringTargetsContents(contents []byte) ([]YAMLStringTargetNode, error) {
	document, err := parseYAMLDocument(contents)
	if err != nil {
		return nil, err
	}

	rootNode, err := yamlRootMapping(document, "")
	if err != nil {
		return nil, err
	}

	nodes := buildYAMLStringTargetNodes(rootNode, nil)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("does not contain any existing string-valued YAML paths")
	}

	return nodes, nil
}

func buildYAMLStringTargetNodes(mappingNode *yaml.Node, parentSegments []string) []YAMLStringTargetNode {
	if mappingNode.Kind != yaml.MappingNode || mappingNode.Anchor != "" || hasDuplicateYAMLMappingKeys(mappingNode) {
		return nil
	}

	nodes := make([]YAMLStringTargetNode, 0)
	for index := 0; index < len(mappingNode.Content); index += 2 {
		keyNode := mappingNode.Content[index]
		valueNode := mappingNode.Content[index+1]
		if !isInspectableYAMLMappingKey(keyNode) || valueNode.Kind == yaml.AliasNode || valueNode.Anchor != "" {
			continue
		}

		pathSegments := appendPathSegment(parentSegments, keyNode.Value)
		yamlPath := strings.Join(pathSegments, ".")
		switch {
		case isYAMLStringScalar(valueNode):
			nodes = append(nodes, YAMLStringTargetNode{
				Name:       keyNode.Value,
				YAMLPath:   yamlPath,
				Selectable: true,
			})
		case valueNode.Kind == yaml.MappingNode:
			children := buildYAMLStringTargetNodes(valueNode, pathSegments)
			if len(children) == 0 {
				continue
			}

			nodes = append(nodes, YAMLStringTargetNode{
				Name:     keyNode.Value,
				YAMLPath: yamlPath,
				Children: children,
			})
		}
	}

	return nodes
}

func hasDuplicateYAMLMappingKeys(mappingNode *yaml.Node) bool {
	seenKeys := make(map[string]struct{})
	for index := 0; index < len(mappingNode.Content); index += 2 {
		keyNode := mappingNode.Content[index]
		if isYAMLMergeKey(keyNode) || !isYAMLStringMappingKey(keyNode) {
			continue
		}

		if _, exists := seenKeys[keyNode.Value]; exists {
			return true
		}
		seenKeys[keyNode.Value] = struct{}{}
	}

	return false
}

func yamlMappingHasMergeKey(mappingNode *yaml.Node) bool {
	for index := 0; index < len(mappingNode.Content); index += 2 {
		if isYAMLMergeKey(mappingNode.Content[index]) {
			return true
		}
	}

	return false
}

func isInspectableYAMLMappingKey(node *yaml.Node) bool {
	return isYAMLStringMappingKey(node) && node.Value != "" && strings.TrimSpace(node.Value) == node.Value && !strings.Contains(node.Value, ".")
}

func isYAMLStringMappingKey(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.ShortTag() == "!!str"
}

func isYAMLStringScalar(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.ShortTag() == "!!str"
}

func isYAMLMergeKey(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && (node.ShortTag() == "!!merge" || node.Value == "<<")
}

func yamlPathContext(pathSegments []string) string {
	if len(pathSegments) == 0 {
		return "root"
	}

	return fmt.Sprintf("%q", strings.Join(pathSegments, "."))
}
