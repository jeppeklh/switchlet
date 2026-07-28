package editor

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jeppeklh/switchlet/internal/config"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

// ValidateTOMLTarget verifies that a TOML file contains a string at the
// configured TOML path.
func ValidateTOMLTarget(targetPath string, tomlPath string) error {
	if tomlPath == "" {
		return fmt.Errorf("TOML path must be set")
	}

	contents, _, err := readTargetFile(targetPath)
	if err != nil {
		return err
	}

	if _, err := parseTOMLStringTarget(contents, tomlPath); err != nil {
		return fmt.Errorf("validate TOML target file %q: %w", targetPath, err)
	}

	return nil
}

func replaceTOMLTargetValues(contents []byte, changes []TargetChange) ([]byte, error) {
	document, err := parseTOMLDocument(contents)
	if err != nil {
		return nil, targetError(changes[0].Target, err)
	}

	updates := make([]tomlStringValueUpdate, 0, len(changes))
	for _, change := range changes {
		targetValue, err := document.findStringTarget(change.Target.TOMLPath)
		if err != nil {
			return nil, targetError(change.Target, err)
		}

		replacementValue, err := quoteTOMLString(change.Value)
		if err != nil {
			return nil, targetError(change.Target, err)
		}

		updates = append(updates, tomlStringValueUpdate{
			raw:              targetValue.raw,
			replacementValue: replacementValue,
		})
	}

	return replaceTOMLValueRanges(contents, updates)
}

func readTOMLStringTargetValue(contents []byte, tomlPath string) (string, error) {
	if _, err := parseTOMLStringTarget(contents, tomlPath); err != nil {
		return "", err
	}

	pathSegments, err := config.ParseTOMLPath(tomlPath)
	if err != nil {
		return "", fmt.Errorf("invalid TOML path %q: %w", tomlPath, err)
	}

	var decodedDocument map[string]any
	if err := toml.Unmarshal(contents, &decodedDocument); err != nil {
		return "", fmt.Errorf("contains invalid TOML: %w", err)
	}

	currentValue := any(decodedDocument)
	for index, segment := range pathSegments {
		currentMap, ok := currentValue.(map[string]any)
		if !ok {
			return "", fmt.Errorf("TOML path %q cannot continue through %q because it is not a table", tomlPath, strings.Join(pathSegments[:index], "."))
		}

		nextValue, ok := currentMap[segment]
		if !ok {
			return "", fmt.Errorf("does not contain TOML path %q: missing segment %q", tomlPath, segment)
		}
		currentValue = nextValue
	}

	stringValue, ok := currentValue.(string)
	if !ok {
		return "", fmt.Errorf("TOML path %q must resolve to a string", tomlPath)
	}

	return stringValue, nil
}

type tomlStringValueUpdate struct {
	raw              unstable.Range
	replacementValue []byte
}

type tomlDocument struct {
	tables         map[string]struct{}
	arrayTables    map[string]struct{}
	valuesByPath   map[string][]tomlValueRef
	stringPaths    [][]string
	seenStringPath map[string]struct{}
}

type tomlValueRef struct {
	path         []string
	kind         unstable.Kind
	raw          unstable.Range
	inArrayTable bool
}

func parseTOMLStringTarget(contents []byte, tomlPath string) (tomlValueRef, error) {
	document, err := parseTOMLDocument(contents)
	if err != nil {
		return tomlValueRef{}, err
	}

	return document.findStringTarget(tomlPath)
}

func parseTOMLDocument(contents []byte) (*tomlDocument, error) {
	if err := validateTOMLDocumentSemantics(contents); err != nil {
		return nil, err
	}

	document := newTOMLDocument()
	parser := unstable.Parser{}
	parser.Reset(contents)

	var currentTable []string
	currentTableIsArray := false
	for parser.NextExpression() {
		expression := parser.Expression()
		switch expression.Kind {
		case unstable.Table:
			currentTable = tomlNodeKeySegments(expression)
			currentTableIsArray = false
			document.addTable(currentTable)
		case unstable.ArrayTable:
			currentTable = tomlNodeKeySegments(expression)
			currentTableIsArray = true
			document.addArrayTable(currentTable)
		case unstable.KeyValue:
			pathSegments := appendPathSegments(currentTable, tomlNodeKeySegments(expression)...)
			document.addValue(pathSegments, expression.Value(), currentTableIsArray)
		}
	}
	if err := parser.Error(); err != nil {
		return nil, fmt.Errorf("contains invalid TOML: %w", err)
	}

	return document, nil
}

func validateTOMLDocumentSemantics(contents []byte) error {
	var decodedDocument map[string]any
	if err := toml.Unmarshal(contents, &decodedDocument); err != nil {
		return fmt.Errorf("contains invalid TOML: %w", err)
	}

	return nil
}

func newTOMLDocument() *tomlDocument {
	return &tomlDocument{
		tables:         make(map[string]struct{}),
		arrayTables:    make(map[string]struct{}),
		valuesByPath:   make(map[string][]tomlValueRef),
		seenStringPath: make(map[string]struct{}),
	}
}

