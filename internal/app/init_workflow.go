package app

import (
	"fmt"
	"path/filepath"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

// InitTarget is the configuration target type used by init workflow callers.
type InitTarget = config.Target

// InitProfile is the configuration profile type used by init workflow callers.
type InitProfile = config.Profile

// InitProfileValue is the profile value type used by init workflow callers.
type InitProfileValue = config.ProfileValue

// InitTargetType identifies the target implementation selected during init.
type InitTargetType = config.TargetType

const (
	// InitTargetTypeJSON identifies a JSON target selected by jsonPath.
	InitTargetTypeJSON InitTargetType = config.TargetTypeJSON
	// InitTargetTypeDotenv identifies a dotenv target selected by key.
	InitTargetTypeDotenv InitTargetType = config.TargetTypeDotenv
	// InitTargetTypeYAML identifies a YAML target selected by yamlPath.
	InitTargetTypeYAML InitTargetType = config.TargetTypeYAML
	// InitTargetTypeTOML identifies a TOML target selected by tomlPath.
	InitTargetTypeTOML InitTargetType = config.TargetTypeTOML
)

// InitTargetFileCandidate describes one discovered init target file candidate.
type InitTargetFileCandidate = editor.TargetFileCandidate

// InitStringTargetNode describes one browseable JSON target node during init.
type InitStringTargetNode = editor.StringTargetNode

// InitYAMLStringTargetNode describes one browseable YAML target node during init.
type InitYAMLStringTargetNode = editor.YAMLStringTargetNode

// InitTOMLStringTargetNode describes one browseable TOML target node during init.
type InitTOMLStringTargetNode = editor.TOMLStringTargetNode

// InitTargetFileSelection contains the inspected file data needed by the init wizard.
type InitTargetFileSelection struct {
	Path        string
	DisplayPath string
	TargetType  InitTargetType
	Nodes       []InitStringTargetNode
	YAMLNodes   []InitYAMLStringTargetNode
	TOMLNodes   []InitTOMLStringTargetNode
	DotenvKeys  []string
}

// InitWorkflowDependencies lets tests or command composition replace init effects.
type InitWorkflowDependencies struct {
	DiscoverTargetFileCandidates func(string) ([]InitTargetFileCandidate, error)
	InspectStringTargets         func(string) ([]InitStringTargetNode, error)
	InspectYAMLStringTargets     func(string) ([]InitYAMLStringTargetNode, error)
	InspectTOMLStringTargets     func(string) ([]InitTOMLStringTargetNode, error)
	InspectDotenvKeys            func(string) ([]string, error)
	ValidateStringTarget         func(string, string) error
	ValidateYAMLTarget           func(string, string) error
	ValidateTOMLTarget           func(string, string) error
	ValidateDotenvTarget         func(string, string) error
}

// InitWorkflow coordinates init effects owned by config and editor packages.
type InitWorkflow struct {
	dependencies InitWorkflowDependencies
}

// NewInitWorkflow creates an init workflow using supplied dependencies and defaults.
func NewInitWorkflow(dependencies InitWorkflowDependencies) InitWorkflow {
	return InitWorkflow{dependencies: dependencies}
}

// DefaultInitWorkflow creates the production init workflow.
func DefaultInitWorkflow() InitWorkflow {
	return NewInitWorkflow(InitWorkflowDependencies{})
}

// DiscoverTargetFileCandidates returns files that can provide an init target.
func (workflow InitWorkflow) DiscoverTargetFileCandidates(projectRoot string) ([]InitTargetFileCandidate, error) {
	if workflow.dependencies.DiscoverTargetFileCandidates != nil {
		return workflow.dependencies.DiscoverTargetFileCandidates(projectRoot)
	}

	return editor.DiscoverTargetFileCandidates(projectRoot)
}

// InspectTargetFileCandidate inspects one discovered file candidate.
func (workflow InitWorkflow) InspectTargetFileCandidate(candidate InitTargetFileCandidate) (InitTargetFileSelection, error) {
	targetType := candidate.Type
	if targetType == "" {
		inferredType, ok := config.InferTargetType(candidate.Path)
		if !ok {
			return InitTargetFileSelection{}, fmt.Errorf("target type cannot be inferred from file %q", candidate.Path)
		}
		targetType = inferredType
	}

	return workflow.InspectTargetFile(candidate.Path, candidate.RelativePath, targetType)
}

// InspectTargetFile inspects one explicit target file using the requested type.
func (workflow InitWorkflow) InspectTargetFile(targetPath string, displayPath string, targetType InitTargetType) (InitTargetFileSelection, error) {
	selection := InitTargetFileSelection{
		Path:        targetPath,
		DisplayPath: displayPath,
		TargetType:  targetType,
	}

	switch targetType {
	case config.TargetTypeJSON:
		nodes, err := workflow.inspectStringTargets(targetPath)
		if err != nil {
			return InitTargetFileSelection{}, err
		}
		selection.Nodes = nodes
	case config.TargetTypeDotenv:
		keys, err := workflow.inspectDotenvKeys(targetPath)
		if err != nil {
			return InitTargetFileSelection{}, err
		}
		selection.DotenvKeys = keys
	case config.TargetTypeYAML:
		nodes, err := workflow.inspectYAMLStringTargets(targetPath)
		if err != nil {
			return InitTargetFileSelection{}, err
		}
		selection.YAMLNodes = nodes
	case config.TargetTypeTOML:
		nodes, err := workflow.inspectTOMLStringTargets(targetPath)
		if err != nil {
			return InitTargetFileSelection{}, err
		}
		selection.TOMLNodes = nodes
	default:
		return InitTargetFileSelection{}, fmt.Errorf("target type %q is not supported", targetType)
	}

	return selection, nil
}

// ValidateStringTarget validates an explicit JSON selector for init.
func (workflow InitWorkflow) ValidateStringTarget(targetPath string, jsonPath string) error {
	if workflow.dependencies.ValidateStringTarget != nil {
		return workflow.dependencies.ValidateStringTarget(targetPath, jsonPath)
	}

	return editor.ValidateStringTarget(targetPath, jsonPath)
}

// ValidateDotenvTarget validates an explicit dotenv key for init.
func (workflow InitWorkflow) ValidateDotenvTarget(targetPath string, key string) error {
	if workflow.dependencies.ValidateDotenvTarget != nil {
		return workflow.dependencies.ValidateDotenvTarget(targetPath, key)
	}

	return editor.ValidateDotenvTarget(targetPath, key)
}

// ValidateYAMLTarget validates an explicit YAML selector for init.
func (workflow InitWorkflow) ValidateYAMLTarget(targetPath string, yamlPath string) error {
	if workflow.dependencies.ValidateYAMLTarget != nil {
		return workflow.dependencies.ValidateYAMLTarget(targetPath, yamlPath)
	}

	return editor.ValidateYAMLTarget(targetPath, yamlPath)
}

// ValidateTOMLTarget validates an explicit TOML selector for init.
func (workflow InitWorkflow) ValidateTOMLTarget(targetPath string, tomlPath string) error {
	if workflow.dependencies.ValidateTOMLTarget != nil {
		return workflow.dependencies.ValidateTOMLTarget(targetPath, tomlPath)
	}

	return editor.ValidateTOMLTarget(targetPath, tomlPath)
}

// InferInitTargetType infers a target type from a path using config ownership rules.
func InferInitTargetType(targetPath string) (InitTargetType, bool) {
	return config.InferTargetType(targetPath)
}

// ResolveInitTargetPath resolves a user-entered init target path from project root.
func ResolveInitTargetPath(workingDirectory string, targetPath string) string {
	resolvedTargetPath := targetPath
	if !filepath.IsAbs(resolvedTargetPath) {
		resolvedTargetPath = filepath.Join(workingDirectory, resolvedTargetPath)
	}

	return filepath.Clean(resolvedTargetPath)
}

// DisplayInitTargetPath returns a project-relative target path when possible.
func DisplayInitTargetPath(workingDirectory string, targetPath string) string {
	relativePath, err := filepath.Rel(workingDirectory, targetPath)
	if err == nil {
		return relativePath
	}

	return targetPath
}

// InitProfilesHaveLiteralValues reports whether init output contains literals.
func InitProfilesHaveLiteralValues(profiles []InitProfile) bool {
	for _, profile := range profiles {
		if profile.Value != nil {
			return true
		}
		for _, value := range profile.Values {
			if value.Value != nil {
				return true
			}
		}
	}

	return false
}

func (workflow InitWorkflow) inspectStringTargets(targetPath string) ([]InitStringTargetNode, error) {
	if workflow.dependencies.InspectStringTargets != nil {
		return workflow.dependencies.InspectStringTargets(targetPath)
	}

	return editor.InspectStringTargets(targetPath)
}

func (workflow InitWorkflow) inspectDotenvKeys(targetPath string) ([]string, error) {
	if workflow.dependencies.InspectDotenvKeys != nil {
		return workflow.dependencies.InspectDotenvKeys(targetPath)
	}

	return editor.InspectDotenvKeys(targetPath)
}

func (workflow InitWorkflow) inspectYAMLStringTargets(targetPath string) ([]InitYAMLStringTargetNode, error) {
	if workflow.dependencies.InspectYAMLStringTargets != nil {
		return workflow.dependencies.InspectYAMLStringTargets(targetPath)
	}

	return editor.InspectYAMLStringTargets(targetPath)
}

func (workflow InitWorkflow) inspectTOMLStringTargets(targetPath string) ([]InitTOMLStringTargetNode, error) {
	if workflow.dependencies.InspectTOMLStringTargets != nil {
		return workflow.dependencies.InspectTOMLStringTargets(targetPath)
	}

	return editor.InspectTOMLStringTargets(targetPath)
}
