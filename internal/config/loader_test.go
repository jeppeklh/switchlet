package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
)

func TestLoad_LoadsAndValidatesLegacyVersionOneConfiguration(t *testing.T) {
	configPath := fixturePath(t, "valid", "basic", ".switchlet.yaml")

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	projectRoot := fixturePath(t, "valid", "basic")
	wantTargetPath := filepath.Join(projectRoot, "src", "MyApplication", "appsettings.Development.json")

	if loadedConfig.Version != 1 {
		t.Fatalf("Version = %d, want 1", loadedConfig.Version)
	}
	if loadedConfig.Target.File != wantTargetPath {
		t.Fatalf("Target.File = %q, want %q", loadedConfig.Target.File, wantTargetPath)
	}
	if loadedConfig.Target.JSONPath != "ConnectionStrings.DefaultConnection" {
		t.Fatalf("Target.JSONPath = %q, want %q", loadedConfig.Target.JSONPath, "ConnectionStrings.DefaultConnection")
	}
	if len(loadedConfig.Targets) != 1 {
		t.Fatalf("len(Targets) = %d, want 1", len(loadedConfig.Targets))
	}
	if loadedConfig.Targets[0].Name != "default" {
		t.Fatalf("Targets[0].Name = %q, want %q", loadedConfig.Targets[0].Name, "default")
	}
	if loadedConfig.Targets[0].Type != config.TargetTypeJSON {
		t.Fatalf("Targets[0].Type = %q, want %q", loadedConfig.Targets[0].Type, config.TargetTypeJSON)
	}
	if loadedConfig.Targets[0].JSONPath != "ConnectionStrings.DefaultConnection" {
		t.Fatalf("Targets[0].JSONPath = %q, want %q", loadedConfig.Targets[0].JSONPath, "ConnectionStrings.DefaultConnection")
	}
	if len(loadedConfig.Profiles) != 2 {
		t.Fatalf("len(Profiles) = %d, want 2", len(loadedConfig.Profiles))
	}

	if loadedConfig.Profiles[0].Name != "Local" {
		t.Fatalf("Profiles[0].Name = %q, want %q", loadedConfig.Profiles[0].Name, "Local")
	}
	if loadedConfig.Profiles[0].Value == nil {
		t.Fatal("Profiles[0].Value is nil, want literal value")
	}
	if !strings.Contains(*loadedConfig.Profiles[0].Value, "Database=MyApplication") {
		t.Fatalf("Profiles[0].Value = %q, want literal connection string", *loadedConfig.Profiles[0].Value)
	}
	if len(loadedConfig.Profiles[0].Values) != 1 {
		t.Fatalf("len(Profiles[0].Values) = %d, want 1", len(loadedConfig.Profiles[0].Values))
	}
	if loadedConfig.Profiles[0].Values[0].Target != "default" {
		t.Fatalf("Profiles[0].Values[0].Target = %q, want %q", loadedConfig.Profiles[0].Values[0].Target, "default")
	}

	if loadedConfig.Profiles[1].Name != "Test" {
		t.Fatalf("Profiles[1].Name = %q, want %q", loadedConfig.Profiles[1].Name, "Test")
	}
	if loadedConfig.Profiles[1].ValueFromEnv == nil {
		t.Fatal("Profiles[1].ValueFromEnv is nil, want environment variable name")
	}
	if len(loadedConfig.Profiles[1].Values) != 1 {
		t.Fatalf("len(Profiles[1].Values) = %d, want 1", len(loadedConfig.Profiles[1].Values))
	}
	if loadedConfig.Profiles[1].Values[0].ValueFromEnv == nil {
		t.Fatal("Profiles[1].Values[0].ValueFromEnv is nil, want environment variable name")
	}
	if *loadedConfig.Profiles[1].ValueFromEnv != "MYAPPLICATION_TEST_CONNECTION_STRING" {
		t.Fatalf("Profiles[1].ValueFromEnv = %q, want %q", *loadedConfig.Profiles[1].ValueFromEnv, "MYAPPLICATION_TEST_CONNECTION_STRING")
	}
}

