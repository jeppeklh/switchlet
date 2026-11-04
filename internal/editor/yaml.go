package editor

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

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
		targetNode, err := findYAMLScalarTarget(document, change.Target.YAMLPath, change.Target.ValueType)
		if err != nil {
			return nil, targetError(change.Target, err)
		}

		updates = append(updates, yamlStringValueUpdate{
			targetNode:       targetNode,
			valueType:        change.Target.ValueType,
			replacementValue: change.Value,
		})
	}

	for _, update := range updates {
		update.targetNode.Value = update.replacementValue
		update.targetNode.Tag = scalarYAMLTag(update.valueType)
		if config.EffectiveValueType(update.valueType) != config.ValueTypeString {
			update.targetNode.Style = 0
		}
	}

	if updatedContents, ok := replaceYAMLScalarTokens(contents, document, updates); ok {
		return updatedContents, nil
	}
	return serializeYAMLDocument(document)
}

// replaceYAMLScalarTokens preserves source formatting for plain and quoted
// single-line scalars. Complex scalars retain the existing serializer fallback.
// Re-parsing and comparing the entire tree guards against ambiguous boundaries.
func replaceYAMLScalarTokens(contents []byte, expected *yaml.Node, updates []yamlStringValueUpdate) ([]byte, bool) {
	type replacement struct {
		start, end int
		value      []byte
	}
	replacements := make([]replacement, 0, len(updates))
	for _, update := range updates {
		start, ok := yamlSourceOffset(contents, update.targetNode.Line, update.targetNode.Column)
		if !ok {
			return nil, false
		}
		end, ok := yamlScalarTokenEnd(contents, start)
		if !ok {
			return nil, false
		}

		style := update.targetNode.Style & (yaml.SingleQuotedStyle | yaml.DoubleQuotedStyle)
		if strings.ContainsAny(update.replacementValue, "\r\n\u0085\u2028\u2029,[]{}") {
			style = yaml.DoubleQuotedStyle
		}
		value, err := yaml.Marshal(&yaml.Node{
			Kind: yaml.ScalarNode, Tag: scalarYAMLTag(update.valueType), Style: style, Value: update.replacementValue,
		})
		if err != nil {
			return nil, false
		}
		value = bytes.TrimSuffix(value, []byte("\n"))
		if bytes.ContainsAny(value, "\r\n") {
			return nil, false
		}
		replacements = append(replacements, replacement{start: start, end: end, value: value})
	}

	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	updated := append([]byte(nil), contents...)
	previousStart := len(contents)
	for _, replacement := range replacements {
		if replacement.end > previousStart {
			return nil, false
		}
		replaced := make([]byte, 0, len(updated)-(replacement.end-replacement.start)+len(replacement.value))
		replaced = append(replaced, updated[:replacement.start]...)
		replaced = append(replaced, replacement.value...)
		updated = append(replaced, updated[replacement.end:]...)
		previousStart = replacement.start
	}

	actual, err := parseYAMLDocument(updated)
	if err != nil || !yamlNodesEqual(expected, actual) {
		return nil, false
	}
	return updated, true
}

func yamlSourceOffset(contents []byte, line, column int) (int, bool) {
	if line < 1 || column < 1 {
		return 0, false
	}
	offset := 0
	if bytes.HasPrefix(contents, []byte{0xef, 0xbb, 0xbf}) {
		offset = 3
	}
	for currentLine := 1; currentLine < line; currentLine++ {
		next := bytes.IndexByte(contents[offset:], '\n')
		if next < 0 {
			return 0, false
		}
		offset += next + 1
	}
	for currentColumn := 1; currentColumn < column; currentColumn++ {
		if offset >= len(contents) || contents[offset] == '\n' || contents[offset] == '\r' {
			return 0, false
		}
		_, size := utf8.DecodeRune(contents[offset:])
		offset += size
	}
	return offset, offset < len(contents)
}

func yamlScalarTokenEnd(contents []byte, start int) (int, bool) {
	first := contents[start]
	if first == '!' || first == '&' || first == '*' || first == '|' || first == '>' {
		return 0, false
	}
	quoted := first == '\'' || first == '"'
	for offset := start; offset < len(contents); offset++ {
		character := contents[offset]
		if character == '\r' || character == '\n' {
			if quoted {
				return 0, false
			}
			return yamlPlainScalarEnd(contents, start, offset)
		}
		if quoted {
			if offset == start {
				continue
			}
			if first == '"' && character == '\\' {
				if offset+1 >= len(contents) || contents[offset+1] == '\r' || contents[offset+1] == '\n' {
					return 0, false
				}
				offset++
				continue
			}
			if character == first {
				if first == '\'' && offset+1 < len(contents) && contents[offset+1] == '\'' {
					offset++
					continue
				}
				return offset + 1, true
			}
			continue
		}
		if character == ',' || character == ']' || character == '}' ||
			(character == '#' && (offset == start || contents[offset-1] == ' ' || contents[offset-1] == '\t')) {
			return yamlPlainScalarEnd(contents, start, offset)
		}
	}
	if quoted {
		return 0, false
	}
	return yamlPlainScalarEnd(contents, start, len(contents))
}

func yamlPlainScalarEnd(contents []byte, start, end int) (int, bool) {
	for end > start && (contents[end-1] == ' ' || contents[end-1] == '\t') {
		end--
	}
	return end, end > start
}

// Formatting and source locations are deliberately excluded; scalar types,
// values, mapping order, and anchors/alias names must all remain equivalent.
func yamlNodesEqual(expected, actual *yaml.Node) bool {
	if expected.Kind != actual.Kind || expected.Tag != actual.Tag || expected.Value != actual.Value ||
		expected.Anchor != actual.Anchor || len(expected.Content) != len(actual.Content) {
		return false
	}
	for index := range expected.Content {
		if !yamlNodesEqual(expected.Content[index], actual.Content[index]) {
			return false
		}
	}
	return true
}

