package profile_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/profile"
)

func TestResolveProfile_ReturnsLiteralValue(t *testing.T) {
	configuredProfile := config.Profile{
		Name:      "Local",
		Value:     stringPointer("Server=localhost;Database=MyApplication;Password=secret;"),
		Protected: true,
	}

	resolvedProfile := profile.ResolveProfile(configuredProfile)

	if !resolvedProfile.IsAvailable() {
		t.Fatalf("IsAvailable() = false, want true (error: %v)", resolvedProfile.ResolutionError)
	}
	if resolvedProfile.Name != "Local" {
		t.Fatalf("Name = %q, want %q", resolvedProfile.Name, "Local")
	}
	if resolvedProfile.Source != profile.ValueSourceLiteral {
		t.Fatalf("Source = %q, want %q", resolvedProfile.Source, profile.ValueSourceLiteral)
	}
	if !resolvedProfile.Protected {
		t.Fatal("Protected = false, want true")
	}
	if resolvedProfile.EnvironmentVariableName != "" {
		t.Fatalf("EnvironmentVariableName = %q, want empty string", resolvedProfile.EnvironmentVariableName)
	}
	if resolvedProfile.Value != "Server=localhost;Database=MyApplication;Password=secret;" {
		t.Fatalf("Value = %q, want literal connection string", resolvedProfile.Value)
	}
	if resolvedProfile.MaskedValue != "hidden literal value" {
		t.Fatalf("MaskedValue = %q, want redacted literal value", resolvedProfile.MaskedValue)
	}
	if resolvedProfile.ResolutionError != nil {
		t.Fatalf("ResolutionError = %v, want nil", resolvedProfile.ResolutionError)
	}
}

func TestResolveProfile_ResolvesEnvironmentVariable(t *testing.T) {
	t.Setenv("MYAPPLICATION_TEST_CONNECTION_STRING", "Server=test;Database=MyApplication;Pwd=secret;")

	configuredProfile := config.Profile{
		Name:         "Test",
		ValueFromEnv: stringPointer("MYAPPLICATION_TEST_CONNECTION_STRING"),
	}

	resolvedProfile := profile.ResolveProfile(configuredProfile)

	if !resolvedProfile.IsAvailable() {
		t.Fatalf("IsAvailable() = false, want true (error: %v)", resolvedProfile.ResolutionError)
	}
	if resolvedProfile.Source != profile.ValueSourceEnvironment {
		t.Fatalf("Source = %q, want %q", resolvedProfile.Source, profile.ValueSourceEnvironment)
	}
	if resolvedProfile.EnvironmentVariableName != "MYAPPLICATION_TEST_CONNECTION_STRING" {
		t.Fatalf("EnvironmentVariableName = %q, want %q", resolvedProfile.EnvironmentVariableName, "MYAPPLICATION_TEST_CONNECTION_STRING")
	}
	if resolvedProfile.Value != "Server=test;Database=MyApplication;Pwd=secret;" {
		t.Fatalf("Value = %q, want resolved environment value", resolvedProfile.Value)
	}
	if resolvedProfile.MaskedValue != "hidden environment value" {
		t.Fatalf("MaskedValue = %q, want redacted environment value", resolvedProfile.MaskedValue)
	}
	if resolvedProfile.ResolutionError != nil {
		t.Fatalf("ResolutionError = %v, want nil", resolvedProfile.ResolutionError)
	}
}