func TestLoad_LoadsAndValidatesVersionTwoConfiguration(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/development.json
  jsonPath: database.primary.url

profiles:
  - name: Local
    value: postgres://localhost:5432/myapp

  - name: Test
    valueFromEnv: MYAPP_TEST_DATABASE_URL
`)+"\n")
	writeFile(t, projectRoot, "config/development.json", strings.TrimSpace(`
{
  "database": {
    "primary": {
      "url": "postgres://old"
    }
  }
}
`)+"\n")

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if loadedConfig.Version != 2 {
		t.Fatalf("Version = %d, want 2", loadedConfig.Version)
	}
	if loadedConfig.Target.File != filepath.Join(projectRoot, "config", "development.json") {
		t.Fatalf("Target.File = %q, want %q", loadedConfig.Target.File, filepath.Join(projectRoot, "config", "development.json"))
	}
	if loadedConfig.Target.JSONPath != "database.primary.url" {
		t.Fatalf("Target.JSONPath = %q, want %q", loadedConfig.Target.JSONPath, "database.primary.url")
	}
	if len(loadedConfig.Targets) != 1 {
		t.Fatalf("len(Targets) = %d, want 1", len(loadedConfig.Targets))
	}
	if loadedConfig.Targets[0].Name != "default" {
		t.Fatalf("Targets[0].Name = %q, want %q", loadedConfig.Targets[0].Name, "default")
	}
	if loadedConfig.Targets[0].Type != config.TargetTypeJSON {
		t.Fatalf("Targets[0].Type = %q, want %q", loadedConfig.Targets[0].Type, config.TargetTypeJSON)
	}
	if loadedConfig.Targets[0].JSONPath != "database.primary.url" {
		t.Fatalf("Targets[0].JSONPath = %q, want %q", loadedConfig.Targets[0].JSONPath, "database.primary.url")
	}
	if len(loadedConfig.Profiles) != 2 {
		t.Fatalf("len(Profiles) = %d, want 2", len(loadedConfig.Profiles))
	}
	if len(loadedConfig.Profiles[0].Values) != 1 {
		t.Fatalf("len(Profiles[0].Values) = %d, want 1", len(loadedConfig.Profiles[0].Values))
	}
	if loadedConfig.Profiles[0].Values[0].Target != "default" {
		t.Fatalf("Profiles[0].Values[0].Target = %q, want %q", loadedConfig.Profiles[0].Values[0].Target, "default")
	}
}

func TestLoad_LoadsVersionThreeConfiguration(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: backend/appsettings.Development.json
    type: json
    jsonPath: ConnectionStrings.DefaultConnection

  - name: frontendApi
    file: frontend/.env.local
    type: dotenv
    key: VITE_API_URL

profiles:
  - name: Local
    values:
      - target: database
        value: Server=localhost;Database=App;Trusted_Connection=True;
      - target: frontendApi
        value: http://localhost:5173

  - name: Staging
    protected: true
    values:
      - target: database
        valueFromEnv: STAGING_DATABASE_URL
      - target: frontendApi
        value: https://api.staging.example.com

  - name: Local Database Only
    values:
      - target: database
        value: Server=localhost;Database=App;Trusted_Connection=True;
`)+"\n")

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if loadedConfig.Version != 3 {
		t.Fatalf("Version = %d, want 3", loadedConfig.Version)
	}
	if len(loadedConfig.Targets) != 2 {
		t.Fatalf("len(Targets) = %d, want 2", len(loadedConfig.Targets))
	}

	databaseTarget := loadedConfig.Targets[0]
	if databaseTarget.Name != "database" {
		t.Fatalf("Targets[0].Name = %q, want %q", databaseTarget.Name, "database")
	}
	if databaseTarget.File != filepath.Join(projectRoot, "backend", "appsettings.Development.json") {
		t.Fatalf("Targets[0].File = %q, want resolved backend target path", databaseTarget.File)
	}
	if databaseTarget.Type != config.TargetTypeJSON {
		t.Fatalf("Targets[0].Type = %q, want %q", databaseTarget.Type, config.TargetTypeJSON)
	}
	if databaseTarget.JSONPath != "ConnectionStrings.DefaultConnection" {
		t.Fatalf("Targets[0].JSONPath = %q, want %q", databaseTarget.JSONPath, "ConnectionStrings.DefaultConnection")
	}

	frontendTarget := loadedConfig.Targets[1]
	if frontendTarget.Name != "frontendApi" {
		t.Fatalf("Targets[1].Name = %q, want %q", frontendTarget.Name, "frontendApi")
	}
	if frontendTarget.File != filepath.Join(projectRoot, "frontend", ".env.local") {
		t.Fatalf("Targets[1].File = %q, want resolved frontend target path", frontendTarget.File)
	}
	if frontendTarget.Type != config.TargetTypeDotenv {
		t.Fatalf("Targets[1].Type = %q, want %q", frontendTarget.Type, config.TargetTypeDotenv)
	}
	if frontendTarget.Key != "VITE_API_URL" {
		t.Fatalf("Targets[1].Key = %q, want %q", frontendTarget.Key, "VITE_API_URL")
	}

	if len(loadedConfig.Profiles) != 3 {
		t.Fatalf("len(Profiles) = %d, want 3", len(loadedConfig.Profiles))
	}
	if len(loadedConfig.Profiles[0].Values) != 2 {
		t.Fatalf("len(Profiles[0].Values) = %d, want 2", len(loadedConfig.Profiles[0].Values))
	}
	if loadedConfig.Profiles[0].Values[0].Target != "database" {
		t.Fatalf("Profiles[0].Values[0].Target = %q, want %q", loadedConfig.Profiles[0].Values[0].Target, "database")
	}
	if loadedConfig.Profiles[0].Values[1].Target != "frontendApi" {
		t.Fatalf("Profiles[0].Values[1].Target = %q, want %q", loadedConfig.Profiles[0].Values[1].Target, "frontendApi")
	}
	if !loadedConfig.Profiles[1].Protected {
		t.Fatal("Profiles[1].Protected = false, want true")
	}
	if loadedConfig.Profiles[1].Values[0].ValueFromEnv == nil {
		t.Fatal("Profiles[1].Values[0].ValueFromEnv is nil, want environment variable name")
	}
	if *loadedConfig.Profiles[1].Values[0].ValueFromEnv != "STAGING_DATABASE_URL" {
		t.Fatalf("Profiles[1].Values[0].ValueFromEnv = %q, want %q", *loadedConfig.Profiles[1].Values[0].ValueFromEnv, "STAGING_DATABASE_URL")
	}
	if len(loadedConfig.Profiles[2].Values) != 1 {
		t.Fatalf("len(Profiles[2].Values) = %d, want partial profile with one value", len(loadedConfig.Profiles[2].Values))
	}
}

