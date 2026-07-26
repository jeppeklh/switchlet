package app_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/profile"
)

func TestApplication_ApplyProfile_AppliesLiteralProfile(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "primary": {
      "url": "postgres://old"
    }
  },
  "AllowedHosts": "*"
}
`)+"\n")

	application := app.New(
		config.Target{File: targetPath, JSONPath: "database.primary.url"},
		[]config.Profile{{Name: "Local", Value: stringPointer("postgres://new")}},
	)
	result, err := application.ApplyProfileByName("Local")
	if err != nil {
		t.Fatalf("ApplyProfileByName returned error: %v", err)
	}

	if result.ProfileName != "Local" {
		t.Fatalf("ProfileName = %q, want %q", result.ProfileName, "Local")
	}
	if result.TargetPath != "database.primary.url" {
		t.Fatalf("TargetPath = %q, want %q", result.TargetPath, "database.primary.url")
	}

	updatedRoot := decodeJSONRoot(t, readFile(t, targetPath))
	database := updatedRoot["database"].(map[string]any)
	primary := database["primary"].(map[string]any)
	if primary["url"] != "postgres://new" {
		t.Fatalf("database.primary.url = %q, want %q", primary["url"], "postgres://new")
	}
	if updatedRoot["AllowedHosts"] != "*" {
		t.Fatalf("AllowedHosts = %q, want %q", updatedRoot["AllowedHosts"], "*")
	}
}

func TestApplication_Profiles_ReturnsResolvedDisplayDataForAvailableProfiles(t *testing.T) {
	t.Setenv("MYAPPLICATION_TEST_CONNECTION_STRING", "Server=test;Database=App;Password=super-secret;")

	application := app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("Server=localhost;Database=App;Pwd=local-secret;")},
			{Name: "Test", ValueFromEnv: stringPointer("MYAPPLICATION_TEST_CONNECTION_STRING"), Protected: true},
		},
	)

	items := application.Profiles()
	if len(items) != 2 {
		t.Fatalf("len(Profiles()) = %d, want 2", len(items))
	}

	if items[0].Source != app.ProfileSourceLiteral {
		t.Fatalf("Profiles()[0].Source = %q, want %q", items[0].Source, app.ProfileSourceLiteral)
	}
	if items[0].MaskedValue != "Server=localhost;Database=App;Pwd=****;" {
		t.Fatalf("Profiles()[0].MaskedValue = %q, want masked literal value", items[0].MaskedValue)
	}
	if !items[1].Available {
		t.Fatalf("Profiles()[1].Available = false, want true (reason: %q)", items[1].UnavailableReason)
	}
	if !items[1].Protected {
		t.Fatal("Profiles()[1].Protected = false, want true")
	}
	if items[1].Source != app.ProfileSourceEnvironment {
		t.Fatalf("Profiles()[1].Source = %q, want %q", items[1].Source, app.ProfileSourceEnvironment)
	}
	if items[1].EnvironmentVariableName != "MYAPPLICATION_TEST_CONNECTION_STRING" {
		t.Fatalf("Profiles()[1].EnvironmentVariableName = %q, want %q", items[1].EnvironmentVariableName, "MYAPPLICATION_TEST_CONNECTION_STRING")
	}
	if items[1].MaskedValue != "Server=test;Database=App;Password=****;" {
		t.Fatalf("Profiles()[1].MaskedValue = %q, want masked environment value", items[1].MaskedValue)
	}
	if items[1].UnavailableReason != "" {
		t.Fatalf("Profiles()[1].UnavailableReason = %q, want empty string", items[1].UnavailableReason)
	}
}

func TestApplication_Profiles_ReturnsUnavailableResolutionError(t *testing.T) {
	application := app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MYAPPLICATION_MISSING_CONNECTION_STRING")}},
	)

	items := application.Profiles()
	if len(items) != 1 {
		t.Fatalf("len(Profiles()) = %d, want 1", len(items))
	}
	if items[0].Available {
		t.Fatal("Profiles()[0].Available = true, want false")
	}
	if !strings.Contains(items[0].UnavailableReason, "MYAPPLICATION_MISSING_CONNECTION_STRING") {
		t.Fatalf("Profiles()[0].UnavailableReason = %q, want environment variable name", items[0].UnavailableReason)
	}
	if items[0].MaskedValue != "" {
		t.Fatalf("Profiles()[0].MaskedValue = %q, want empty string", items[0].MaskedValue)
	}
}

func TestApplication_Profiles_ReturnsUnavailableForEmptyLiteralValue(t *testing.T) {
	application := app.New(
		config.Target{},
		[]config.Profile{{Name: "Local", Value: stringPointer("")}},
	)

	items := application.Profiles()
	if len(items) != 1 {
		t.Fatalf("len(Profiles()) = %d, want 1", len(items))
	}
	if items[0].Available {
		t.Fatal("Profiles()[0].Available = true, want false")
	}
	if !strings.Contains(items[0].UnavailableReason, "value is empty") {
		t.Fatalf("Profiles()[0].UnavailableReason = %q, want empty-value guidance", items[0].UnavailableReason)
	}
}

func TestApplication_InspectProfileByName_ReturnsResolvedDisplayData(t *testing.T) {
	t.Setenv("MYAPPLICATION_TEST_CONNECTION_STRING", "Server=test;Database=App;Password=super-secret;")

	application := app.New(
		config.Target{},
		[]config.Profile{{Name: "Test", ValueFromEnv: stringPointer("MYAPPLICATION_TEST_CONNECTION_STRING"), Protected: true}},
	)

	item, err := application.InspectProfileByName("Test")
	if err != nil {
		t.Fatalf("InspectProfileByName returned error: %v", err)
	}
	if !item.Available {
		t.Fatalf("Available = false, want true (reason: %q)", item.UnavailableReason)
	}
	if !item.Protected {
		t.Fatal("Protected = false, want true")
	}
	if item.Source != app.ProfileSourceEnvironment {
		t.Fatalf("Source = %q, want %q", item.Source, app.ProfileSourceEnvironment)
	}
	if item.EnvironmentVariableName != "MYAPPLICATION_TEST_CONNECTION_STRING" {
		t.Fatalf("EnvironmentVariableName = %q, want %q", item.EnvironmentVariableName, "MYAPPLICATION_TEST_CONNECTION_STRING")
	}
	if item.MaskedValue != "Server=test;Database=App;Password=****;" {
		t.Fatalf("MaskedValue = %q, want masked value", item.MaskedValue)
	}
	if item.UnavailableReason != "" {
		t.Fatalf("UnavailableReason = %q, want empty string", item.UnavailableReason)
	}
}

func TestApplication_InspectProfileByName_ReturnsUnavailableProfile(t *testing.T) {
	application := app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MYAPPLICATION_MISSING_CONNECTION_STRING")}},
	)

	item, err := application.InspectProfileByName("Production")
	if err != nil {
		t.Fatalf("InspectProfileByName returned error: %v", err)
	}
	if item.Available {
		t.Fatal("Available = true, want false")
	}
	if !strings.Contains(item.UnavailableReason, "MYAPPLICATION_MISSING_CONNECTION_STRING") {
		t.Fatalf("UnavailableReason = %q, want environment variable name", item.UnavailableReason)
	}
	if item.MaskedValue != "" {
		t.Fatalf("MaskedValue = %q, want empty string", item.MaskedValue)
	}
}

func TestApplication_InspectProfileByName_ReturnsErrorForUnknownProfile(t *testing.T) {
	application := app.New(config.Target{}, []config.Profile{{Name: "Local", Value: stringPointer("postgres://local")}})

	_, err := application.InspectProfileByName("Missing")
	if err == nil {
		t.Fatal("InspectProfileByName returned nil error, want not-found error")
	}
	if !errors.Is(err, app.ErrProfileNotFound) {
		t.Fatalf("InspectProfileByName returned error %v, want ErrProfileNotFound", err)
	}
}

func TestApplication_ApplyProfile_AppliesEnvironmentProfile(t *testing.T) {
	t.Setenv("MYAPPLICATION_SERVICE_URL", "https://env.example.test")

	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"serviceUrl":"https://old.example.test"}`)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "serviceUrl"},
		[]config.Profile{{Name: "Test", ValueFromEnv: stringPointer("MYAPPLICATION_SERVICE_URL")}},
	)
	result, err := application.ApplyProfileByName("Test")
	if err != nil {
		t.Fatalf("ApplyProfileByName returned error: %v", err)
	}
	if result.ProfileName != "Test" {
		t.Fatalf("ProfileName = %q, want %q", result.ProfileName, "Test")
	}
	if result.TargetPath != "serviceUrl" {
		t.Fatalf("TargetPath = %q, want %q", result.TargetPath, "serviceUrl")
	}

	updatedRoot := decodeJSONRoot(t, readFile(t, targetPath))
	if updatedRoot["serviceUrl"] != "https://env.example.test" {
		t.Fatalf("serviceUrl = %q, want resolved environment value", updatedRoot["serviceUrl"])
	}
}

