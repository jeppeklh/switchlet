package editor

import "github.com/jeppeklh/switchlet/internal/config"

// TargetFileCandidate describes one discovered target file that contains at
// least one existing selector that Switchlet can manage.
type TargetFileCandidate struct {
	Path         string
	RelativePath string
	Type         config.TargetType
}

// StringTargetNode describes one browseable JSON property that either resolves
// to a selectable string value or contains nested selectable children.
type StringTargetNode struct {
	Name       string
	JSONPath   string
	Selectable bool
	Children   []StringTargetNode
}

// YAMLStringTargetNode describes one browseable YAML mapping key that either
// resolves to a selectable string scalar or contains nested selectable children.
type YAMLStringTargetNode struct {
	Name       string
	YAMLPath   string
	Selectable bool
	Children   []YAMLStringTargetNode
}
