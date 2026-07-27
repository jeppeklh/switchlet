package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

func TestRunCommand_HelpWritesUsage(t *testing.T) {
	var output bytes.Buffer

	err := runCommand([]string{"help"}, t.TempDir(), func(model tea.Model) error {
		t.Fatal("runProgram should not be called for help")
		return nil
	}, strings.NewReader(""), &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if !strings.Contains(output.String(), "switchlet init") {
		t.Fatalf("help output %q does not mention init", output.String())
	}
	if !strings.Contains(output.String(), "switchlet list") {
		t.Fatalf("help output %q does not mention list", output.String())
	}
	if !strings.Contains(output.String(), "switchlet inspect <profile-name>") {
		t.Fatalf("help output %q does not mention inspect", output.String())
	}
	if !strings.Contains(output.String(), "switchlet apply <profile-name>") {
		t.Fatalf("help output %q does not mention apply", output.String())
	}
	if !strings.Contains(output.String(), "switchlet help [command]") {
		t.Fatalf("help output %q does not mention help topic usage", output.String())
	}
	if !strings.Contains(output.String(), "Examples:") {
		t.Fatalf("help output %q does not include examples", output.String())
	}
	if !strings.Contains(output.String(), "switchlet apply Local --dry-run") {
		t.Fatalf("help output %q does not include apply dry-run example", output.String())
	}
	if !strings.Contains(output.String(), "switchlet help apply") {
		t.Fatalf("help output %q does not include command help example", output.String())
	}
	if !strings.Contains(output.String(), "--dry-run") {
		t.Fatalf("help output %q does not mention --dry-run", output.String())
	}
	if !strings.Contains(output.String(), "--allow-protected") {
		t.Fatalf("help output %q does not mention --allow-protected", output.String())
	}
	if !strings.Contains(output.String(), "--json") {
		t.Fatalf("help output %q does not mention --json", output.String())
	}
	if !strings.Contains(output.String(), "full-screen terminal UI") {
		t.Fatalf("help output %q does not mention full-screen terminal UI behavior", output.String())
	}
	if !strings.Contains(output.String(), "Enter/y to confirm") {
		t.Fatalf("help output %q does not document the protected confirmation keys", output.String())
	}
}

func TestRunCommand_HelpTopicWritesSubcommandUsage(t *testing.T) {
	var output bytes.Buffer

	err := runCommand([]string{"help", "apply"}, t.TempDir(), func(model tea.Model) error {
		t.Fatal("runProgram should not be called for help topic")
		return nil
	}, strings.NewReader(""), &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if !strings.Contains(output.String(), "switchlet apply <profile-name>") {
		t.Fatalf("help output %q does not mention apply usage", output.String())
	}
	if !strings.Contains(output.String(), "--allow-protected") {
		t.Fatalf("help output %q does not mention apply flags", output.String())
	}
	if !strings.Contains(output.String(), "Examples:") || !strings.Contains(output.String(), "switchlet apply Production --dry-run --allow-protected") {
		t.Fatalf("help output %q does not include apply examples", output.String())
	}
	if !strings.Contains(output.String(), "interactive TUI already prompt for confirmation") {
		t.Fatalf("help output %q does not distinguish the interactive protected flow", output.String())
	}
}

func TestRunCommand_HelpTopicInitWritesGuidedUsage(t *testing.T) {
	var output bytes.Buffer

	err := runCommand([]string{"help", "init"}, t.TempDir(), func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init help topic")
		return nil
	}, strings.NewReader(""), &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if !strings.Contains(output.String(), "guides you through target-file selection") {
		t.Fatalf("help output %q does not describe guided file discovery", output.String())
	}
	if !strings.Contains(output.String(), "large file lists") {
		t.Fatalf("help output %q does not mention large-repository narrowing", output.String())
	}
	if !strings.Contains(output.String(), "dotenv keys") {
		t.Fatalf("help output %q does not mention dotenv key selection", output.String())
	}
	if !strings.Contains(output.String(), "Version 3 target/profile configuration") {
		t.Fatalf("help output %q does not mention Version 3 target/profile output", output.String())
	}
	if !strings.Contains(output.String(), "project .gitignore") {
		t.Fatalf("help output %q does not mention literal-value gitignore protection", output.String())
	}
	if !strings.Contains(output.String(), "full-screen terminal UI") {
		t.Fatalf("help output %q does not mention full-screen wizard behavior", output.String())
	}
}

func TestPromptTargetJSONPath_AllowsSearchingLargeSelectablePathSets(t *testing.T) {
	selectedFile := targetFileSelection{
		path:        "/tmp/config.json",
		displayPath: "config.json",
		targetType:  config.TargetTypeJSON,
		nodes: jsonTargetSelectorNodes([]editor.StringTargetNode{
			{
				Name:     "database",
				JSONPath: "database",
				Children: []editor.StringTargetNode{
					{Name: "primary", JSONPath: "database.primary", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.primary.url", Selectable: true}}},
					{Name: "replicaA", JSONPath: "database.replicaA", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaA.url", Selectable: true}}},
					{Name: "replicaB", JSONPath: "database.replicaB", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaB.url", Selectable: true}}},
					{Name: "replicaC", JSONPath: "database.replicaC", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaC.url", Selectable: true}}},
					{Name: "replicaD", JSONPath: "database.replicaD", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaD.url", Selectable: true}}},
					{Name: "replicaE", JSONPath: "database.replicaE", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaE.url", Selectable: true}}},
					{Name: "replicaF", JSONPath: "database.replicaF", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaF.url", Selectable: true}}},
					{Name: "replicaG", JSONPath: "database.replicaG", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaG.url", Selectable: true}}},
					{Name: "replicaH", JSONPath: "database.replicaH", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaH.url", Selectable: true}}},
					{Name: "replicaI", JSONPath: "database.replicaI", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaI.url", Selectable: true}}},
					{Name: "replicaJ", JSONPath: "database.replicaJ", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaJ.url", Selectable: true}}},
					{Name: "replicaK", JSONPath: "database.replicaK", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaK.url", Selectable: true}}},
					{Name: "replicaL", JSONPath: "database.replicaL", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaL.url", Selectable: true}}},
				},
			},
		}),
	}
	dependencies := initDependencies{
		validateStringTarget: func(string, string) error { return nil },
	}
	var output bytes.Buffer
	prompter := initPrompter{
		reader: bufio.NewReader(strings.NewReader(strings.Join([]string{"2", "replicaL", "1"}, "\n") + "\n")),
		writer: &output,
	}

	jsonPath, chooseDifferentFile, err := promptTargetJSONPath(prompter, selectedFile, dependencies)
	if err != nil {
		t.Fatalf("promptTargetJSONPath returned error: %v", err)
	}
	if chooseDifferentFile {
		t.Fatal("promptTargetJSONPath chose a different file, want selected JSON path")
	}
	if jsonPath != "database.replicaL.url" {
		t.Fatalf("jsonPath = %q, want %q", jsonPath, "database.replicaL.url")
	}
	if !strings.Contains(output.String(), `Select JSON value matching "replicaL" in config.json:`) {
		t.Fatalf("prompt output %q does not report the filtered JSON-path selection", output.String())
	}
}

func TestPromptTargetJSONPath_SearchKeepsPathSelectionRecoverableWhenNothingMatches(t *testing.T) {
	selectedFile := targetFileSelection{
		path:        "/tmp/config.json",
		displayPath: "config.json",
		targetType:  config.TargetTypeJSON,
		nodes: jsonTargetSelectorNodes([]editor.StringTargetNode{{
			Name:     "database",
			JSONPath: "database",
			Children: []editor.StringTargetNode{
				{Name: "primary", JSONPath: "database.primary", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.primary.url", Selectable: true}}},
				{Name: "replicaA", JSONPath: "database.replicaA", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaA.url", Selectable: true}}},
				{Name: "replicaB", JSONPath: "database.replicaB", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaB.url", Selectable: true}}},
				{Name: "replicaC", JSONPath: "database.replicaC", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaC.url", Selectable: true}}},
				{Name: "replicaD", JSONPath: "database.replicaD", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaD.url", Selectable: true}}},
				{Name: "replicaE", JSONPath: "database.replicaE", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaE.url", Selectable: true}}},
				{Name: "replicaF", JSONPath: "database.replicaF", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaF.url", Selectable: true}}},
				{Name: "replicaG", JSONPath: "database.replicaG", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaG.url", Selectable: true}}},
				{Name: "replicaH", JSONPath: "database.replicaH", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaH.url", Selectable: true}}},
				{Name: "replicaI", JSONPath: "database.replicaI", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaI.url", Selectable: true}}},
				{Name: "replicaJ", JSONPath: "database.replicaJ", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaJ.url", Selectable: true}}},
				{Name: "replicaK", JSONPath: "database.replicaK", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaK.url", Selectable: true}}},
				{Name: "replicaL", JSONPath: "database.replicaL", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replicaL.url", Selectable: true}}},
			},
		}}),
	}
	dependencies := initDependencies{
		validateStringTarget: func(string, string) error { return nil },
	}
	var output bytes.Buffer
	prompter := initPrompter{
		reader: bufio.NewReader(strings.NewReader(strings.Join([]string{"2", "missing", "replicaA", "1"}, "\n") + "\n")),
		writer: &output,
	}

	jsonPath, chooseDifferentFile, err := promptTargetJSONPath(prompter, selectedFile, dependencies)
	if err != nil {
		t.Fatalf("promptTargetJSONPath returned error: %v", err)
	}
	if chooseDifferentFile {
		t.Fatal("promptTargetJSONPath chose a different file, want selected JSON path")
	}
	if jsonPath != "database.replicaA.url" {
		t.Fatalf("jsonPath = %q, want %q", jsonPath, "database.replicaA.url")
	}
	if !strings.Contains(output.String(), `No selectable JSON paths in config.json match "missing".`) {
		t.Fatalf("prompt output %q does not report the recoverable no-match search state", output.String())
	}
}

func TestRunInit_LimitsLargeDiscoveredTargetFileListUntilTheUserNarrowsIt(t *testing.T) {
	projectRoot := t.TempDir()
	desiredCandidate := editor.TargetFileCandidate{
		Path:         filepath.Join(projectRoot, "src", "MyApplication", "appsettings.Development.json"),
		RelativePath: filepath.Join("src", "MyApplication", "appsettings.Development.json"),
	}
	candidates := make([]editor.TargetFileCandidate, 0, targetFileChoiceWindowSize+4)
	for index := 0; index < targetFileChoiceWindowSize+3; index++ {
		candidates = append(candidates, editor.TargetFileCandidate{
			Path:         filepath.Join(projectRoot, "packages", fmt.Sprintf("package-%02d", index), "config.json"),
			RelativePath: filepath.Join("packages", fmt.Sprintf("package-%02d", index), "config.json"),
		})
	}
	candidates = append(candidates, desiredCandidate)

	input := strings.NewReader(strings.Join([]string{
		strconv.Itoa(targetFileChoiceWindowSize + 1),
		"appsettings",
		"1",
		"1",
		"database",
		"n",
		"Local",
		"1",
		"postgres://localhost:5432/myapp",
		"n",
		"n",
		"n",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runInit(projectRoot, input, &output, initDependencies{
		validateCreateLocation: func(string) error { return nil },
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return candidates, nil
		},
		inspectStringTargets: func(string) ([]editor.StringTargetNode, error) {
			return []editor.StringTargetNode{{
				Name:       "url",
				JSONPath:   "database.primary.url",
				Selectable: true,
			}}, nil
		},
		validateStringTarget: func(string, string) error { return nil },
		createConfig: func(string, []config.Target, []config.Profile) (string, config.Config, error) {
			t.Fatal("createConfig should not be called after cancellation")
			return "", config.Config{}, nil
		},
		validateCreatedConfig: func(config.Config) error { return nil },
		removeFile:            func(string) error { return nil },
	})
	if err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}

	outputText := output.String()
	if !strings.Contains(outputText, fmt.Sprintf("showing %d of %d discovered files", targetFileChoiceWindowSize, len(candidates))) {
		t.Fatalf("init output %q does not report the truncated candidate window", outputText)
	}
	if strings.Contains(outputText, candidates[targetFileChoiceWindowSize+1].RelativePath) {
		t.Fatalf("init output %q should not print candidates outside the initial result window", outputText)
	}
	if !strings.Contains(outputText, desiredCandidate.RelativePath) {
		t.Fatalf("init output %q does not show the narrowed matching candidate", outputText)
	}
	if !strings.Contains(outputText, "database [json] -> src/MyApplication/appsettings.Development.json") {
		t.Fatalf("init output %q does not summarize the narrowed target file", outputText)
	}
}

func TestRunInit_KeepsFileSelectionRecoverableWhenAFilterMatchesNothing(t *testing.T) {
	projectRoot := t.TempDir()
	desiredCandidate := editor.TargetFileCandidate{
		Path:         filepath.Join(projectRoot, "src", "MyApplication", "appsettings.Development.json"),
		RelativePath: filepath.Join("src", "MyApplication", "appsettings.Development.json"),
	}
	candidates := make([]editor.TargetFileCandidate, 0, targetFileChoiceWindowSize+2)
	for index := 0; index < targetFileChoiceWindowSize+1; index++ {
		candidates = append(candidates, editor.TargetFileCandidate{
			Path:         filepath.Join(projectRoot, "packages", fmt.Sprintf("package-%02d", index), "config.json"),
			RelativePath: filepath.Join("packages", fmt.Sprintf("package-%02d", index), "config.json"),
		})
	}
	candidates = append(candidates, desiredCandidate)

	input := strings.NewReader(strings.Join([]string{
		strconv.Itoa(targetFileChoiceWindowSize + 1),
		"missing",
		"appsettings",
		"1",
		"1",
		"database",
		"n",
		"Local",
		"1",
		"postgres://localhost:5432/myapp",
		"n",
		"n",
		"n",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runInit(projectRoot, input, &output, initDependencies{
		validateCreateLocation: func(string) error { return nil },
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return candidates, nil
		},
		inspectStringTargets: func(string) ([]editor.StringTargetNode, error) {
			return []editor.StringTargetNode{{
				Name:       "url",
				JSONPath:   "database.primary.url",
				Selectable: true,
			}}, nil
		},
		validateStringTarget: func(string, string) error { return nil },
		createConfig: func(string, []config.Target, []config.Profile) (string, config.Config, error) {
			t.Fatal("createConfig should not be called after cancellation")
			return "", config.Config{}, nil
		},
		validateCreatedConfig: func(config.Config) error { return nil },
		removeFile:            func(string) error { return nil },
	})
	if err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}

	outputText := output.String()
	if !strings.Contains(outputText, `No discovered configuration files match "missing".`) {
		t.Fatalf("init output %q does not report the no-match filter state", outputText)
	}
	if !strings.Contains(outputText, "database [json] -> src/MyApplication/appsettings.Development.json") {
		t.Fatalf("init output %q does not recover to the narrowed target file", outputText)
	}
}

func TestRunCommand_InitCreatesConfigurationFromGuidedSelection(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "primary": {
      "url": "postgres://old"
    }
  }
}
`)+"\n")

	input := strings.NewReader(strings.Join([]string{
		"1",
		"1",
		"1",
		"1",
		"database",
		"n",
		"Local",
		"1",
		"postgres://localhost:5432/myapp",
		"n",
		"n",
		"",
		"",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runCommand([]string{"init"}, projectRoot, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, input, &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}

	loadedConfig, err := config.Load(filepath.Join(projectRoot, ".switchlet.yaml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loadedConfig.Version != 3 {
		t.Fatalf("Version = %d, want 3", loadedConfig.Version)
	}
	if len(loadedConfig.Targets) != 1 {
		t.Fatalf("len(targets) = %d, want 1", len(loadedConfig.Targets))
	}
	if loadedConfig.Targets[0].Name != "database" {
		t.Fatalf("target name = %q, want database", loadedConfig.Targets[0].Name)
	}
	if loadedConfig.Targets[0].JSONPath != "database.primary.url" {
		t.Fatalf("JSON path = %q, want %q", loadedConfig.Targets[0].JSONPath, "database.primary.url")
	}
	if len(loadedConfig.Profiles) != 1 {
		t.Fatalf("len(profiles) = %d, want 1", len(loadedConfig.Profiles))
	}
	if loadedConfig.Profiles[0].Name != "Local" {
		t.Fatalf("profile name = %q, want %q", loadedConfig.Profiles[0].Name, "Local")
	}
	if len(loadedConfig.Profiles[0].Values) != 1 {
		t.Fatalf("len(profile values) = %d, want 1", len(loadedConfig.Profiles[0].Values))
	}
	profileValue := loadedConfig.Profiles[0].Values[0]
	if profileValue.Target != "database" {
		t.Fatalf("profile value target = %q, want database", profileValue.Target)
	}
	if profileValue.Value == nil || *profileValue.Value != "postgres://localhost:5432/myapp" {
		t.Fatalf("literal profile value = %#v, want configured literal value", profileValue.Value)
	}

	contents, err := os.ReadFile(filepath.Join(projectRoot, ".switchlet.yaml"))
	if err != nil {
		t.Fatalf("read configuration file: %v", err)
	}
	gitignoreContents, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("read gitignore file: %v", err)
	}
	if !strings.Contains(string(contents), "version: 3") {
		t.Fatalf("configuration file contents %q do not contain version 3", string(contents))
	}
	if !strings.Contains(string(contents), "name: database") {
		t.Fatalf("configuration file contents %q do not contain target name", string(contents))
	}
	if !strings.Contains(string(contents), "jsonPath: database.primary.url") {
		t.Fatalf("configuration file contents %q do not contain JSON path", string(contents))
	}
	if !strings.Contains(string(contents), "target: database") {
		t.Fatalf("configuration file contents %q do not contain profile target reference", string(contents))
	}
	if strings.Contains(string(contents), "connectionName:") {
		t.Fatalf("configuration file contents %q must not contain legacy connectionName", string(contents))
	}
	if !strings.Contains(output.String(), "Created configuration:") {
		t.Fatalf("init output %q does not report created configuration", output.String())
	}
	if strings.Contains(output.String(), "postgres://old") {
		t.Fatalf("init output %q must not include the existing target value", output.String())
	}
	if !strings.Contains(output.String(), "Detected format: JSON") {
		t.Fatalf("init output %q does not show the detected target file format", output.String())
	}
	if !strings.Contains(output.String(), "Step 5 of 5: Review and create configuration") {
		t.Fatalf("init output %q does not include the final review step", output.String())
	}
	if !strings.Contains(output.String(), "Create .switchlet.yaml now? [Y/n]: ") {
		t.Fatalf("init output %q does not show the Enter-as-yes confirmation prompt", output.String())
	}
	if !strings.Contains(output.String(), "Add .switchlet.yaml to the project .gitignore? [Y/n]: ") {
		t.Fatalf("init output %q does not show the literal-value gitignore prompt", output.String())
	}
	if !strings.Contains(output.String(), "Updated project .gitignore to ignore .switchlet.yaml.") {
		t.Fatalf("init output %q does not report the gitignore update", output.String())
	}
	if string(gitignoreContents) != ".switchlet.yaml\n" {
		t.Fatalf("gitignore contents = %q, want %q", string(gitignoreContents), ".switchlet.yaml\n")
	}
}

func TestRunCommand_InitCreatesYAMLTargetFromGuidedSelection(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, "worker.yaml", strings.TrimSpace(`
queue:
  endpoint: https://old.example.test
`)+"\n")

	input := strings.NewReader(strings.Join([]string{
		"1",
		"1",
		"1",
		"workerQueue",
		"n",
		"Local",
		"1",
		"https://queue.local",
		"n",
		"n",
		"",
		"",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runCommand([]string{"init"}, projectRoot, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, input, &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}

	loadedConfig, err := config.Load(filepath.Join(projectRoot, ".switchlet.yaml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loadedConfig.Targets) != 1 {
		t.Fatalf("len(targets) = %d, want 1", len(loadedConfig.Targets))
	}
	target := loadedConfig.Targets[0]
	if target.Name != "workerQueue" || target.Type != config.TargetTypeYAML || target.YAMLPath != "queue.endpoint" {
		t.Fatalf("target = %#v, want workerQueue YAML target", target)
	}
	if target.JSONPath != "" || target.Key != "" {
		t.Fatalf("target = %#v, want no JSON path or dotenv key", target)
	}

	contents, err := os.ReadFile(filepath.Join(projectRoot, ".switchlet.yaml"))
	if err != nil {
		t.Fatalf("read configuration file: %v", err)
	}
	for _, expected := range []string{"type: yaml", "yamlPath: queue.endpoint", "target: workerQueue"} {
		if !strings.Contains(string(contents), expected) {
			t.Fatalf("configuration file contents %q do not contain %q", string(contents), expected)
		}
	}
	if strings.Contains(string(contents), "jsonPath:") || strings.Contains(string(contents), "key:") {
		t.Fatalf("configuration file contents %q should not contain JSON path or dotenv key fields", string(contents))
	}
	if !strings.Contains(output.String(), "Detected format: YAML") || !strings.Contains(output.String(), "YAML path: queue.endpoint") {
		t.Fatalf("init output %q does not summarize YAML target selection", output.String())
	}
}

func TestRunCommand_InitReturnsErrorWhenConfigurationExistsInParentDirectory(t *testing.T) {
	workspaceRoot := t.TempDir()
	projectRoot := filepath.Join(workspaceRoot, "project")
	nestedDirectory := filepath.Join(projectRoot, "src", "MyApplication")

	if err := os.MkdirAll(nestedDirectory, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 1

target:
  file: appsettings.Development.json
  connectionName: DefaultConnection

profiles:
  - name: Local
    value: "Server=localhost;Database=App;"
`)+"\n")

	var output bytes.Buffer
	err := runCommand([]string{"init"}, nestedDirectory, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, strings.NewReader(""), &output)
	if err == nil {
		t.Fatal("runCommand returned nil error, want existing configuration error")
	}
	if !strings.Contains(err.Error(), "discovered existing configuration file") {
		t.Fatalf("runCommand returned error %q, want existing configuration error", err)
	}
}

func TestRunCommand_InitCancellationDoesNotWriteConfiguration(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "service": {
    "baseUrl": "https://old.example.test"
  }
}
`)+"\n")

	input := strings.NewReader(strings.Join([]string{
		"1",
		"1",
		"1",
		"service",
		"n",
		"Local",
		"1",
		"https://new.example.test",
		"n",
		"n",
		"n",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runCommand([]string{"init"}, projectRoot, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, input, &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}

	_, statErr := os.Stat(filepath.Join(projectRoot, ".switchlet.yaml"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("configuration file stat error = %v, want not-exist error", statErr)
	}
	if !strings.Contains(output.String(), "Initialization cancelled.") {
		t.Fatalf("init output %q does not report cancellation", output.String())
	}
}

func TestRunInit_RemovesCreatedConfigurationWhenFinalValidationFails(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "primary": {
      "url": "postgres://old"
    }
  }
}
`)+"\n")

	input := strings.NewReader(strings.Join([]string{
		"1",
		"1",
		"1",
		"1",
		"database",
		"n",
		"Local",
		"1",
		"postgres://localhost:5432/myapp",
		"n",
		"n",
		"y",
		"y",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runInit(projectRoot, input, &output, initDependencies{
		validateCreateLocation:       config.ValidateCreateLocation,
		discoverTargetFileCandidates: editor.DiscoverTargetFileCandidates,
		inspectStringTargets:         editor.InspectStringTargets,
		validateStringTarget:         editor.ValidateStringTarget,
		createConfig:                 config.Create,
		validateCreatedConfig: func(loadedConfig config.Config) error {
			return errors.New("target validation failed")
		},
		removeFile: os.Remove,
	})
	if err == nil {
		t.Fatal("runInit returned nil error, want final validation error")
	}
	if !strings.Contains(err.Error(), "target validation failed") {
		t.Fatalf("runInit returned error %q, want final validation error", err)
	}

	_, statErr := os.Stat(filepath.Join(projectRoot, ".switchlet.yaml"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("configuration file stat error = %v, want not-exist error", statErr)
	}
}

func TestRunCommand_InitKeepsSelectedFileWhileRetryingJSONPathSelection(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "replica": {
      "url": "postgres://replica"
    }
  }
}
`)+"\n")

	input := strings.NewReader(strings.Join([]string{
		"1",
		"2",
		"database.primary.url",
		"1",
		"1",
		"1",
		"database",
		"n",
		"Local",
		"1",
		"postgres://localhost:5432/myapp",
		"n",
		"n",
		"n",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runCommand([]string{"init"}, projectRoot, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, input, &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if !strings.Contains(output.String(), `does not contain JSON path "database.primary.url"`) {
		t.Fatalf("init output %q does not report missing JSON path", output.String())
	}
	if strings.Count(output.String(), "Select configuration file:") != 1 {
		t.Fatalf("init output %q should prompt for the target file only once", output.String())
	}

	_, statErr := os.Stat(filepath.Join(projectRoot, ".switchlet.yaml"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("configuration file stat error = %v, want not-exist error after cancellation", statErr)
	}
}

func TestRunCommand_InitFallsBackToManualFileAndJSONPathEntryWhenDiscoveryFindsNothing(t *testing.T) {
	workspaceRoot := t.TempDir()
	projectRoot := filepath.Join(workspaceRoot, "project")
	sharedRoot := filepath.Join(workspaceRoot, "shared")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("create project root: %v", err)
	}
	writeFile(t, projectRoot, "arrays-only.json", `{"services":[{"baseUrl":"https://old.example.test"}]}`)
	writeFile(t, sharedRoot, "config.json", strings.TrimSpace(`
{
  "service": {
    "baseUrl": "https://old.example.test"
  }
}
`)+"\n")

	input := strings.NewReader(strings.Join([]string{
		"missing.json",
		filepath.Join("..", "shared", "config.json"),
		"2",
		"service.baseUrl",
		"service",
		"n",
		"Local",
		"1",
		"https://new.example.test",
		"n",
		"n",
		"n",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runCommand([]string{"init"}, projectRoot, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, input, &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if !strings.Contains(output.String(), "No supported configuration files with existing JSON or YAML string values or unambiguous dotenv keys were discovered") {
		t.Fatalf("init output %q does not report empty discovery results", output.String())
	}
	if !strings.Contains(output.String(), `Error: stat target file`) {
		t.Fatalf("init output %q does not report the invalid manual file path", output.String())
	}

	_, statErr := os.Stat(filepath.Join(projectRoot, ".switchlet.yaml"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("configuration file stat error = %v, want not-exist error after cancellation", statErr)
	}
}

func TestRunCommand_InitAllowsManualTargetFileEntryOutsideTheDiscoveredCandidateSet(t *testing.T) {
	workspaceRoot := t.TempDir()
	projectRoot := filepath.Join(workspaceRoot, "project")
	sharedRoot := filepath.Join(workspaceRoot, "shared")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("create project root: %v", err)
	}
	writeFile(t, projectRoot, "config.json", `{"serviceUrl":"https://project.example.test"}`)
	writeFile(t, sharedRoot, "config.json", `{"serviceUrl":"https://shared.example.test"}`)

	input := strings.NewReader(strings.Join([]string{
		"2",
		filepath.Join("..", "shared", "config.json"),
		"1",
		"service",
		"n",
		"Local",
		"1",
		"https://new.example.test",
		"n",
		"n",
		"n",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runCommand([]string{"init"}, projectRoot, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, input, &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if !strings.Contains(output.String(), filepath.Join("..", "shared", "config.json")) {
		t.Fatalf("init output %q does not summarize the manually entered off-path target file", output.String())
	}

	_, statErr := os.Stat(filepath.Join(projectRoot, ".switchlet.yaml"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("configuration file stat error = %v, want not-exist error after cancellation", statErr)
	}
}

func TestRunCommand_InitAllowsBackingOutToChooseDifferentFile(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, "a.json", strings.TrimSpace(`
{
  "service": {
    "baseUrl": "https://first.example.test"
  }
}
`)+"\n")
	writeFile(t, projectRoot, "b.json", strings.TrimSpace(`
{
  "database": {
    "primary": {
      "url": "postgres://second"
    }
  }
}
`)+"\n")

	input := strings.NewReader(strings.Join([]string{
		"1",
		"3",
		"2",
		"1",
		"1",
		"1",
		"database",
		"n",
		"Local",
		"1",
		"postgres://localhost:5432/myapp",
		"n",
		"n",
		"n",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runCommand([]string{"init"}, projectRoot, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, input, &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if strings.Count(output.String(), "Select configuration file:") != 2 {
		t.Fatalf("init output %q should prompt for the target file twice", output.String())
	}
	if !strings.Contains(output.String(), "database [json] -> b.json") {
		t.Fatalf("init output %q does not summarize the second selected target file", output.String())
	}
	if !strings.Contains(output.String(), "JSON path: database.primary.url") {
		t.Fatalf("init output %q does not summarize the selected JSON path", output.String())
	}

	_, statErr := os.Stat(filepath.Join(projectRoot, ".switchlet.yaml"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("configuration file stat error = %v, want not-exist error after cancellation", statErr)
	}
}

func TestRunCommand_InitCreatesVersionThreeConfigForMultipleTargetsAndPartialProfiles(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "url": "postgres://old"
  }
}
`)+"\n")
	writeFile(t, projectRoot, "frontend/.env.local", strings.TrimSpace(`
VITE_API_URL=http://localhost:5173
VITE_FEATURES=local
`)+"\n")

	input := strings.NewReader(strings.Join([]string{
		"1",
		"1",
		"1",
		"database",
		"y",
		"2",
		"1",
		"frontendApi",
		"n",
		"Local Database",
		"y",
		"1",
		"postgres://localhost:5432/app",
		"n",
		"n",
		"y",
		"Frontend Local",
		"n",
		"y",
		"1",
		"http://localhost:5173",
		"n",
		"n",
		"",
		"",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runCommand([]string{"init"}, projectRoot, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, input, &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v\noutput: %s", err, output.String())
	}

	loadedConfig, err := config.Load(filepath.Join(projectRoot, ".switchlet.yaml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loadedConfig.Version != 3 {
		t.Fatalf("Version = %d, want 3", loadedConfig.Version)
	}
	if len(loadedConfig.Targets) != 2 {
		t.Fatalf("len(targets) = %d, want 2", len(loadedConfig.Targets))
	}
	if loadedConfig.Targets[0].Name != "database" || loadedConfig.Targets[0].Type != config.TargetTypeJSON || loadedConfig.Targets[0].JSONPath != "database.url" {
		t.Fatalf("first target = %#v, want database JSON target", loadedConfig.Targets[0])
	}
	if loadedConfig.Targets[1].Name != "frontendApi" || loadedConfig.Targets[1].Type != config.TargetTypeDotenv || loadedConfig.Targets[1].Key != "VITE_API_URL" {
		t.Fatalf("second target = %#v, want frontendApi dotenv target", loadedConfig.Targets[1])
	}
	if len(loadedConfig.Profiles) != 2 {
		t.Fatalf("len(profiles) = %d, want 2", len(loadedConfig.Profiles))
	}
	if len(loadedConfig.Profiles[0].Values) != 1 || loadedConfig.Profiles[0].Values[0].Target != "database" {
		t.Fatalf("first profile values = %#v, want database-only partial profile", loadedConfig.Profiles[0].Values)
	}
	if len(loadedConfig.Profiles[1].Values) != 1 || loadedConfig.Profiles[1].Values[0].Target != "frontendApi" {
		t.Fatalf("second profile values = %#v, want frontendApi-only partial profile", loadedConfig.Profiles[1].Values)
	}

	outputText := output.String()
	for _, expected := range []string{
		"Set database in Local Database? [Y/n]:",
		"Set frontendApi in Local Database? [Y/n]:",
		"Leaving frontendApi unchanged in Local Database.",
		"Set frontendApi for Frontend Local",
	} {
		if !strings.Contains(outputText, expected) {
			t.Fatalf("init output %q does not contain managed-value prompt %q", outputText, expected)
		}
	}

	contents, err := os.ReadFile(filepath.Join(projectRoot, ".switchlet.yaml"))
	if err != nil {
		t.Fatalf("read configuration file: %v", err)
	}
	for _, expected := range []string{"version: 3", "type: dotenv", "key: VITE_API_URL", "target: frontendApi"} {
		if !strings.Contains(string(contents), expected) {
			t.Fatalf("configuration file contents %q do not contain %q", string(contents), expected)
		}
	}
}

func TestPromptTargetName_RefusesDuplicateManagedValueNames(t *testing.T) {
	var output bytes.Buffer
	prompter := initPrompter{
		reader: bufio.NewReader(strings.NewReader("database\nfrontendApi\n")),
		writer: &output,
	}

	name, err := promptTargetName(prompter, map[string]struct{}{"database": {}})
	if err != nil {
		t.Fatalf("promptTargetName returned error: %v", err)
	}
	if name != "frontendApi" {
		t.Fatalf("name = %q, want frontendApi", name)
	}
	if !strings.Contains(output.String(), `managed value name "database" is already configured`) {
		t.Fatalf("output %q does not report duplicate managed value name", output.String())
	}
}
