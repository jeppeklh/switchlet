package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestConfigEditWorkflow_LoadDocument_NormalizesCompatibilityConfigAndReportsConversion(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := writeConfigFile(t, projectRoot, strings.TrimSpace(`
version: 2

target:
  file: config.json
  jsonPath: service.baseUrl

profiles:
  - name: Local
    value: https://local.example.test
`)+"\n")

	document, err := app.DefaultConfigEditWorkflow().LoadDocument(configPath)
	if err != nil {
		t.Fatalf("LoadDocument returned error: %v", err)
	}

	if !document.ConvertsToVersionThree {
		t.Fatal("ConvertsToVersionThree = false, want true for compatibility config")
	}
	if document.OriginalVersion != 2 {
		t.Fatalf("OriginalVersion = %d, want 2", document.OriginalVersion)
	}
	if document.Targets[0].Name != "default" || document.Profiles[0].Values[0].Target != "default" {
		t.Fatalf("document = %#v, want normalized default target references", document)
	}
	if document.Profiles[0].Value != nil {
		t.Fatal("document profile retained compatibility top-level Value, want Version 3 draft values only")
	}

	changes := app.DefaultConfigEditWorkflow().SummarizeChanges(document)
	if !containsConfigEditChange(changes, app.ConfigEditChangeCompatibilityConversion) {
		t.Fatalf("changes = %#v, want compatibility conversion warning", changes)
	}
}

func TestConfigEditWorkflow_UpdateProfileSummarizesWithoutRawValues(t *testing.T) {
	workflow := app.DefaultConfigEditWorkflow()
	projectRoot, configPath := singleTargetConfig(t, "https://local.example.test")
	document := loadConfigEditDocument(t, workflow, projectRoot, configPath)

	updatedDocument, err := workflow.UpdateProfile(document, "Local", config.Profile{
		Name:      "Local",
		Protected: true,
		Values: []config.ProfileValue{{
			Target: "service",
			Value:  stringPointer("https://super-secret.example.test"),
		}},
	})
	if err != nil {
		t.Fatalf("UpdateProfile returned error: %v", err)
	}

	changes := workflow.SummarizeChanges(updatedDocument)
	if !containsConfigEditChange(changes, app.ConfigEditChangeProfileUpdated) {
		t.Fatalf("changes = %#v, want profile updated change", changes)
	}
	if strings.Contains(configEditChangesString(changes), "super-secret") {
		t.Fatalf("changes = %#v, must not contain raw profile value", changes)
	}
}

func TestConfigEditWorkflow_AddAndRemoveProfiles(t *testing.T) {
	workflow := app.DefaultConfigEditWorkflow()
	projectRoot, configPath := twoProfileConfig(t)
	document := loadConfigEditDocument(t, workflow, projectRoot, configPath)

	updatedDocument, err := workflow.AddProfile(document, config.Profile{
		Name: "Staging",
		Values: []config.ProfileValue{{
			Target: "service",
			Value:  stringPointer("https://staging.example.test"),
		}},
	})
	if err != nil {
		t.Fatalf("AddProfile returned error: %v", err)
	}
	updatedDocument, err = workflow.RemoveProfile(updatedDocument, "Local")
	if err != nil {
		t.Fatalf("RemoveProfile returned error: %v", err)
	}

	if err := workflow.ValidateDraft(updatedDocument); err != nil {
		t.Fatalf("ValidateDraft returned error: %v", err)
	}
	changes := workflow.SummarizeChanges(updatedDocument)
	if !containsConfigEditChange(changes, app.ConfigEditChangeProfileAdded) {
		t.Fatalf("changes = %#v, want profile added change", changes)
	}
	if !containsConfigEditChange(changes, app.ConfigEditChangeProfileRemoved) {
		t.Fatalf("changes = %#v, want profile removed change", changes)
	}
}