func TestResolveProfile_ReturnsValuesForMultipleTargets(t *testing.T) {
	t.Setenv("STAGING_FRONTEND_API", "https://api.staging.example.test")

	configuredProfile := config.Profile{
		Name:      "Staging",
		Protected: true,
		Values: []config.ProfileValue{
			{Target: "database", Value: stringPointer("Server=db;Database=App;Password=secret;")},
			{Target: "frontendApi", ValueFromEnv: stringPointer("STAGING_FRONTEND_API")},
		},
	}

	resolvedProfile := profile.ResolveProfile(configuredProfile)

	if !resolvedProfile.IsAvailable() {
		t.Fatalf("IsAvailable() = false, want true (error: %v)", resolvedProfile.ResolutionError)
	}
	if resolvedProfile.Source != profile.ValueSourceMixed {
		t.Fatalf("Source = %q, want %q", resolvedProfile.Source, profile.ValueSourceMixed)
	}
	if !resolvedProfile.Protected {
		t.Fatal("Protected = false, want true")
	}
	if len(resolvedProfile.Values) != 2 {
		t.Fatalf("len(Values) = %d, want 2", len(resolvedProfile.Values))
	}

	databaseValue := resolvedProfile.Values[0]
	if databaseValue.Target != "database" {
		t.Fatalf("Values[0].Target = %q, want %q", databaseValue.Target, "database")
	}
	if databaseValue.Source != profile.ValueSourceLiteral {
		t.Fatalf("Values[0].Source = %q, want %q", databaseValue.Source, profile.ValueSourceLiteral)
	}
	if databaseValue.MaskedValue != "hidden literal value" {
		t.Fatalf("Values[0].MaskedValue = %q, want redacted database value", databaseValue.MaskedValue)
	}

	frontendValue := resolvedProfile.Values[1]
	if frontendValue.Target != "frontendApi" {
		t.Fatalf("Values[1].Target = %q, want %q", frontendValue.Target, "frontendApi")
	}
	if frontendValue.Source != profile.ValueSourceEnvironment {
		t.Fatalf("Values[1].Source = %q, want %q", frontendValue.Source, profile.ValueSourceEnvironment)
	}
	if frontendValue.EnvironmentVariableName != "STAGING_FRONTEND_API" {
		t.Fatalf("Values[1].EnvironmentVariableName = %q, want %q", frontendValue.EnvironmentVariableName, "STAGING_FRONTEND_API")
	}
	if frontendValue.Value != "https://api.staging.example.test" {
		t.Fatalf("Values[1].Value = %q, want resolved environment value", frontendValue.Value)
	}
	if frontendValue.MaskedValue != "hidden environment value" {
		t.Fatalf("Values[1].MaskedValue = %q, want redacted environment value", frontendValue.MaskedValue)
	}
}

func TestResolveProfile_ReturnsUnavailableForEmptyLiteralValue(t *testing.T) {
	configuredProfile := config.Profile{
		Name:  "Local",
		Value: stringPointer(""),
	}

	resolvedProfile := profile.ResolveProfile(configuredProfile)

	if resolvedProfile.IsAvailable() {
		t.Fatal("IsAvailable() = true, want false")
	}
	if !errors.Is(resolvedProfile.ResolutionError, profile.ErrProfileValueEmpty) {
		t.Fatalf("ResolutionError = %v, want ErrProfileValueEmpty", resolvedProfile.ResolutionError)
	}
	if !strings.Contains(resolvedProfile.ResolutionError.Error(), `profile "Local" value is empty`) {
		t.Fatalf("ResolutionError = %q, want empty-value guidance", resolvedProfile.ResolutionError)
	}
	if resolvedProfile.MaskedValue != "" {
		t.Fatalf("MaskedValue = %q, want empty string", resolvedProfile.MaskedValue)
	}
}

func TestResolveProfile_ReturnsUnavailableForMissingEnvironmentVariable(t *testing.T) {
	configuredProfile := config.Profile{
		Name:         "Production",
		ValueFromEnv: stringPointer("MYAPPLICATION_MISSING_CONNECTION_STRING"),
	}

	resolvedProfile := profile.ResolveProfile(configuredProfile)

	if resolvedProfile.IsAvailable() {
		t.Fatal("IsAvailable() = true, want false")
	}
	if !errors.Is(resolvedProfile.ResolutionError, profile.ErrEnvironmentVariableNotSet) {
		t.Fatalf("ResolutionError = %v, want ErrEnvironmentVariableNotSet", resolvedProfile.ResolutionError)
	}
	if !strings.Contains(resolvedProfile.ResolutionError.Error(), `MYAPPLICATION_MISSING_CONNECTION_STRING`) {
		t.Fatalf("ResolutionError = %q, want environment variable name", resolvedProfile.ResolutionError)
	}
	if strings.Contains(resolvedProfile.ResolutionError.Error(), "Password=") {
		t.Fatalf("ResolutionError = %q, must not contain secrets", resolvedProfile.ResolutionError)
	}
	if resolvedProfile.Value != "" {
		t.Fatalf("Value = %q, want empty string", resolvedProfile.Value)
	}
	if resolvedProfile.MaskedValue != "" {
		t.Fatalf("MaskedValue = %q, want empty string", resolvedProfile.MaskedValue)
	}
}

