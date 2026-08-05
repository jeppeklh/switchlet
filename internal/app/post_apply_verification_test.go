package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

func TestApplication_ApplyProfile_ReturnsPostApplyVerificationErrorForMismatchWithoutLeakingValues(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writePostApplyTargetFile(t, projectRoot, "config.json", `{"service":{"baseUrl":"https://old.example.test"}}`)

	application := New(
		config.Target{File: targetPath, JSONPath: "service.baseUrl"},
		[]config.Profile{{Name: "Local", Value: stringPointer("https://expected-secret.example.test")}},
	)
	application.readTargetValues = func(targets []config.Target) (map[string]string, error) {
		if len(targets) != 1 || targets[0].Name != defaultTargetName {
			t.Fatalf("read targets = %#v, want only default target", targets)
		}

		return map[string]string{defaultTargetName: "https://current-secret.example.test"}, nil
	}

	_, err := application.ApplyProfileByName("Local")
	if err == nil {
		t.Fatal("ApplyProfileByName returned nil error, want post-apply verification error")
	}
	if !errors.Is(err, ErrPostApplyVerificationFailed) {
		t.Fatalf("ApplyProfileByName returned error %v, want ErrPostApplyVerificationFailed", err)
	}
	var verificationErr PostApplyVerificationError
	if !errors.As(err, &verificationErr) {
		t.Fatalf("ApplyProfileByName returned error %v, want PostApplyVerificationError", err)
	}
	if len(verificationErr.Failures) != 1 {
		t.Fatalf("len(Failures) = %d, want 1", len(verificationErr.Failures))
	}
	failure := verificationErr.Failures[0]
	if failure.TargetName != defaultTargetName || failure.TargetType != config.TargetTypeJSON || failure.Selector != "service.baseUrl" {
		t.Fatalf("failure = %#v, want default JSON target context", failure)
	}
	if failure.Reason != "current value does not match selected profile value" {
		t.Fatalf("failure.Reason = %q, want mismatch reason", failure.Reason)
	}
	for _, forbidden := range []string{"expected-secret", "current-secret"} {
		if strings.Contains(err.Error(), forbidden) || strings.Contains(failure.Reason, forbidden) {
			t.Fatalf("post-apply verification failure leaked value %q: err=%q failure=%#v", forbidden, err, failure)
		}
	}
	if !strings.Contains(string(readPostApplyFile(t, targetPath)), "https://expected-secret.example.test") {
		t.Fatalf("target file = %q, want write to have completed before verification failure", string(readPostApplyFile(t, targetPath)))
	}
}

func TestApplication_ApplyProfile_ReturnsPostApplyVerificationErrorForReadFailure(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writePostApplyTargetFile(t, projectRoot, "config.json", `{"service":{"baseUrl":"https://old.example.test"}}`)

	application := New(
		config.Target{File: targetPath, JSONPath: "service.baseUrl"},
		[]config.Profile{{Name: "Local", Value: stringPointer("https://expected-secret.example.test")}},
	)
	application.readTargetValues = func(targets []config.Target) (map[string]string, error) {
		return nil, editor.TargetError{Target: targets[0], Err: errors.New("read permission denied")}
	}

	_, err := application.ApplyProfileByName("Local")
	if err == nil {
		t.Fatal("ApplyProfileByName returned nil error, want post-apply read verification error")
	}
	if !errors.Is(err, ErrPostApplyVerificationFailed) {
		t.Fatalf("ApplyProfileByName returned error %v, want ErrPostApplyVerificationFailed", err)
	}
	var verificationErr PostApplyVerificationError
	if !errors.As(err, &verificationErr) {
		t.Fatalf("ApplyProfileByName returned error %v, want PostApplyVerificationError", err)
	}
	if len(verificationErr.Failures) != 1 {
		t.Fatalf("len(Failures) = %d, want 1", len(verificationErr.Failures))
	}
	failure := verificationErr.Failures[0]
	if failure.TargetName != defaultTargetName || failure.TargetFile != targetPath || failure.Selector != "service.baseUrl" {
		t.Fatalf("failure = %#v, want target context for read failure", failure)
	}
	if !strings.Contains(failure.Reason, "read permission denied") {
		t.Fatalf("failure.Reason = %q, want read failure reason", failure.Reason)
	}
	if strings.Contains(err.Error(), "expected-secret") || strings.Contains(failure.Reason, "expected-secret") {
		t.Fatalf("post-apply read failure leaked resolved value: err=%q failure=%#v", err, failure)
	}
	if !strings.Contains(string(readPostApplyFile(t, targetPath)), "https://expected-secret.example.test") {
		t.Fatalf("target file = %q, want write to have completed before verification read failure", string(readPostApplyFile(t, targetPath)))
	}
}

func TestApplication_ApplyProfile_VerifiesOnlyIncludedTargetsForPartialProfile(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writePostApplyTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	frontendPath := writePostApplyTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")
	originalFrontendContents := readPostApplyFile(t, frontendPath)
	var readTargets []config.Target

	application := NewWithTargets(
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
	application.readTargetValues = func(targets []config.Target) (map[string]string, error) {
		readTargets = append([]config.Target(nil), targets...)
		return map[string]string{"database": "postgres://new"}, nil
	}

	result, err := application.ApplyProfileByName("Database Only")
	if err != nil {
		t.Fatalf("ApplyProfileByName returned error: %v", err)
	}
	if len(result.Changes) != 1 || result.Changes[0].TargetName != "database" {
		t.Fatalf("Changes = %#v, want only database target", result.Changes)
	}
	if len(readTargets) != 1 || readTargets[0].Name != "database" {
		t.Fatalf("verification read targets = %#v, want only included database target", readTargets)
	}
	if !strings.Contains(string(readPostApplyFile(t, databasePath)), "postgres://new") {
		t.Fatalf("database target = %q, want updated value", string(readPostApplyFile(t, databasePath)))
	}
	if !bytes.Equal(readPostApplyFile(t, frontendPath), originalFrontendContents) {
		t.Fatal("omitted frontend target changed after partial profile apply")
	}
}

func TestApplication_ApplyProfileWithOptions_DryRunDoesNotRunPostApplyVerification(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writePostApplyTargetFile(t, projectRoot, "config.json", `{"service":{"baseUrl":"https://old.example.test"}}`)
	originalContents := readPostApplyFile(t, targetPath)
	verificationCalled := false

	application := New(
		config.Target{File: targetPath, JSONPath: "service.baseUrl"},
		[]config.Profile{{Name: "Local", Value: stringPointer("https://expected-secret.example.test")}},
	)
	application.readTargetValues = func(targets []config.Target) (map[string]string, error) {
		verificationCalled = true
		return nil, fmt.Errorf("verification should not run during dry-run")
	}

	result, err := application.ApplyProfileByNameWithOptions("Local", ApplyOptions{DryRun: true})
	if err != nil {
		t.Fatalf("ApplyProfileByNameWithOptions returned error: %v", err)
	}
	if !result.DryRun {
		t.Fatal("DryRun = false, want true")
	}
	if verificationCalled {
		t.Fatal("post-apply verification ran during dry-run")
	}
	if !bytes.Equal(readPostApplyFile(t, targetPath), originalContents) {
		t.Fatal("target file changed during dry-run")
	}
}

func writePostApplyTargetFile(t *testing.T, rootDir string, relativePath string, contents string) string {
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

func readPostApplyFile(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %q: %v", path, err)
	}

	return contents
}

func stringPointer(value string) *string {
	return &value
}