func TestLoad_LoadsVersionTwoNestedDatabaseExampleFixture(t *testing.T) {
	configPath := fixturePath(t, "valid", "generic-database", ".switchlet.yaml")

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	projectRoot := fixturePath(t, "valid", "generic-database")
	if loadedConfig.Version != 2 {
		t.Fatalf("Version = %d, want 2", loadedConfig.Version)
	}
	if loadedConfig.Target.File != filepath.Join(projectRoot, "config", "development.json") {
		t.Fatalf("Target.File = %q, want fixture target path", loadedConfig.Target.File)
	}
	if loadedConfig.Target.JSONPath != "database.primary.url" {
		t.Fatalf("Target.JSONPath = %q, want %q", loadedConfig.Target.JSONPath, "database.primary.url")
	}
	if len(loadedConfig.Profiles) != 2 {
		t.Fatalf("len(Profiles) = %d, want 2", len(loadedConfig.Profiles))
	}
}

func TestLoad_LoadsVersionTwoServiceBaseURLExampleFixture(t *testing.T) {
	configPath := fixturePath(t, "valid", "service-base-url", ".switchlet.yaml")

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	projectRoot := fixturePath(t, "valid", "service-base-url")
	if loadedConfig.Version != 2 {
		t.Fatalf("Version = %d, want 2", loadedConfig.Version)
	}
	if loadedConfig.Target.File != filepath.Join(projectRoot, "config", "runtime.json") {
		t.Fatalf("Target.File = %q, want fixture target path", loadedConfig.Target.File)
	}
	if loadedConfig.Target.JSONPath != "services.backend.baseUrl" {
		t.Fatalf("Target.JSONPath = %q, want %q", loadedConfig.Target.JSONPath, "services.backend.baseUrl")
	}
	if len(loadedConfig.Profiles) != 2 {
		t.Fatalf("len(Profiles) = %d, want 2", len(loadedConfig.Profiles))
	}
	if !loadedConfig.Profiles[1].Protected {
		t.Fatal("Profiles[1].Protected = false, want true")
	}
}

func TestLoad_ResolvesTargetPathRelativeToConfiguration(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := writeFile(t, projectRoot, "config/.switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: ../src/MyApplication/appsettings.Development.json
  jsonPath: ConnectionStrings.DefaultConnection

profiles:
  - name: Local
    value: "Server=localhost;Database=MyApplication;"
`)+"\n")
	writeFile(t, projectRoot, "src/MyApplication/appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=MyApplication;"
  }
}
`)+"\n")

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	wantTargetPath := filepath.Join(projectRoot, "src", "MyApplication", "appsettings.Development.json")
	if loadedConfig.Target.File != wantTargetPath {
		t.Fatalf("Target.File = %q, want %q", loadedConfig.Target.File, wantTargetPath)
	}
}

func TestLoad_ReturnsErrorForInvalidYAML(t *testing.T) {
	configPath := fixturePath(t, "invalid", "invalid-yaml", ".switchlet.yaml")

	_, err := config.Load(configPath)
	if err == nil {
		t.Fatal("Load returned nil error, want parse error")
	}

	if !strings.Contains(err.Error(), "parse configuration file") {
		t.Fatalf("Load returned error %q, want parse configuration file error", err)
	}
}
