package editor

// TargetFileCandidate describes one discovered JSON file that contains at
// least one existing string-valued JSON path that Switchlet can manage.
type TargetFileCandidate struct {
	Path         string
	RelativePath string
}

// StringTargetNode describes one browseable JSON property that either resolves
// to a selectable string value or contains nested selectable children.
type StringTargetNode struct {
	Name       string
	JSONPath   string
	Selectable bool
	Children   []StringTargetNode
}
