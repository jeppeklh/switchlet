package profile_test

import (
	"errors"
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
	if resolvedProfile.MaskedValue != "Server=localhost;Database=MyApplication;Password=****;" {
		t.Fatalf("MaskedValue = %q, want masked literal connection string", resolvedProfile.MaskedValue)
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
	if resolvedProfile.MaskedValue != "Server=test;Database=MyApplication;Pwd=****;" {
		t.Fatalf("MaskedValue = %q, want masked environment value", resolvedProfile.MaskedValue)
	}
	if resolvedProfile.ResolutionError != nil {
		t.Fatalf("ResolutionError = %v, want nil", resolvedProfile.ResolutionError)
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