func (document *tomlDocument) addTable(pathSegments []string) {
	document.addTablePrefixes(pathSegments, len(pathSegments))
}

func (document *tomlDocument) addArrayTable(pathSegments []string) {
	document.addTablePrefixes(pathSegments, len(pathSegments)-1)
	document.arrayTables[tomlPathKey(pathSegments)] = struct{}{}
}

func (document *tomlDocument) addValue(pathSegments []string, valueNode *unstable.Node, inArrayTable bool) {
	document.addTablePrefixes(pathSegments, len(pathSegments)-1)

	pathKey := tomlPathKey(pathSegments)
	pathCopy := clonePathSegments(pathSegments)
	document.valuesByPath[pathKey] = append(document.valuesByPath[pathKey], tomlValueRef{
		path:         pathCopy,
		kind:         valueNode.Kind,
		raw:          valueNode.Raw,
		inArrayTable: inArrayTable,
	})

	if inArrayTable || valueNode.Kind != unstable.String || !isInspectableTOMLPath(pathSegments) {
		return
	}
	if _, exists := document.seenStringPath[pathKey]; exists {
		return
	}

	document.stringPaths = append(document.stringPaths, pathCopy)
	document.seenStringPath[pathKey] = struct{}{}
}

func (document *tomlDocument) addTablePrefixes(pathSegments []string, maxLength int) {
	if maxLength > len(pathSegments) {
		maxLength = len(pathSegments)
	}
	for length := 1; length <= maxLength; length++ {
		document.tables[tomlPathKey(pathSegments[:length])] = struct{}{}
	}
}

func (document *tomlDocument) findStringTarget(tomlPath string) (tomlValueRef, error) {
	pathSegments, err := config.ParseTOMLPath(tomlPath)
	if err != nil {
		return tomlValueRef{}, fmt.Errorf("invalid TOML path %q: %w", tomlPath, err)
	}

	if arrayTablePath, ok := document.arrayTablePrefix(pathSegments); ok {
		return tomlValueRef{}, fmt.Errorf("TOML path %q uses unsupported array table at %q", tomlPath, strings.Join(arrayTablePath, "."))
	}

	valueRefs := document.valuesByPath[tomlPathKey(pathSegments)]
	if len(valueRefs) > 1 {
		return tomlValueRef{}, fmt.Errorf("TOML path %q is ambiguous because it is defined more than once", tomlPath)
	}
	if len(valueRefs) == 1 {
		valueRef := valueRefs[0]
		if valueRef.inArrayTable {
			return tomlValueRef{}, fmt.Errorf("TOML path %q uses unsupported array table", tomlPath)
		}
		if valueRef.kind != unstable.String {
			return tomlValueRef{}, fmt.Errorf("TOML path %q must resolve to a string", tomlPath)
		}

		return valueRef, nil
	}

	if err := document.validateMissingPathContext(tomlPath, pathSegments); err != nil {
		return tomlValueRef{}, err
	}

	return tomlValueRef{}, fmt.Errorf("does not contain TOML path %q: missing segment %q", tomlPath, pathSegments[len(pathSegments)-1])
}

func (document *tomlDocument) validateMissingPathContext(tomlPath string, pathSegments []string) error {
	for index := 0; index < len(pathSegments)-1; index++ {
		prefix := pathSegments[:index+1]
		prefixKey := tomlPathKey(prefix)
		if _, exists := document.arrayTables[prefixKey]; exists {
			return fmt.Errorf("TOML path %q uses unsupported array table at %q", tomlPath, strings.Join(prefix, "."))
		}

		if valueRefs := document.valuesByPath[prefixKey]; len(valueRefs) > 0 {
			return fmt.Errorf("TOML path %q cannot continue through %q because %s", tomlPath, strings.Join(prefix, "."), tomlNonTableReason(valueRefs[0].kind))
		}
		if _, exists := document.tables[prefixKey]; exists {
			continue
		}

		return fmt.Errorf("does not contain TOML path %q: missing segment %q", tomlPath, pathSegments[index])
	}

	return nil
}

func (document *tomlDocument) arrayTablePrefix(pathSegments []string) ([]string, bool) {
	for length := 1; length <= len(pathSegments); length++ {
		prefix := pathSegments[:length]
		if _, exists := document.arrayTables[tomlPathKey(prefix)]; exists {
			return prefix, true
		}
	}

	return nil, false
}

func tomlNonTableReason(kind unstable.Kind) string {
	switch kind {
	case unstable.Array:
		return "arrays are not supported"
	case unstable.InlineTable:
		return "inline tables are not supported"
	default:
		return "it is not a table"
	}
}

func replaceTOMLValueRanges(contents []byte, updates []tomlStringValueUpdate) ([]byte, error) {
	sort.Slice(updates, func(leftIndex int, rightIndex int) bool {
		return updates[leftIndex].raw.Offset > updates[rightIndex].raw.Offset
	})

	updatedContents := append([]byte(nil), contents...)
	for _, update := range updates {
		start := int(update.raw.Offset)
		end := start + int(update.raw.Length)
		if start < 0 || end < start || end > len(updatedContents) {
			return nil, fmt.Errorf("TOML value range is outside the target file")
		}

		replacedContents := make([]byte, 0, len(updatedContents)-(end-start)+len(update.replacementValue))
		replacedContents = append(replacedContents, updatedContents[:start]...)
		replacedContents = append(replacedContents, update.replacementValue...)
		replacedContents = append(replacedContents, updatedContents[end:]...)
		updatedContents = replacedContents
	}

	return updatedContents, nil
}