func TestConfigEditWorkflow_AddProfileDraftSupportsPartialProtectedEnvironmentProfile(t *testing.T) {
	workflow := app.DefaultConfigEditWorkflow()
	projectRoot, configPath := multiTargetConfig(t)
	document := loadConfigEditDocument(t, workflow, projectRoot, configPath)
	draft := workflow.NewProfileDraft(document)
	draft.Name = "Staging API"
	draft.Protected = true
	draft.Values[1].Included = true
	draft.Values[1].Source = app.ProfileSourceEnvironment
	draft.Values[1].EnvironmentVariableName = "STAGING_API_URL"

	updatedDocument, err := workflow.AddProfileDraft(document, draft)
	if err != nil {
		t.Fatalf("AddProfileDraft returned error: %v", err)
	}

	addedProfile := updatedDocument.Profiles[len(updatedDocument.Profiles)-1]
	if !addedProfile.Protected {
		t.Fatal("added profile Protected = false, want true")
	}
	if len(addedProfile.Values) != 1 || addedProfile.Values[0].Target != "frontendApi" || addedProfile.Values[0].ValueFromEnv == nil || *addedProfile.Values[0].ValueFromEnv != "STAGING_API_URL" {
		t.Fatalf("added profile values = %#v, want one environment-backed frontendApi value", addedProfile.Values)
	}
}

func TestConfigEditWorkflow_ProfileDraftRejectsDuplicateEmptyAndMissingValues(t *testing.T) {
	workflow := app.DefaultConfigEditWorkflow()
	projectRoot, configPath := twoProfileConfig(t)
	document := loadConfigEditDocument(t, workflow, projectRoot, configPath)

	draft := workflow.NewProfileDraft(document)
	draft.Name = "Local"
	draft.Values[0].Included = true
	draft.Values[0].LiteralValue = "https://duplicate.example.test"
	if _, err := workflow.AddProfileDraft(document, draft); err == nil || !strings.Contains(err.Error(), "duplicate profile name") {
		t.Fatalf("AddProfileDraft duplicate returned %v, want duplicate profile error", err)
	}

	draft = workflow.NewProfileDraft(document)
	draft.Name = "  "
	draft.Values[0].Included = true
	draft.Values[0].LiteralValue = "https://empty-name.example.test"
	if _, err := workflow.AddProfileDraft(document, draft); err == nil || !strings.Contains(err.Error(), "profile name must be set") {
		t.Fatalf("AddProfileDraft empty name returned %v, want profile name error", err)
	}

	draft = workflow.NewProfileDraft(document)
	draft.Name = "No Values"
	draft.Values[0].Included = false
	if _, err := workflow.AddProfileDraft(document, draft); err == nil || !strings.Contains(err.Error(), "must include at least one managed value") {
		t.Fatalf("AddProfileDraft no values returned %v, want managed value error", err)
	}
}

func TestConfigEditWorkflow_RemoveLastProfileBlocksSave(t *testing.T) {
	workflow := app.DefaultConfigEditWorkflow()
	projectRoot, configPath := singleTargetConfig(t, "https://local.example.test")
	document := loadConfigEditDocument(t, workflow, projectRoot, configPath)

	updatedDocument, err := workflow.RemoveProfile(document, "Local")
	if err != nil {
		t.Fatalf("RemoveProfile returned error: %v", err)
	}
	if workflow.IsSaveable(updatedDocument) {
		t.Fatal("IsSaveable = true, want false after removing the last profile")
	}
	_, err = workflow.PrepareSave(updatedDocument)
	if err == nil || !strings.Contains(err.Error(), "at least one profile must be configured") {
		t.Fatalf("PrepareSave returned %v, want missing profile validation error", err)
	}
}

func TestConfigEditWorkflow_RenameManagedValueUpdatesProfileReferencesAndSummary(t *testing.T) {
	workflow := app.DefaultConfigEditWorkflow()
	projectRoot, configPath := twoProfileConfig(t)
	document := loadConfigEditDocument(t, workflow, projectRoot, configPath)

	updatedDocument, result, err := workflow.RenameManagedValue(document, "service", "backendService")
	if err != nil {
		t.Fatalf("RenameManagedValue returned error: %v", err)
	}

	if updatedDocument.Targets[0].Name != "backendService" {
		t.Fatalf("managed value name = %q, want backendService", updatedDocument.Targets[0].Name)
	}
	for _, profile := range updatedDocument.Profiles {
		if profile.Values[0].Target != "backendService" {
			t.Fatalf("profile %q value target = %q, want backendService", profile.Name, profile.Values[0].Target)
		}
	}
	if strings.Join(result.UpdatedProfiles, ",") != "Local,Test" {
		t.Fatalf("UpdatedProfiles = %#v, want Local and Test", result.UpdatedProfiles)
	}

	changes := workflow.SummarizeChanges(updatedDocument)
	if !containsConfigEditChange(changes, app.ConfigEditChangeManagedValueRenamed) {
		t.Fatalf("changes = %#v, want managed-value rename change", changes)
	}
	if !strings.Contains(configEditChangesString(changes), "Updated profile references in: Local, Test") {
		t.Fatalf("changes = %#v, want affected profile references", changes)
	}
}

