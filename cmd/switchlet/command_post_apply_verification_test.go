package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestApplyCommandError_PostApplyVerificationFailureIsValueSafeRuntimeError(t *testing.T) {
	verificationErr := app.PostApplyVerificationError{
		ProfileName: "Staging",
		Failures: []app.PostApplyVerificationFailure{{
			TargetDescriptor: app.TargetDescriptor{
				TargetName:   "database",
				TargetFile:   "/repo/backend/appsettings.Development.json",
				TargetType:   config.TargetTypeJSON,
				SelectorName: "jsonPath",
				Selector:     "database.url",
			},
			Reason: "current value does not match selected profile value",
		}},
	}

	commandErr := applyCommandError(commandOutputOptions{}, false, app.Application{}, "Staging", verificationErr, "/repo")
	if exitCodeForError(commandErr) != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", exitCodeForError(commandErr), runtimeExitCode)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := writeCommandError(commandErr, &stdout, &stderr); err != nil {
		t.Fatalf("writeCommandError returned error: %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty text error stdout", stdout.String())
	}

	for _, expected := range []string{
		`Writes completed for profile "Staging"`,
		"could not confirm the final managed state",
		"Verification failures:",
		"database [json]",
		"file: backend/appsettings.Development.json",
		"jsonPath: database.url",
		"current value does not match selected profile value",
		"switchlet status",
		"switchlet diff Staging",
	} {
		if !strings.Contains(stderr.String(), expected) {
			t.Fatalf("stderr %q does not contain %q", stderr.String(), expected)
		}
	}
	for _, forbidden := range []string{"expected-secret", "current-secret"} {
		if strings.Contains(stderr.String(), forbidden) {
			t.Fatalf("stderr %q must not contain raw value %q", stderr.String(), forbidden)
		}
	}
}

func TestApplyCommandError_PostApplyVerificationFailureJSONIsStructuredAndValueSafe(t *testing.T) {
	verificationErr := app.PostApplyVerificationError{
		ProfileName: "Staging",
		Failures: []app.PostApplyVerificationFailure{{
			TargetDescriptor: app.TargetDescriptor{
				TargetName:   "frontendApi",
				TargetFile:   "/repo/frontend/.env.local",
				TargetType:   config.TargetTypeDotenv,
				SelectorName: "key",
				Selector:     "VITE_API_URL",
			},
			Reason: "target could not be read after write",
		}},
	}

	commandErr := applyCommandError(commandOutputOptions{}, true, app.Application{}, "Staging", verificationErr, "/repo")
	if exitCodeForError(commandErr) != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", exitCodeForError(commandErr), runtimeExitCode)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := writeCommandError(commandErr, &stdout, &stderr); err != nil {
		t.Fatalf("writeCommandError returned error: %v", err)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty JSON error stderr", stderr.String())
	}

	var payload struct {
		Error struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal JSON error: %v\noutput: %q", err, stdout.String())
	}
	if payload.Error.Kind != "post_apply_verification_failed" {
		t.Fatalf("error.kind = %q, want post_apply_verification_failed", payload.Error.Kind)
	}
	for _, expected := range []string{"Writes completed", "frontendApi [dotenv]", "VITE_API_URL", "target could not be read after write"} {
		if !strings.Contains(payload.Error.Message, expected) {
			t.Fatalf("error.message = %q, want %q", payload.Error.Message, expected)
		}
	}
	for _, forbidden := range []string{"expected-secret", "current-secret"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("stdout %q must not contain raw value %q", stdout.String(), forbidden)
		}
	}
}
