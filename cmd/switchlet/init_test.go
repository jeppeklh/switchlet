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
	if !strings.Contains(output.String(), "--dry-run") {
		t.Fatalf("help output %q does not mention --dry-run", output.String())
	}
	if !strings.Contains(output.String(), "--allow-protected") {
		t.Fatalf("help output %q does not mention --allow-protected", output.String())
	}
	if !strings.Contains(output.String(), "--json") {
		t.Fatalf("help output %q does not mention --json", output.String())
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
	if !strings.Contains(output.String(), "guides you through file selection") {
		t.Fatalf("help output %q does not describe guided file discovery", output.String())
	}
	if !strings.Contains(output.String(), "narrow large file lists") {
		t.Fatalf("help output %q does not mention large-repository narrowing", output.String())
	}
	if !strings.Contains(output.String(), "search existing string-valued JSON paths") {
		t.Fatalf("help output %q does not mention JSON-path search", output.String())
	}
}

func TestPromptTargetJSONPath_AllowsSearchingLargeSelectablePathSets(t *testing.T) {
	selectedFile := targetFileSelection{
		path:        "/tmp/config.json",
		displayPath: "config.json",
		nodes: []editor.StringTargetNode{
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
		},
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
	if !strings.Contains(output.String(), `Select target JSON path matching "replicaL" in config.json:`) {
		t.Fatalf("prompt output %q does not report the filtered JSON-path selection", output.String())
	}
}

func TestPromptTargetJSONPath_SearchKeepsPathSelectionRecoverableWhenNothingMatches(t *testing.T) {
	selectedFile := targetFileSelection{
		path:        "/tmp/config.json",
		displayPath: "config.json",
		nodes: []editor.StringTargetNode{{
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
		}},
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
		createConfig: func(string, config.Target, []config.Profile) (string, config.Config, error) {
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
	if !strings.Contains(outputText, "Target file: src/MyApplication/appsettings.Development.json") {
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
		createConfig: func(string, config.Target, []config.Profile) (string, config.Config, error) {
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
	if !strings.Contains(outputText, `No discovered target JSON files match "missing".`) {
		t.Fatalf("init output %q does not report the no-match filter state", outputText)
	}
	if !strings.Contains(outputText, "Target file: src/MyApplication/appsettings.Development.json") {
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
		"Local",
		"1",
		"postgres://localhost:5432/myapp",
		"n",
		"n",
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
	if loadedConfig.Version != 2 {
		t.Fatalf("Version = %d, want 2", loadedConfig.Version)
	}
	if loadedConfig.Target.JSONPath != "database.primary.url" {
		t.Fatalf("JSON path = %q, want %q", loadedConfig.Target.JSONPath, "database.primary.url")
	}
	if len(loadedConfig.Profiles) != 1 {
		t.Fatalf("len(profiles) = %d, want 1", len(loadedConfig.Profiles))
	}
	if loadedConfig.Profiles[0].Name != "Local" {
		t.Fatalf("profile name = %q, want %q", loadedConfig.Profiles[0].Name, "Local")
	}
	if loadedConfig.Profiles[0].Value == nil || *loadedConfig.Profiles[0].Value != "postgres://localhost:5432/myapp" {
		t.Fatalf("literal profile value = %#v, want configured literal value", loadedConfig.Profiles[0].Value)
	}

	contents, err := os.ReadFile(filepath.Join(projectRoot, ".switchlet.yaml"))
	if err != nil {
		t.Fatalf("read configuration file: %v", err)
	}
	if !strings.Contains(string(contents), "jsonPath: database.primary.url") {
		t.Fatalf("configuration file contents %q do not contain version 2 JSON path", string(contents))
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
	if !strings.Contains(output.String(), "Step 4 of 4: Review and create configuration") {
		t.Fatalf("init output %q does not include the final review step", output.String())
	}
	if !strings.Contains(output.String(), "Create .switchlet.yaml now? [Y/n]: ") {
		t.Fatalf("init output %q does not show the Enter-as-yes confirmation prompt", output.String())
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
		"Local",
		"1",
		"postgres://localhost:5432/myapp",
		"n",
		"n",
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
	if strings.Count(output.String(), "Select target JSON file:") != 1 {
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
	if !strings.Contains(output.String(), "No target JSON files with selectable string values were discovered") {
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
	if strings.Count(output.String(), "Select target JSON file:") != 2 {
		t.Fatalf("init output %q should prompt for the target file twice", output.String())
	}
	if !strings.Contains(output.String(), "Target file: b.json") {
		t.Fatalf("init output %q does not summarize the second selected target file", output.String())
	}
	if !strings.Contains(output.String(), "Target JSON path: database.primary.url") {
		t.Fatalf("init output %q does not summarize the selected JSON path", output.String())
	}

	_, statErr := os.Stat(filepath.Join(projectRoot, ".switchlet.yaml"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("configuration file stat error = %v, want not-exist error after cancellation", statErr)
	}
}
