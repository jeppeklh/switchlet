package app_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestApplication_Profiles_ReturnsTOMLTargetContext(t *testing.T) {
	application := app.NewWithTargets(
		[]config.Target{{Name: "serviceEndpoint", File: "services/development.toml", Type: config.TargetTypeTOML, TOMLPath: "services.api.endpoint"}},
		[]config.Profile{{
			Name: "Local",
			Values: []config.ProfileValue{
				{Target: "serviceEndpoint", Value: stringPointer("http://localhost:8080")},
			},
		}},
	)

	items := application.Profiles()
	if len(items) != 1 {
		t.Fatalf("len(Profiles()) = %d, want 1", len(items))
	}
	if len(items[0].Values) != 1 {
		t.Fatalf("len(Values) = %d, want 1", len(items[0].Values))
	}

	valueItem := items[0].Values[0]
	if valueItem.TargetType != config.TargetTypeTOML {
		t.Fatalf("TargetType = %q, want toml", valueItem.TargetType)
	}
	if valueItem.SelectorName != "tomlPath" || valueItem.Selector != "services.api.endpoint" {
		t.Fatalf("selector = %s %q, want tomlPath services.api.endpoint", valueItem.SelectorName, valueItem.Selector)
	}
}

func TestApplication_Profiles_ReturnsUnavailableTOMLProfileValue(t *testing.T) {
	t.Setenv("STAGING_SERVICE_ENDPOINT", "")

	application := app.NewWithTargets(
		[]config.Target{{Name: "serviceEndpoint", File: "services/development.toml", Type: config.TargetTypeTOML, TOMLPath: "services.api.endpoint"}},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "serviceEndpoint", ValueFromEnv: stringPointer("STAGING_SERVICE_ENDPOINT")},
			},
		}},
	)

	items := application.Profiles()
	if len(items) != 1 {
		t.Fatalf("len(Profiles()) = %d, want 1", len(items))
	}
	if items[0].Available {
		t.Fatal("Available = true, want false")
	}
	if len(items[0].Values) != 1 {
		t.Fatalf("len(Values) = %d, want 1", len(items[0].Values))
	}
	valueItem := items[0].Values[0]
	if valueItem.TargetType != config.TargetTypeTOML || valueItem.SelectorName != "tomlPath" {
		t.Fatalf("value item = %#v, want TOML target context", valueItem)
	}
	if valueItem.EnvironmentVariableName != "STAGING_SERVICE_ENDPOINT" {
		t.Fatalf("EnvironmentVariableName = %q, want STAGING_SERVICE_ENDPOINT", valueItem.EnvironmentVariableName)
	}
	if !strings.Contains(valueItem.UnavailableReason, "STAGING_SERVICE_ENDPOINT") {
		t.Fatalf("UnavailableReason = %q, want environment variable name", valueItem.UnavailableReason)
	}
}

func TestApplication_ApplyProfile_AppliesTOMLTarget(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "services/development.toml", strings.TrimSpace(`
[services.api]
endpoint = "http://old.example.test"
retries = 3
`)+"\n")

	application := app.NewWithTargets(
		[]config.Target{{Name: "serviceEndpoint", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "services.api.endpoint"}},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "serviceEndpoint", Value: stringPointer("https://api.staging.example.test")},
			},
		}},
	)

	result, err := application.ApplyProfileByName("Staging")
	if err != nil {
		t.Fatalf("ApplyProfileByName returned error: %v", err)
	}

	if result.ProfileName != "Staging" {
		t.Fatalf("ProfileName = %q, want Staging", result.ProfileName)
	}
	if result.TargetPath != "services.api.endpoint" {
		t.Fatalf("TargetPath = %q, want services.api.endpoint", result.TargetPath)
	}
	if len(result.Changes) != 1 {
		t.Fatalf("len(Changes) = %d, want 1", len(result.Changes))
	}
	change := result.Changes[0]
	if change.TargetType != config.TargetTypeTOML || change.SelectorName != "tomlPath" || change.Selector != "services.api.endpoint" {
		t.Fatalf("change = %#v, want TOML planned change", change)
	}

	updatedContents := string(readFile(t, targetPath))
	if !strings.Contains(updatedContents, `endpoint = "https://api.staging.example.test"`) {
		t.Fatalf("TOML contents = %q, want updated endpoint", updatedContents)
	}
	if !strings.Contains(updatedContents, "retries = 3") {
		t.Fatalf("TOML contents = %q, want unrelated value preserved", updatedContents)
	}
}