func TestConfigEditWorkflow_RemoveManagedValueRemovesProfileValuesAndBlocksInvalidSave(t *testing.T) {
	workflow := app.DefaultConfigEditWorkflow()
	projectRoot, configPath := multiTargetConfig(t)
	document := loadConfigEditDocument(t, workflow, projectRoot, configPath)

	updatedDocument, result, err := workflow.RemoveManagedValue(document, "frontendApi")
	if err != nil {
		t.Fatalf("RemoveManagedValue returned error: %v", err)
	}

	if strings.Join(result.AffectedProfiles, ",") != "Local,Frontend Only" {
		t.Fatalf("AffectedProfiles = %#v, want Local and Frontend Only", result.AffectedProfiles)
	}
	if strings.Join(result.InvalidProfiles, ",") != "Frontend Only" {
		t.Fatalf("InvalidProfiles = %#v, want Frontend Only", result.InvalidProfiles)
	}
	if workflow.IsSaveable(updatedDocument) {
		t.Fatal("IsSaveable = true, want false for profile with no values")
	}

	_, err = workflow.PrepareSave(updatedDocument)
	if err == nil {
		t.Fatal("PrepareSave returned nil error, want invalid draft error")
	}
	if !strings.Contains(err.Error(), `profile "Frontend Only" must include at least one value`) {
		t.Fatalf("PrepareSave returned error %q, want empty profile value error", err)
	}
	changes := workflow.SummarizeChanges(updatedDocument)
	if !containsConfigEditChange(changes, app.ConfigEditChangeManagedValueRemoved) {
		t.Fatalf("changes = %#v, want managed-value removal change", changes)
	}
}

func TestConfigEditWorkflow_PrepareSaveValidatesTargetsAndCommitsConfigReplacement(t *testing.T) {
	workflow := app.DefaultConfigEditWorkflow()
	projectRoot, configPath := singleTargetConfig(t, "https://local.example.test")
	document := loadConfigEditDocument(t, workflow, projectRoot, configPath)
	originalContents := readAppTestFile(t, configPath)

	updatedDocument, err := workflow.UpdateProfile(document, "Local", config.Profile{
		Name: "Local",
		Values: []config.ProfileValue{{
			Target: "service",
			Value:  stringPointer("https://updated.example.test"),
		}},
	})
	if err != nil {
		t.Fatalf("UpdateProfile returned error: %v", err)
	}

	preparedSave, err := workflow.PrepareSave(updatedDocument)
	if err != nil {
		t.Fatalf("PrepareSave returned error: %v", err)
	}
	if preparedSave.ConfigPath != configPath {
		t.Fatalf("ConfigPath = %q, want %q", preparedSave.ConfigPath, configPath)
	}
	if !containsConfigEditChange(preparedSave.Changes, app.ConfigEditChangeProfileUpdated) {
		t.Fatalf("prepared changes = %#v, want profile update", preparedSave.Changes)
	}
	if string(readAppTestFile(t, configPath)) != string(originalContents) {
		t.Fatal("PrepareSave modified .switchlet.yaml before Commit")
	}
	if err := preparedSave.Commit(); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	contents := string(readAppTestFile(t, configPath))
	if !strings.Contains(contents, "https://updated.example.test") || strings.Contains(contents, "https://local.example.test") {
		t.Fatalf("configuration contents = %q, want committed profile update", contents)
	}
}