func TestApplication_ApplyProfile_AppliesMultipleTargetProfile(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=old;Database=App;"
  }
}
`)+"\n")
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", strings.TrimSpace(`
VITE_API_URL=http://localhost:5173
VITE_FEATURES=local
`)+"\n")

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "ConnectionStrings.DefaultConnection"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{{
			Name:      "Staging",
			Protected: true,
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("Server=staging;Database=App;")},
				{Target: "frontendApi", Value: stringPointer("https://api.staging.example.test")},
			},
		}},
	)

	result, err := application.ApplyProfileByName("Staging")
	if err != nil {
		t.Fatalf("ApplyProfileByName returned error: %v", err)
	}

	if result.ProfileName != "Staging" {
		t.Fatalf("ProfileName = %q, want %q", result.ProfileName, "Staging")
	}
	if !result.Protected {
		t.Fatal("Protected = false, want true")
	}
	if len(result.Changes) != 2 {
		t.Fatalf("len(Changes) = %d, want 2", len(result.Changes))
	}
	if result.Changes[0].TargetName != "database" || result.Changes[0].Selector != "ConnectionStrings.DefaultConnection" {
		t.Fatalf("Changes[0] = %#v, want database target", result.Changes[0])
	}
	if result.Changes[1].TargetName != "frontendApi" || result.Changes[1].SelectorName != "key" || result.Changes[1].Selector != "VITE_API_URL" {
		t.Fatalf("Changes[1] = %#v, want frontend dotenv target", result.Changes[1])
	}

	databaseRoot := decodeJSONRoot(t, readFile(t, databasePath))
	connectionStrings := databaseRoot["ConnectionStrings"].(map[string]any)
	if connectionStrings["DefaultConnection"] != "Server=staging;Database=App;" {
		t.Fatalf("DefaultConnection = %q, want updated database value", connectionStrings["DefaultConnection"])
	}
	if got := string(readFile(t, frontendPath)); !strings.Contains(got, "VITE_API_URL=https://api.staging.example.test") {
		t.Fatalf("dotenv contents = %q, want updated API URL", got)
	}
	if got := string(readFile(t, frontendPath)); !strings.Contains(got, "VITE_FEATURES=local") {
		t.Fatalf("dotenv contents = %q, want unrelated entry preserved", got)
	}
}

func TestApplication_ApplyProfile_OnlyUpdatesIncludedTargetsForPartialProfile(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")
	originalFrontendContents := readFile(t, frontendPath)

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{{
			Name: "Database Only",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://new")},
			},
		}},
	)

	profileItems := application.Profiles()
	if len(profileItems) != 1 {
		t.Fatalf("len(Profiles()) = %d, want 1", len(profileItems))
	}
	if !profileItems[0].Partial {
		t.Fatal("Profiles()[0].Partial = false, want true")
	}
	if profileItems[0].TargetCount != 1 || profileItems[0].TotalTargets != 2 {
		t.Fatalf("TargetCount/TotalTargets = %d/%d, want 1/2", profileItems[0].TargetCount, profileItems[0].TotalTargets)
	}

	result, err := application.ApplyProfileByName("Database Only")
	if err != nil {
		t.Fatalf("ApplyProfileByName returned error: %v", err)
	}
	if len(result.Changes) != 1 || result.Changes[0].TargetName != "database" {
		t.Fatalf("Changes = %#v, want only database target", result.Changes)
	}

	databaseRoot := decodeJSONRoot(t, readFile(t, databasePath))
	database := databaseRoot["database"].(map[string]any)
	if database["url"] != "postgres://new" {
		t.Fatalf("database.url = %q, want updated value", database["url"])
	}
	if !bytes.Equal(readFile(t, frontendPath), originalFrontendContents) {
		t.Fatal("omitted frontend target changed after applying partial profile")
	}
}

func TestApplication_ApplyProfileWithOptions_DryRunValidatesMultipleTargetsWithoutWriting(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")
	originalDatabaseContents := readFile(t, databasePath)
	originalFrontendContents := readFile(t, frontendPath)

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://new")},
				{Target: "frontendApi", Value: stringPointer("https://api.staging.example.test")},
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
	if len(result.Changes) != 2 {
		t.Fatalf("len(Changes) = %d, want 2", len(result.Changes))
	}
	if !bytes.Equal(readFile(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed during dry run")
	}
	if !bytes.Equal(readFile(t, frontendPath), originalFrontendContents) {
		t.Fatal("frontend target changed during dry run")
	}
}

func TestApplication_ApplyProfileWithOptions_DryRunPreparationFailureLeavesAllTargetsUnchangedAndHidesSecrets(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")
	originalDatabaseContents := readFile(t, databasePath)
	originalFrontendContents := readFile(t, frontendPath)

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://database-secret")},
				{Target: "frontendApi", Value: stringPointer("https://api.secret.example.test\nNEXT=value")},
			},
		}},
	)

	_, err := application.ApplyProfileByNameWithOptions("Staging", app.ApplyOptions{DryRun: true})
	if err == nil {
		t.Fatal("ApplyProfileByNameWithOptions returned nil error, want dry-run preparation failure")
	}
	for _, expected := range []string{
		`dry-run apply profile "Staging"`,
		`target "frontendApi"`,
		`key "VITE_API_URL"`,
		"replacement value must not contain newline characters",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("ApplyProfileByNameWithOptions returned error %q, want substring %q", err, expected)
		}
	}
	for _, forbidden := range []string{"database-secret", "api.secret.example.test"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("ApplyProfileByNameWithOptions returned error %q, must not contain resolved value %q", err, forbidden)
		}
	}
	if !bytes.Equal(readFile(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed after dry-run preparation failure")
	}
	if !bytes.Equal(readFile(t, frontendPath), originalFrontendContents) {
		t.Fatal("frontend target changed after dry-run preparation failure")
	}
}

func TestApplication_ApplyProfile_PreparationFailureLeavesAllTargetsUnchangedAndHidesSecrets(t *testing.T) {
	t.Setenv("STAGING_DATABASE_URL", "postgres://user:super-secret@example.test/app")

	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")
	originalDatabaseContents := readFile(t, databasePath)
	originalFrontendContents := readFile(t, frontendPath)

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "MISSING_KEY"},
		},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "database", ValueFromEnv: stringPointer("STAGING_DATABASE_URL")},
				{Target: "frontendApi", Value: stringPointer("https://api.staging.example.test")},
			},
		}},
	)

	_, err := application.ApplyProfileByName("Staging")
	if err == nil {
		t.Fatal("ApplyProfileByName returned nil error, want preparation failure")
	}
	if !strings.Contains(err.Error(), `target "frontendApi"`) || !strings.Contains(err.Error(), `key "MISSING_KEY"`) {
		t.Fatalf("ApplyProfileByName returned error %q, want frontend target and key context", err)
	}
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("ApplyProfileByName returned error %q, must not contain resolved secrets", err)
	}
	if !bytes.Equal(readFile(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed after another target failed preparation")
	}
	if !bytes.Equal(readFile(t, frontendPath), originalFrontendContents) {
		t.Fatal("frontend target changed after preparation failure")
	}
}

func TestApplication_ApplyProfile_ReturnsErrorForMissingEnvironmentVariable(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"serviceUrl":"https://old.example.test"}`)
	originalContents := readFile(t, targetPath)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "serviceUrl"},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MYAPPLICATION_MISSING_CONNECTION_STRING")}},
	)
	_, err := application.ApplyProfileByName("Production")
	if err == nil {
		t.Fatal("ApplyProfileByName returned nil error, want missing environment variable error")
	}
	if !errors.Is(err, app.ErrProfileUnavailable) {
		t.Fatalf("ApplyProfileByName returned error %v, want ErrProfileUnavailable", err)
	}
	if !errors.Is(err, profile.ErrEnvironmentVariableNotSet) {
		t.Fatalf("ApplyProfileByName returned error %v, want ErrEnvironmentVariableNotSet", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after missing environment variable error")
	}
}

