package app

import (
	"errors"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/profile"
)

// EnvironmentState is derived from the existing resolver, never from a second
// read or separate validation of the environment.
type EnvironmentState string

const (
	EnvironmentReady       EnvironmentState = "ready"
	EnvironmentMissing     EnvironmentState = "missing"
	EnvironmentEmpty       EnvironmentState = "empty"
	EnvironmentInvalidType EnvironmentState = "invalid_type"
)

// EnvironmentUse describes one typed expectation of a shared variable.
type EnvironmentUse struct {
	ProfileName string
	TargetName  string
	ValueType   config.ValueType
	State       EnvironmentState
}

// EnvironmentRequirement groups safe readiness states by environment name.
type EnvironmentRequirement struct {
	Name  string
	State EnvironmentState
	Uses  []EnvironmentUse
}

func environmentState(err error) EnvironmentState {
	switch {
	case err == nil:
		return EnvironmentReady
	case errors.Is(err, profile.ErrEnvironmentVariableNotSet):
		return EnvironmentMissing
	case errors.Is(err, profile.ErrEnvironmentVariableEmpty):
		return EnvironmentEmpty
	default:
		return EnvironmentInvalidType
	}
}

func environmentRequirements(items []ProfileItem) []EnvironmentRequirement {
	var requirements []EnvironmentRequirement
	indexes := make(map[string]int)
	for _, item := range items {
		for _, value := range item.Values {
			if value.Source != ProfileSourceEnvironment {
				continue
			}
			index, ok := indexes[value.EnvironmentVariableName]
			if !ok {
				index = len(requirements)
				indexes[value.EnvironmentVariableName] = index
				requirements = append(requirements, EnvironmentRequirement{Name: value.EnvironmentVariableName, State: EnvironmentReady})
			}
			requirement := &requirements[index]
			requirement.Uses = append(requirement.Uses, EnvironmentUse{ProfileName: item.Name, TargetName: value.TargetName, ValueType: config.EffectiveValueType(value.ValueType), State: value.environmentState})
			if value.environmentState != EnvironmentReady {
				requirement.State = value.environmentState
			}
		}
	}
	return requirements
}