func TestConfigEditWorkflow_PrepareSavePropagatesStaleConfigError(t *testing.T) {
	workflow := app.DefaultConfigEditWorkflow()
	projectRoot, configPath := singleTargetConfig(t, "https://local.example.test")
	document := loadConfigEditDocument(t, workflow, projectRoot, configPath)
	updatedDocument, err := workflow.UpdateProfile(document, "Local", config.Profile{
		Name: "Local",
		Values: []config.ProfileValue{{
			Target: "service",
			Value:  stringPointer("https://updated.example.test"),
		}},
	})
	if err != nil {
		t.Fatalf("UpdateProfile returned error: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(strings.ReplaceAll(string(readAppTestFile(t, configPath)), "Local", "Changed")), 0o644); err != nil {
		t.Fatalf("modify configuration after load: %v", err)
	}

	_, err = workflow.PrepareSave(updatedDocument)
	if err == nil {
		t.Fatal("PrepareSave returned nil error, want stale config error")
	}
	if !errors.Is(err, config.ErrConfigChanged) {
		t.Fatalf("PrepareSave returned error %v, want ErrConfigChanged", err)
	}
}

func singleTargetConfig(t *testing.T, profileValue string) (string, string) {
	t.Helper()

	projectRoot := t.TempDir()
	writeTargetFile(t, projectRoot, "config.json", `{"service":{"baseUrl":"https://old.example.test"}}`)
	configPath := writeConfigFile(t, projectRoot, strings.TrimSpace(`
version: 3

targets:
  - name: service
    file: config.json
    type: json
    jsonPath: service.baseUrl

profiles:
  - name: Local
    values:
      - target: service
        value: `+profileValue+`
`)+"\n")

	return projectRoot, configPath
}

func twoProfileConfig(t *testing.T) (string, string) {
	t.Helper()

	projectRoot, configPath := singleTargetConfig(t, "https://local.example.test")
	contents := strings.TrimSpace(`
version: 3

targets:
  - name: service
    file: config.json
    type: json
    jsonPath: service.baseUrl

profiles:
  - name: Local
    values:
      - target: service
        value: https://local.example.test
  - name: Test
    protected: true
    values:
      - target: service
        valueFromEnv: TEST_SERVICE_URL
`) + "\n"
	if err := os.WriteFile(configPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write two-profile config: %v", err)
	}

	return projectRoot, configPath
}

func multiTargetConfig(t *testing.T) (string, string) {
	t.Helper()

	projectRoot := t.TempDir()
	writeTargetFile(t, projectRoot, "config.json", `{"service":{"baseUrl":"https://old.example.test"}}`)
	writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")
	configPath := writeConfigFile(t, projectRoot, strings.TrimSpace(`
version: 3

targets:
  - name: service
    file: config.json
    type: json
    jsonPath: service.baseUrl
  - name: frontendApi
    file: frontend/.env.local
    type: dotenv
    key: VITE_API_URL

profiles:
  - name: Local
    values:
      - target: service
        value: https://local.example.test
      - target: frontendApi
        value: http://localhost:5173
  - name: Frontend Only
    values:
      - target: frontendApi
        value: https://frontend.example.test
`)+"\n")

	return projectRoot, configPath
}

func loadConfigEditDocument(t *testing.T, workflow app.ConfigEditWorkflow, projectRoot string, configPath string) app.ConfigEditDocument {
	t.Helper()

	document, err := workflow.LoadDocument(configPath)
	if err != nil {
		t.Fatalf("LoadDocument returned error: %v", err)
	}
	if document.ProjectRoot != projectRoot {
		t.Fatalf("ProjectRoot = %q, want %q", document.ProjectRoot, projectRoot)
	}

	return document
}

func writeConfigFile(t *testing.T, projectRoot string, contents string) string {
	t.Helper()

	configPath := filepath.Join(projectRoot, ".switchlet.yaml")
	if err := os.WriteFile(configPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write configuration file %q: %v", configPath, err)
	}

	return configPath
}

func readAppTestFile(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %q: %v", path, err)
	}

	return contents
}

func containsConfigEditChange(changes []app.ConfigEditChange, kind app.ConfigEditChangeKind) bool {
	for _, change := range changes {
		if change.Kind == kind {
			return true
		}
	}

	return false
}

func configEditChangesString(changes []app.ConfigEditChange) string {
	var builder strings.Builder
	for _, change := range changes {
		builder.WriteString(change.Summary)
		for _, detail := range change.Detail {
			builder.WriteString("\n")
			builder.WriteString(detail)
		}
	}

	return builder.String()
}