func TestApplication_ApplyProfileWithOptions_DryRunValidatesTOMLWithoutWriting(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "services/development.toml", strings.TrimSpace(`
[services.api]
endpoint = "http://old.example.test"
retries = 3
`)+"\n")
	originalContents := readFile(t, targetPath)

	application := app.NewWithTargets(
		[]config.Target{{Name: "serviceEndpoint", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "services.api.endpoint"}},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "serviceEndpoint", Value: stringPointer("https://api.staging.example.test")},
			},
		}},
	)

	result, err := application.ApplyProfileByNameWithOptions("Staging", app.ApplyOptions{DryRun: true})
	if err != nil {
		t.Fatalf("ApplyProfileByNameWithOptions returned error: %v", err)
	}
	if !result.DryRun {
		t.Fatal("DryRun = false, want true")
	}
	if len(result.Changes) != 1 || result.Changes[0].SelectorName != "tomlPath" || result.Changes[0].Selector != "services.api.endpoint" {
		t.Fatalf("Changes = %#v, want TOML dry-run planned change", result.Changes)
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("TOML target changed during dry run")
	}
}

func TestApplication_ApplyProfile_AppliesMixedJSONYAMLTOMLDotenvProfile(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	workerPath := writeTargetFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
queue:
  endpoint: old-queue
  retries: 3
`)+"\n")
	servicePath := writeTargetFile(t, projectRoot, "services/development.toml", strings.TrimSpace(`
[services.api]
endpoint = "http://old.example.test"
retries = 3
`)+"\n")
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "workerQueue", File: workerPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
			{Name: "serviceEndpoint", File: servicePath, Type: config.TargetTypeTOML, TOMLPath: "services.api.endpoint"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://staging")},
				{Target: "workerQueue", Value: stringPointer("staging-queue")},
				{Target: "serviceEndpoint", Value: stringPointer("https://api.staging.example.test")},
				{Target: "frontendApi", Value: stringPointer("https://frontend.staging.example.test")},
			},
		}},
	)

	result, err := application.ApplyProfileByName("Staging")
	if err != nil {
		t.Fatalf("ApplyProfileByName returned error: %v", err)
	}

	if len(result.Changes) != 4 {
		t.Fatalf("len(Changes) = %d, want 4", len(result.Changes))
	}
	if result.Changes[2].TargetName != "serviceEndpoint" || result.Changes[2].TargetType != config.TargetTypeTOML || result.Changes[2].SelectorName != "tomlPath" {
		t.Fatalf("TOML change = %#v, want serviceEndpoint tomlPath", result.Changes[2])
	}
	if decodeJSONRoot(t, readFile(t, databasePath))["database"].(map[string]any)["url"] != "postgres://staging" {
		t.Fatal("database target was not updated")
	}
	if !strings.Contains(string(readFile(t, workerPath)), "endpoint: staging-queue") {
		t.Fatalf("YAML target = %q, want updated endpoint", string(readFile(t, workerPath)))
	}
	if !strings.Contains(string(readFile(t, servicePath)), `endpoint = "https://api.staging.example.test"`) {
		t.Fatalf("TOML target = %q, want updated endpoint", string(readFile(t, servicePath)))
	}
	if !strings.Contains(string(readFile(t, frontendPath)), "VITE_API_URL=https://frontend.staging.example.test") {
		t.Fatalf("dotenv target = %q, want updated API URL", string(readFile(t, frontendPath)))
	}
}

func TestApplication_ApplyProfile_TOMLPartialProfilesModifyOnlyIncludedTargets(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	servicePath := writeTargetFile(t, projectRoot, "services/development.toml", strings.TrimSpace(`
[services.api]
endpoint = "http://old.example.test"
retries = 3
`)+"\n")
	originalDatabaseContents := readFile(t, databasePath)
	originalServiceContents := readFile(t, servicePath)

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "serviceEndpoint", File: servicePath, Type: config.TargetTypeTOML, TOMLPath: "services.api.endpoint"},
		},
		[]config.Profile{
			{
				Name: "Service Endpoint Only",
				Values: []config.ProfileValue{
					{Target: "serviceEndpoint", Value: stringPointer("https://api.partial.example.test")},
				},
			},
			{
				Name: "Database Only",
				Values: []config.ProfileValue{
					{Target: "database", Value: stringPointer("postgres://new")},
				},
			},
		},
	)

	serviceResult, err := application.ApplyProfileByName("Service Endpoint Only")
	if err != nil {
		t.Fatalf("ApplyProfileByName returned error: %v", err)
	}
	if len(serviceResult.Changes) != 1 || serviceResult.Changes[0].TargetName != "serviceEndpoint" || serviceResult.Changes[0].SelectorName != "tomlPath" {
		t.Fatalf("Changes = %#v, want only TOML target", serviceResult.Changes)
	}
	updatedServiceContents := readFile(t, servicePath)
	if !strings.Contains(string(updatedServiceContents), `endpoint = "https://api.partial.example.test"`) {
		t.Fatalf("TOML target = %q, want updated endpoint", string(updatedServiceContents))
	}
	if !bytes.Equal(readFile(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed after TOML-only partial profile")
	}

	_, err = application.ApplyProfileByName("Database Only")
	if err != nil {
		t.Fatalf("ApplyProfileByName returned error: %v", err)
	}
	if bytes.Equal(readFile(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target was not updated")
	}
	if !bytes.Equal(readFile(t, servicePath), updatedServiceContents) {
		t.Fatal("TOML target changed after profile that omitted TOML")
	}
	if bytes.Equal(updatedServiceContents, originalServiceContents) {
		t.Fatal("TOML target was not updated by TOML-only partial profile")
	}
}

func TestApplication_ApplyProfileWithOptions_ProtectedTOMLProfileRequiresOptIn(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "services/development.toml", "[services.api]\nendpoint = \"http://old.example.test\"\n")
	originalContents := readFile(t, targetPath)

	application := app.NewWithTargets(
		[]config.Target{{Name: "serviceEndpoint", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "services.api.endpoint"}},
		[]config.Profile{{
			Name:      "Production",
			Protected: true,
			Values: []config.ProfileValue{
				{Target: "serviceEndpoint", Value: stringPointer("https://api.production.example.test")},
			},
		}},
	)

	_, err := application.ApplyProfileByNameWithOptions("Production", app.ApplyOptions{})
	if err == nil {
		t.Fatal("ApplyProfileByNameWithOptions returned nil error, want protected-profile error")
	}
	if !errors.Is(err, app.ErrProtectedProfileRequiresApproval) {
		t.Fatalf("ApplyProfileByNameWithOptions returned error %v, want ErrProtectedProfileRequiresApproval", err)
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("TOML target changed after protected-profile refusal")
	}
}

func TestApplication_TargetFailureFromError_ReturnsTOMLSelectorContext(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "services/development.toml", strings.TrimSpace(`
[services.api]
retries = 3
`)+"\n")

	application := app.NewWithTargets(
		[]config.Target{{Name: "serviceEndpoint", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "services.api.endpoint"}},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "serviceEndpoint", Value: stringPointer("https://secret.example.test")},
			},
		}},
	)

	_, err := application.ApplyProfileByName("Staging")
	if err == nil {
		t.Fatal("ApplyProfileByName returned nil error, want TOML target failure")
	}

	failure, ok := app.TargetFailureFromError(err)
	if !ok {
		t.Fatalf("TargetFailureFromError(%v) returned ok=false, want TOML target failure", err)
	}
	if failure.TargetName != "serviceEndpoint" || failure.TargetType != config.TargetTypeTOML {
		t.Fatalf("failure = %#v, want serviceEndpoint TOML target", failure)
	}
	if failure.SelectorName != "tomlPath" || failure.Selector != "services.api.endpoint" {
		t.Fatalf("failure selector = %s %q, want tomlPath services.api.endpoint", failure.SelectorName, failure.Selector)
	}
	if strings.Contains(err.Error(), "secret.example.test") || strings.Contains(failure.Reason, "secret.example.test") {
		t.Fatalf("TOML target failure leaked resolved replacement value: err=%q reason=%q", err, failure.Reason)
	}
}

func TestInitWorkflow_InspectTargetFileCandidateSupportsTOML(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "services/development.toml", strings.TrimSpace(`
[services.api]
endpoint = "http://old.example.test"
retries = 3
`)+"\n")

	selection, err := app.DefaultInitWorkflow().InspectTargetFileCandidate(app.InitTargetFileCandidate{
		Path:         targetPath,
		RelativePath: "services/development.toml",
		Type:         app.InitTargetTypeTOML,
	})
	if err != nil {
		t.Fatalf("InspectTargetFileCandidate returned error: %v", err)
	}

	if selection.TargetType != app.InitTargetTypeTOML {
		t.Fatalf("TargetType = %q, want toml", selection.TargetType)
	}
	if len(selection.TOMLNodes) != 1 || selection.TOMLNodes[0].TOMLPath != "services" {
		t.Fatalf("TOMLNodes = %#v, want services root", selection.TOMLNodes)
	}
	apiNodes := selection.TOMLNodes[0].Children
	if len(apiNodes) != 1 || apiNodes[0].TOMLPath != "services.api" {
		t.Fatalf("api TOML nodes = %#v, want services.api", apiNodes)
	}
	endpointNodes := apiNodes[0].Children
	if len(endpointNodes) != 1 || endpointNodes[0].TOMLPath != "services.api.endpoint" || !endpointNodes[0].Selectable {
		t.Fatalf("endpoint TOML nodes = %#v, want selectable endpoint", endpointNodes)
	}
}

func TestInitWorkflow_ValidateTOMLTargetUsesTOMLSelectorRules(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "services/development.toml", strings.TrimSpace(`
[services.api]
endpoint = "http://old.example.test"
retries = 3
`)+"\n")

	workflow := app.DefaultInitWorkflow()
	if err := workflow.ValidateTOMLTarget(targetPath, "services.api.endpoint"); err != nil {
		t.Fatalf("ValidateTOMLTarget returned error: %v", err)
	}

	err := workflow.ValidateTOMLTarget(targetPath, "services.api.retries")
	if err == nil {
		t.Fatal("ValidateTOMLTarget returned nil error, want non-string error")
	}
	if !strings.Contains(err.Error(), `TOML path "services.api.retries" must resolve to a string`) {
		t.Fatalf("ValidateTOMLTarget returned error %q, want TOML selector context", err)
	}
}
