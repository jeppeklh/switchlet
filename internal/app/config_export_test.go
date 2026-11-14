package app_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func exportDocument(t *testing.T) (app.ConfigEditWorkflow, app.ConfigEditDocument) {
	t.Helper()
	root := t.TempDir()
	text := `# COMMENT_SECRET_SENTINEL
version: 4
targets:
  - name: url
    file: settings.json
    type: json
    jsonPath: url
  - name: enabled
    file: settings.yaml
    type: yaml
    yamlPath: enabled
    valueType: boolean
  - name: count
    file: settings.toml
    type: toml
    tomlPath: count
    valueType: integer
  - name: rate
    file: .env.local
    type: dotenv
    key: RATE
    valueType: number
profiles:
  - name: Local
    protected: true
    values:
      - target: url
        value: LITERAL_SECRET_SENTINEL # INLINE_SECRET_SENTINEL
      - target: enabled
        value: false
      - target: count
        value: 572903
      - target: rate
        value: 0.73121
  - name: Environment
    values:
      - target: url
        valueFromEnv: KEEP_EXPORT_ENV
`
	path := writeConfigFile(t, root, text)
	for _, file := range []string{"settings.json", "settings.yaml", "settings.toml", ".env.local"} {
		if err := os.WriteFile(filepath.Join(root, file), []byte("TARGET_SECRET_SENTINEL"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("KEEP_EXPORT_ENV", "RESOLVED_SECRET_SENTINEL")
	workflow := app.DefaultConfigEditWorkflow()
	return workflow, loadConfigEditDocument(t, workflow, root, path)
}

func TestExampleExport_SanitizesSchemaWithoutMutatingDraft(t *testing.T) {
	workflow, document := exportDocument(t)
	original := readAppTestFile(t, document.ConfigPath)
	draft, err := workflow.NewExampleExportDraft(document)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fmt.Sprint(draft), "SECRET_SENTINEL") {
		t.Fatal("draft exposes secrets")
	}
	for index := range draft.Entries {
		if !draft.Entries[index].ExistingReference {
			draft.Entries[index].EnvironmentName = "EXAMPLE_" + strings.ToUpper(draft.Entries[index].TargetName)
		}
	}
	prepared, err := workflow.PrepareExampleExport(document, draft)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.ReplacesExisting {
		t.Fatal("new example marked as replacement")
	}
	if _, err := os.Stat(prepared.Path); !os.IsNotExist(err) {
		t.Fatal("preparation created example")
	}
	if err := prepared.Commit(nil); err != nil {
		t.Fatal(err)
	}
	contents := readAppTestFile(t, prepared.Path)
	for _, secret := range []string{"SECRET_SENTINEL", "572903", "0.73121", "value: false", document.ProjectRoot} {
		if strings.Contains(string(contents), secret) {
			t.Fatalf("export contains forbidden source data %q", secret)
		}
	}
	received := filepath.Join(t.TempDir(), ".switchlet.yaml")
	if err := os.WriteFile(received, contents, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(received)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != 4 || !loaded.Profiles[0].Protected || len(loaded.Targets) != 4 {
		t.Fatal("schema metadata lost")
	}
	for index, target := range loaded.Targets {
		expected := document.Targets[index]
		if target.Type != expected.Type || target.ValueType != expected.ValueType || filepath.Base(target.File) != filepath.Base(expected.File) {
			t.Fatalf("target changed: %#v", target)
		}
	}
	for _, profile := range loaded.Profiles {
		for _, value := range profile.Values {
			if value.Value != nil || value.ValueFromEnv == nil {
				t.Fatal("literal survived export")
			}
		}
	}
	if *loaded.Profiles[1].Values[0].ValueFromEnv != "KEEP_EXPORT_ENV" {
		t.Fatal("existing reference changed")
	}
	if string(readAppTestFile(t, document.ConfigPath)) != string(original) {
		t.Fatal("local config changed")
	}
	current, err := workflow.LoadDocument(document.ConfigPath)
	if err != nil || !reflect.DeepEqual(current.Profiles, document.Profiles) {
		t.Fatal("source draft mutated")
	}
	for _, target := range document.Targets {
		if string(readAppTestFile(t, target.File)) != "TARGET_SECRET_SENTINEL" {
			t.Fatal("managed file changed")
		}
	}
	if _, err := os.Stat(filepath.Join(document.ProjectRoot, ".gitignore")); !os.IsNotExist(err) {
		t.Fatal("export touched gitignore")
	}
}

func TestExampleExport_NamesSharingAndCompleteMappings(t *testing.T) {
	workflow, document := exportDocument(t)
	initial, err := workflow.NewExampleExportDraft(document)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", "BAD-NAME", "1FIRST", "NAME=value", "BAD NAME", "ø", "NAME\x00"} {
		draft := app.ExampleExportDraft{Entries: append([]app.ExampleExportEntry(nil), initial.Entries...)}
		for index := range draft.Entries {
			if !draft.Entries[index].ExistingReference {
				draft.Entries[index].EnvironmentName = "VALUE_" + draft.Entries[index].TargetName
			}
		}
		draft.Entries[0].EnvironmentName = invalid
		if _, err := workflow.PrepareExampleExport(document, draft); err == nil {
			t.Fatalf("accepted invalid name %q", invalid)
		}
	}
	draft := initial
	for index := range draft.Entries {
		if !draft.Entries[index].ExistingReference {
			draft.Entries[index].EnvironmentName = "KEEP_EXPORT_ENV"
		}
	}
	prepared, err := workflow.PrepareExampleExport(document, draft)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.SharedVariables) != 1 || prepared.SharedVariables[0] != "KEEP_EXPORT_ENV" {
		t.Fatal("missing shared source review")
	}
	prepared.SharedVariables = nil // Public presentation data cannot bypass approval.
	if err := prepared.Commit(nil); err == nil {
		t.Fatal("shared source exported without approval")
	}
	if _, err := os.Stat(prepared.Path); !os.IsNotExist(err) {
		t.Fatal("failed approval wrote output")
	}
	if err := prepared.Commit([]string{"KEEP_EXPORT_ENV"}); err != nil {
		t.Fatal(err)
	}
	draft.Entries[0].EnvironmentName = "keep_export_env"
	if _, err := workflow.PrepareExampleExport(document, draft); err == nil {
		t.Fatal("accepted case-ambiguous names")
	}
	draft.Entries[0].EnvironmentName = "KEEP_EXPORT_ENV"
	draft.Entries[len(draft.Entries)-1].EnvironmentName = "CHANGED"
	if _, err := workflow.PrepareExampleExport(document, draft); err == nil {
		t.Fatal("changed existing environment reference")
	}
	draft.Entries = draft.Entries[:1]
	if _, err := workflow.PrepareExampleExport(document, draft); err == nil {
		t.Fatal("accepted incomplete export")
	}
}

func TestExampleExport_CompatibilityInput(t *testing.T) {
	root := t.TempDir()
	path := writeConfigFile(t, root, "version: 2\ntarget:\n  file: settings.json\n  jsonPath: url\nprofiles:\n  - name: Local\n    value: compatibility-secret\n")
	workflow := app.DefaultConfigEditWorkflow()
	document := loadConfigEditDocument(t, workflow, root, path)
	draft, err := workflow.NewExampleExportDraft(document)
	if err != nil {
		t.Fatal(err)
	}
	draft.Entries[0].EnvironmentName = "COMPAT_URL"
	prepared, err := workflow.PrepareExampleExport(document, draft)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepared.Commit(nil); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(prepared.Path)
	if err != nil || loaded.Version != 3 || len(loaded.Targets) != 1 || *loaded.Profiles[0].Values[0].ValueFromEnv != "COMPAT_URL" {
		t.Fatalf("invalid compatibility export: %#v %v", loaded, err)
	}
}
