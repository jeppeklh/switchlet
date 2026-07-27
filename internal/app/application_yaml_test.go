package app_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestApplication_Profiles_ReturnsYAMLTargetContext(t *testing.T) {
	application := app.NewWithTargets(
		[]config.Target{{Name: "workerQueue", File: "worker/config.yaml", Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"}},
		[]config.Profile{{
			Name: "Local",
			Values: []config.ProfileValue{
				{Target: "workerQueue", Value: stringPointer("local-queue")},
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
	if valueItem.TargetType != config.TargetTypeYAML {
		t.Fatalf("TargetType = %q, want yaml", valueItem.TargetType)
	}
	if valueItem.SelectorName != "yamlPath" || valueItem.Selector != "queue.endpoint" {
		t.Fatalf("selector = %s %q, want yamlPath queue.endpoint", valueItem.SelectorName, valueItem.Selector)
	}
}

func TestApplication_ApplyProfile_AppliesYAMLTarget(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
queue:
  endpoint: old-queue
  retries: 3
`)+"\n")

	application := app.NewWithTargets(
		[]config.Target{{Name: "workerQueue", File: targetPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"}},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "workerQueue", Value: stringPointer("staging-queue")},
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
	if result.TargetPath != "queue.endpoint" {
		t.Fatalf("TargetPath = %q, want queue.endpoint", result.TargetPath)
	}
	if len(result.Changes) != 1 {
		t.Fatalf("len(Changes) = %d, want 1", len(result.Changes))
	}
	change := result.Changes[0]
	if change.TargetType != config.TargetTypeYAML || change.SelectorName != "yamlPath" || change.Selector != "queue.endpoint" {
		t.Fatalf("change = %#v, want YAML planned change", change)
	}

	updatedContents := string(readFile(t, targetPath))
	if !strings.Contains(updatedContents, "endpoint: staging-queue") {
		t.Fatalf("YAML contents = %q, want updated endpoint", updatedContents)
	}
	if !strings.Contains(updatedContents, "retries: 3") {
		t.Fatalf("YAML contents = %q, want unrelated value preserved", updatedContents)
	}
}

func TestApplication_ApplyProfileWithOptions_DryRunValidatesYAMLWithoutWriting(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
queue:
  endpoint: old-queue
  retries: 3
`)+"\n")
	originalContents := readFile(t, targetPath)

	application := app.NewWithTargets(
		[]config.Target{{Name: "workerQueue", File: targetPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"}},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "workerQueue", Value: stringPointer("staging-queue")},
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
	if len(result.Changes) != 1 || result.Changes[0].SelectorName != "yamlPath" || result.Changes[0].Selector != "queue.endpoint" {
		t.Fatalf("Changes = %#v, want YAML dry-run planned change", result.Changes)
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("YAML target changed during dry run")
	}
}

func TestApplication_ApplyProfile_AppliesMixedJSONYAMLDotenvProfile(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	workerPath := writeTargetFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
queue:
  endpoint: old-queue
  retries: 3
`)+"\n")
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "workerQueue", File: workerPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://staging")},
				{Target: "workerQueue", Value: stringPointer("staging-queue")},
				{Target: "frontendApi", Value: stringPointer("https://api.staging.example.test")},
			},
		}},
	)

	result, err := application.ApplyProfileByName("Staging")
	if err != nil {
		t.Fatalf("ApplyProfileByName returned error: %v", err)
	}

	if len(result.Changes) != 3 {
		t.Fatalf("len(Changes) = %d, want 3", len(result.Changes))
	}
	if result.Changes[1].TargetName != "workerQueue" || result.Changes[1].TargetType != config.TargetTypeYAML || result.Changes[1].SelectorName != "yamlPath" {
		t.Fatalf("YAML change = %#v, want workerQueue yamlPath", result.Changes[1])
	}
	if decodeJSONRoot(t, readFile(t, databasePath))["database"].(map[string]any)["url"] != "postgres://staging" {
		t.Fatal("database target was not updated")
	}
	if !strings.Contains(string(readFile(t, workerPath)), "endpoint: staging-queue") {
		t.Fatalf("YAML target = %q, want updated endpoint", string(readFile(t, workerPath)))
	}
	if !strings.Contains(string(readFile(t, frontendPath)), "VITE_API_URL=https://api.staging.example.test") {
		t.Fatalf("dotenv target = %q, want updated API URL", string(readFile(t, frontendPath)))
	}
}

func TestApplication_TargetFailureFromError_ReturnsYAMLSelectorContext(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
queue:
  endpoint: old-queue
`)+"\n")

	application := app.NewWithTargets(
		[]config.Target{{Name: "workerQueue", File: targetPath, Type: config.TargetTypeYAML, YAMLPath: "queue.missing"}},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "workerQueue", Value: stringPointer("secret-queue-value")},
			},
		}},
	)

	_, err := application.ApplyProfileByName("Staging")
	if err == nil {
		t.Fatal("ApplyProfileByName returned nil error, want YAML target failure")
	}

	failure, ok := app.TargetFailureFromError(err)
	if !ok {
		t.Fatalf("TargetFailureFromError(%v) returned ok=false, want YAML target failure", err)
	}
	if failure.TargetName != "workerQueue" || failure.TargetType != config.TargetTypeYAML {
		t.Fatalf("failure = %#v, want workerQueue YAML target", failure)
	}
	if failure.SelectorName != "yamlPath" || failure.Selector != "queue.missing" {
		t.Fatalf("failure selector = %s %q, want yamlPath queue.missing", failure.SelectorName, failure.Selector)
	}
	if strings.Contains(err.Error(), "secret-queue-value") || strings.Contains(failure.Reason, "secret-queue-value") {
		t.Fatalf("YAML target failure leaked resolved replacement value: err=%q reason=%q", err, failure.Reason)
	}
}