func TestApplication_ApplyProfile_ReturnsErrorForEmptyEnvironmentVariable(t *testing.T) {
	t.Setenv("MYAPPLICATION_EMPTY_CONNECTION_STRING", "")

	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"serviceUrl":"https://old.example.test"}`)
	originalContents := readFile(t, targetPath)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "serviceUrl"},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MYAPPLICATION_EMPTY_CONNECTION_STRING")}},
	)
	_, err := application.ApplyProfileByName("Production")
	if err == nil {
		t.Fatal("ApplyProfileByName returned nil error, want empty environment variable error")
	}
	if !errors.Is(err, app.ErrProfileUnavailable) {
		t.Fatalf("ApplyProfileByName returned error %v, want ErrProfileUnavailable", err)
	}
	if !errors.Is(err, profile.ErrEnvironmentVariableEmpty) {
		t.Fatalf("ApplyProfileByName returned error %v, want ErrEnvironmentVariableEmpty", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after empty environment variable error")
	}
}

func TestApplication_ApplyProfile_ReturnsErrorForEmptyResolvedValue(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"serviceUrl":"https://old.example.test"}`)
	originalContents := readFile(t, targetPath)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "serviceUrl"},
		[]config.Profile{{Name: "Local", Value: stringPointer("")}},
	)
	_, err := application.ApplyProfileByName("Local")
	if err == nil {
		t.Fatal("ApplyProfileByName returned nil error, want empty value error")
	}
	if !errors.Is(err, app.ErrProfileUnavailable) {
		t.Fatalf("ApplyProfileByName returned error %v, want ErrProfileUnavailable", err)
	}
	if !errors.Is(err, profile.ErrProfileValueEmpty) {
		t.Fatalf("ApplyProfileByName returned error %v, want ErrProfileValueEmpty", err)
	}
	if !strings.Contains(err.Error(), `profile "Local" value is empty`) {
		t.Fatalf("ApplyProfileByName returned error %q, want empty value guidance", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after empty value error")
	}
}

