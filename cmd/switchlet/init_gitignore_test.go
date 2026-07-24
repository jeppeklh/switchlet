package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

func TestRunInit_DoesNotOfferGitignoreProtectionForEnvironmentOnlyProfiles(t *testing.T) {
	projectRoot := t.TempDir()
	input := strings.NewReader(strings.Join([]string{
		"1",
		"1",
		"Test",
		"2",
		"MYAPP_TEST_DATABASE_URL",
		"n",
		"n",
		"",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runInit(projectRoot, input, &output, initDependencies{
		validateCreateLocation: func(string) error { return nil },
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return []editor.TargetFileCandidate{{
				Path:         filepath.Join(projectRoot, "config.json"),
				RelativePath: "config.json",
			}}, nil
		},
		inspectStringTargets: func(string) ([]editor.StringTargetNode, error) {
			return []editor.StringTargetNode{{
				Name:       "service.baseUrl",
				JSONPath:   "service.baseUrl",
				Selectable: true,
			}}, nil
		},
		validateStringTarget: func(string, string) error { return nil },
		createConfig: func(string, config.Target, []config.Profile) (string, config.Config, error) {
			return filepath.Join(projectRoot, ".switchlet.yaml"), config.Config{
				Version: 2,
				Target: config.Target{
					File:     filepath.Join(projectRoot, "config.json"),
					JSONPath: "service.baseUrl",
				},
				Profiles: []config.Profile{{
					Name:         "Test",
					ValueFromEnv: stringPointer("MYAPP_TEST_DATABASE_URL"),
				}},
			}, nil
		},
		ensureConfigIgnored: func(string) (bool, error) {
			t.Fatal("ensureConfigIgnored should not be called for env-only profiles")
			return false, nil
		},
		validateCreatedConfig: func(config.Config) error { return nil },
		removeFile:            func(string) error { return nil },
	})
	if err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}

	outputText := output.String()
	if strings.Contains(outputText, "Add .switchlet.yaml to the project .gitignore?") {
		t.Fatalf("init output %q unexpectedly shows the literal-value gitignore prompt", outputText)
	}
	if !strings.Contains(outputText, "Created configuration:") {
		t.Fatalf("init output %q does not report successful configuration creation", outputText)
	}
}