func TestResolveProfile_ReturnsUnavailableForOneMissingTargetValue(t *testing.T) {
	t.Setenv("SWITCHLET_TEST_STAGING_FRONTEND_API", "placeholder")
	if err := os.Unsetenv("SWITCHLET_TEST_STAGING_FRONTEND_API"); err != nil {
		t.Fatalf("unset test environment variable: %v", err)
	}

	configuredProfile := config.Profile{
		Name: "Staging",
		Values: []config.ProfileValue{
			{Target: "database", Value: stringPointer("Server=db;Database=App;Password=secret;")},
			{Target: "frontendApi", ValueFromEnv: stringPointer("SWITCHLET_TEST_STAGING_FRONTEND_API")},
		},
	}

	resolvedProfile := profile.ResolveProfile(configuredProfile)

	if resolvedProfile.IsAvailable() {
		t.Fatal("IsAvailable() = true, want false")
	}
	if !errors.Is(resolvedProfile.ResolutionError, profile.ErrEnvironmentVariableNotSet) {
		t.Fatalf("ResolutionError = %v, want ErrEnvironmentVariableNotSet", resolvedProfile.ResolutionError)
	}
	if len(resolvedProfile.Values) != 2 {
		t.Fatalf("len(Values) = %d, want 2", len(resolvedProfile.Values))
	}
	if resolvedProfile.Values[0].ResolutionError != nil {
		t.Fatalf("database ResolutionError = %v, want nil", resolvedProfile.Values[0].ResolutionError)
	}
	if !errors.Is(resolvedProfile.Values[1].ResolutionError, profile.ErrEnvironmentVariableNotSet) {
		t.Fatalf("frontend ResolutionError = %v, want ErrEnvironmentVariableNotSet", resolvedProfile.Values[1].ResolutionError)
	}
	if !strings.Contains(resolvedProfile.Values[1].ResolutionError.Error(), `target "frontendApi"`) {
		t.Fatalf("frontend ResolutionError = %q, want target context", resolvedProfile.Values[1].ResolutionError)
	}
	if strings.Contains(resolvedProfile.ResolutionError.Error(), "secret") {
		t.Fatalf("ResolutionError = %q, must not contain resolved literal secrets", resolvedProfile.ResolutionError)
	}
}

func TestResolveProfile_ReturnsUnavailableForEmptyEnvironmentVariable(t *testing.T) {
	t.Setenv("MYAPPLICATION_EMPTY_CONNECTION_STRING", "")

	configuredProfile := config.Profile{
		Name:         "Production",
		ValueFromEnv: stringPointer("MYAPPLICATION_EMPTY_CONNECTION_STRING"),
	}

	resolvedProfile := profile.ResolveProfile(configuredProfile)

	if resolvedProfile.IsAvailable() {
		t.Fatal("IsAvailable() = true, want false")
	}
	if !errors.Is(resolvedProfile.ResolutionError, profile.ErrEnvironmentVariableEmpty) {
		t.Fatalf("ResolutionError = %v, want ErrEnvironmentVariableEmpty", resolvedProfile.ResolutionError)
	}
	if !strings.Contains(resolvedProfile.ResolutionError.Error(), `MYAPPLICATION_EMPTY_CONNECTION_STRING`) {
		t.Fatalf("ResolutionError = %q, want environment variable name", resolvedProfile.ResolutionError)
	}
	if resolvedProfile.Value != "" {
		t.Fatalf("Value = %q, want empty string", resolvedProfile.Value)
	}
	if resolvedProfile.MaskedValue != "" {
		t.Fatalf("MaskedValue = %q, want empty string", resolvedProfile.MaskedValue)
	}
}

func TestResolveProfile_UnavailableProfileDoesNotAffectOtherProfiles(t *testing.T) {
	literalProfile := config.Profile{
		Name:  "Local",
		Value: stringPointer("Server=localhost;Database=MyApplication;"),
	}
	missingEnvironmentProfile := config.Profile{
		Name:         "Production",
		ValueFromEnv: stringPointer("MYAPPLICATION_PRODUCTION_CONNECTION_STRING"),
	}

	resolvedLiteralProfile := profile.ResolveProfile(literalProfile)
	resolvedMissingEnvironmentProfile := profile.ResolveProfile(missingEnvironmentProfile)

	if !resolvedLiteralProfile.IsAvailable() {
		t.Fatalf("literal IsAvailable() = false, want true (error: %v)", resolvedLiteralProfile.ResolutionError)
	}
	if resolvedMissingEnvironmentProfile.IsAvailable() {
		t.Fatal("missing environment profile IsAvailable() = true, want false")
	}
}