func TestApplication_ApplyProfileWithOptions_ReturnsErrorForProtectedProfileWithoutOptIn(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"serviceUrl":"https://old.example.test"}`)
	originalContents := readFile(t, targetPath)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "serviceUrl"},
		[]config.Profile{{Name: "Production", Value: stringPointer("https://prod.example.test"), Protected: true}},
	)

	_, err := application.ApplyProfileByNameWithOptions("Production", app.ApplyOptions{})
	if err == nil {
		t.Fatal("ApplyProfileByNameWithOptions returned nil error, want protected-profile error")
	}
	if !errors.Is(err, app.ErrProtectedProfileRequiresApproval) {
		t.Fatalf("ApplyProfileByNameWithOptions returned error %v, want ErrProtectedProfileRequiresApproval", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after protected-profile refusal")
	}
}

func TestApplication_ApplyProfileWithOptions_DryRunDoesNotModifyTargetFile(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"serviceUrl":"https://old.example.test"}`)
	originalContents := readFile(t, targetPath)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "serviceUrl"},
		[]config.Profile{{Name: "Local", Value: stringPointer("https://new.example.test")}},
	)

	result, err := application.ApplyProfileByNameWithOptions("Local", app.ApplyOptions{DryRun: true})
	if err != nil {
		t.Fatalf("ApplyProfileByNameWithOptions returned error: %v", err)
	}
	if !result.DryRun {
		t.Fatal("DryRun = false, want true")
	}
	if result.TargetFile != targetPath {
		t.Fatalf("TargetFile = %q, want %q", result.TargetFile, targetPath)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed during dry run")
	}
}