func readYAMLTargetValues(contents []byte, targets []config.Target) (map[string]string, error) {
	values := make(map[string]string, len(targets))
	if len(targets) == 0 {
		return values, nil
	}

	document, err := parseYAMLDocument(contents)
	if err != nil {
		return nil, targetError(targets[0], err)
	}

	for _, target := range targets {
		targetNode, err := findYAMLScalarTarget(document, target.YAMLPath, target.ValueType)
		if err != nil {
			return nil, targetError(target, err)
		}
		value, err := yamlScalar(targetNode, target.ValueType)
		if err != nil {
			return nil, targetError(target, err)
		}
		values[target.Name] = value.Text()
	}

	return values, nil
}

type yamlStringValueUpdate struct {
	valueType        config.ValueType
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
	return findYAMLScalarTarget(document, yamlPath, config.ValueTypeString)
}
func findYAMLScalarTarget(document *yaml.Node, yamlPath string, kind config.ValueType) (*yaml.Node, error) {
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
			traversedPath := formatYAMLPathForError(pathSegments[:index])
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
			if _, err := yamlScalar(nextNode, kind); err != nil {
				return nil, fmt.Errorf("YAML path %q must resolve to a scalar %s: %w", yamlPath, config.EffectiveValueType(kind), err)
			}

			return nextNode, nil
		}

		if nextNode.Kind != yaml.MappingNode {
			traversedPath := formatYAMLPathForError(pathSegments[:index+1])
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
	return filterYAMLStringNodes(buildYAMLScalarTargetNodes(mappingNode, parentSegments))
}
func buildYAMLScalarTargetNodes(mappingNode *yaml.Node, parentSegments []string) []YAMLStringTargetNode {
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
		yamlPath, ok := formatYAMLPath(pathSegments)
		if !ok {
			continue
		}
		switch {
		case supportedYAMLScalar(valueNode):
			nodes = append(nodes, YAMLStringTargetNode{
				Name:       keyNode.Value,
				ValueType:  yamlValueType(valueNode),
				YAMLPath:   yamlPath,
				Selectable: true,
			})
		case valueNode.Kind == yaml.MappingNode:
			children := buildYAMLScalarTargetNodes(valueNode, pathSegments)
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
	return isYAMLStringMappingKey(node) && node.Value != "" && strings.TrimSpace(node.Value) == node.Value
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

	return fmt.Sprintf("%q", formatYAMLPathForError(pathSegments))
}

func formatYAMLPath(pathSegments []string) (string, bool) {
	yamlPath, err := config.FormatYAMLPath(pathSegments)
	if err != nil {
		return "", false
	}

	return yamlPath, true
}

func formatYAMLPathForError(pathSegments []string) string {
	yamlPath, ok := formatYAMLPath(pathSegments)
	if ok {
		return yamlPath
	}

	return strings.Join(pathSegments, ".")
}

func scalarYAMLTag(kind config.ValueType) string {
	switch config.EffectiveValueType(kind) {
	case config.ValueTypeBoolean:
		return "!!bool"
	case config.ValueTypeInteger:
		return "!!int"
	case config.ValueTypeNumber:
		return "!!float"
	default:
		return "!!str"
	}
}
func yamlScalar(node *yaml.Node, kind config.ValueType) (config.Scalar, error) {
	expected := scalarYAMLTag(kind)
	if node.Kind != yaml.ScalarNode || (node.Tag != expected && !(config.EffectiveValueType(kind) == config.ValueTypeNumber && node.Tag == "!!int")) {
		return config.Scalar{}, fmt.Errorf("expected %s scalar", config.EffectiveValueType(kind))
	}
	sourceType := yamlValueType(node)
	return sourceScalar(kind, sourceType, node.Value)
}

func yamlValueType(node *yaml.Node) config.ValueType {
	switch node.Tag {
	case "!!str":
		return ""
	case "!!bool":
		return config.ValueTypeBoolean
	case "!!int":
		return config.ValueTypeInteger
	case "!!float":
		return config.ValueTypeNumber
	}
	return "unsupported"
}
func supportedYAMLScalar(node *yaml.Node) bool {
	if node.Kind != yaml.ScalarNode {
		return false
	}
	_, err := yamlScalar(node, yamlValueType(node))
	return err == nil
}
func filterYAMLStringNodes(nodes []YAMLStringTargetNode) []YAMLStringTargetNode {
	if len(nodes) == 0 {
		return nil
	}
	result := make([]YAMLStringTargetNode, 0)
	for _, node := range nodes {
		node.Children = filterYAMLStringNodes(node.Children)
		if node.Selectable && node.ValueType == "" || len(node.Children) > 0 {
			result = append(result, node)
		}
	}
	return result
}

// InspectYAMLScalarTargets includes supported boolean and numeric selectors.
func InspectYAMLScalarTargets(path string) ([]YAMLStringTargetNode, error) {
	contents, _, err := readTargetFile(path)
	if err != nil {
		return nil, err
	}
	return inspectYAMLScalarContents(contents)
}
func inspectYAMLScalarContents(contents []byte) ([]YAMLStringTargetNode, error) {
	document, err := parseYAMLDocument(contents)
	if err != nil {
		return nil, err
	}
	root, err := yamlRootMapping(document, "")
	if err != nil {
		return nil, err
	}
	nodes := buildYAMLScalarTargetNodes(root, nil)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("does not contain any existing scalar YAML paths")
	}
	return nodes, nil
}