func quoteTOMLString(value string) ([]byte, error) {
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("replacement value must be valid UTF-8")
	}

	var buffer bytes.Buffer
	buffer.WriteByte('"')
	for _, character := range value {
		switch character {
		case '\b':
			buffer.WriteString(`\b`)
		case '\t':
			buffer.WriteString(`\t`)
		case '\n':
			buffer.WriteString(`\n`)
		case '\f':
			buffer.WriteString(`\f`)
		case '\r':
			buffer.WriteString(`\r`)
		case '"':
			buffer.WriteString(`\"`)
		case '\\':
			buffer.WriteString(`\\`)
		default:
			if character < 0x20 || character == 0x7f {
				buffer.WriteString(fmt.Sprintf(`\u%04X`, character))
				continue
			}

			buffer.WriteRune(character)
		}
	}
	buffer.WriteByte('"')

	return buffer.Bytes(), nil
}

func inspectTOMLStringTargetsContents(contents []byte) ([]TOMLStringTargetNode, error) {
	document, err := parseTOMLDocument(contents)
	if err != nil {
		return nil, err
	}

	nodes := buildTOMLStringTargetNodes(document.selectableStringPaths())
	if len(nodes) == 0 {
		return nil, fmt.Errorf("does not contain any existing string-valued TOML paths")
	}

	return nodes, nil
}

func (document *tomlDocument) selectableStringPaths() [][]string {
	paths := make([][]string, 0, len(document.stringPaths))
	for _, pathSegments := range document.stringPaths {
		if _, ok := document.arrayTablePrefix(pathSegments); ok {
			continue
		}

		paths = append(paths, pathSegments)
	}

	return paths
}

func buildTOMLStringTargetNodes(paths [][]string) []TOMLStringTargetNode {
	root := newTOMLStringTargetTreeNode("", nil)
	for _, pathSegments := range paths {
		root.insert(pathSegments)
	}

	return root.targetNodes()
}

type tomlStringTargetTreeNode struct {
	TOMLStringTargetNode
	children     []*tomlStringTargetTreeNode
	childIndexes map[string]int
}

func newTOMLStringTargetTreeNode(name string, parentSegments []string) *tomlStringTargetTreeNode {
	pathSegments := appendPathSegments(parentSegments, name)
	if name == "" {
		pathSegments = nil
	}

	return &tomlStringTargetTreeNode{
		TOMLStringTargetNode: TOMLStringTargetNode{
			Name:     name,
			TOMLPath: strings.Join(pathSegments, "."),
		},
		childIndexes: make(map[string]int),
	}
}

func (node *tomlStringTargetTreeNode) insert(pathSegments []string) {
	currentNode := node
	var parentSegments []string
	for index, segment := range pathSegments {
		childIndex, exists := currentNode.childIndexes[segment]
		if !exists {
			childNode := newTOMLStringTargetTreeNode(segment, parentSegments)
			currentNode.children = append(currentNode.children, childNode)
			childIndex = len(currentNode.children) - 1
			currentNode.childIndexes[segment] = childIndex
		}

		currentNode = currentNode.children[childIndex]
		parentSegments = appendPathSegments(parentSegments, segment)
		if index == len(pathSegments)-1 {
			currentNode.Selectable = true
		}
	}
}

func (node *tomlStringTargetTreeNode) targetNodes() []TOMLStringTargetNode {
	result := make([]TOMLStringTargetNode, 0, len(node.children))
	for _, child := range node.children {
		targetNode := child.TOMLStringTargetNode
		childNodes := child.targetNodes()
		if len(childNodes) > 0 {
			targetNode.Children = childNodes
		}
		result = append(result, targetNode)
	}

	return result
}

func tomlNodeKeySegments(node *unstable.Node) []string {
	iterator := node.Key()
	pathSegments := make([]string, 0)
	for iterator.Next() {
		pathSegments = append(pathSegments, string(iterator.Node().Data))
	}

	return pathSegments
}

func appendPathSegments(pathSegments []string, additionalSegments ...string) []string {
	combinedSegments := make([]string, len(pathSegments)+len(additionalSegments))
	copy(combinedSegments, pathSegments)
	copy(combinedSegments[len(pathSegments):], additionalSegments)

	return combinedSegments
}

func clonePathSegments(pathSegments []string) []string {
	return appendPathSegments(nil, pathSegments...)
}

func tomlPathKey(pathSegments []string) string {
	return strings.Join(pathSegments, "\x00")
}

func isInspectableTOMLPath(pathSegments []string) bool {
	for _, segment := range pathSegments {
		if !isInspectableTOMLPathSegment(segment) {
			return false
		}
	}

	return true
}

func isInspectableTOMLPathSegment(segment string) bool {
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