func TestApplication_ApplyProfile_PropagatesEditorFailureWithoutLeakingSecrets(t *testing.T) {
	t.Setenv("MYAPPLICATION_PRODUCTION_CONNECTION_STRING", "Server=prod;Database=App;Password=super-secret;")

	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{`)
	originalContents := readFile(t, targetPath)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "serviceUrl"},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MYAPPLICATION_PRODUCTION_CONNECTION_STRING")}},
	)
	_, err := application.ApplyProfileByName("Production")
	if err == nil {
		t.Fatal("ApplyProfileByName returned nil error, want editor failure")
	}
	if !strings.Contains(err.Error(), `apply profile "Production"`) {
		t.Fatalf("ApplyProfileByName returned error %q, want contextual profile error", err)
	}
	if !strings.Contains(err.Error(), `contains invalid JSON`) {
		t.Fatalf("ApplyProfileByName returned error %q, want editor failure", err)
	}
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("ApplyProfileByName returned error %q, must not contain secrets", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after editor failure")
	}
}

func TestApplication_ValidateStartup_ReturnsErrorForMissingTargetPath(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{}}`)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "database.primary.url"},
		nil,
	)

	err := application.ValidateStartup()
	if err == nil {
		t.Fatal("ValidateStartup returned nil error, want target-path validation error")
	}
	if !strings.Contains(err.Error(), `does not contain JSON path "database.primary.url"`) {
		t.Fatalf("ValidateStartup returned error %q, want missing target path error", err)
	}
}

func TestApplication_ValidateStartup_ReturnsErrorForNonStringTargetValue(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"primary":{"port":5432}}}`)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "database.primary.port"},
		nil,
	)

	err := application.ValidateStartup()
	if err == nil {
		t.Fatal("ValidateStartup returned nil error, want non-string target error")
	}
	if !strings.Contains(err.Error(), `JSON path "database.primary.port" must resolve to a string`) {
		t.Fatalf("ValidateStartup returned error %q, want non-string target error", err)
	}
}

func writeTargetFile(t *testing.T, rootDir string, relativePath string, contents string) string {
	t.Helper()

	fullPath := filepath.Join(rootDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create parent directories for %q: %v", fullPath, err)
	}

	if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write target file %q: %v", fullPath, err)
	}

	return fullPath
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %q: %v", path, err)
	}

	return contents
}

func decodeJSONRoot(t *testing.T, contents []byte) map[string]any {
	t.Helper()

	var decodedRoot map[string]any
	if err := json.Unmarshal(contents, &decodedRoot); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	return decodedRoot
}

func stringPointer(value string) *string {
	return &value
}
