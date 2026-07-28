package app_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestApplication_ProfileContentsByName_GroupsIncludedTargetsByConfiguredFileOrder(t *testing.T) {
	application := app.NewWithTargets(
		[]config.Target{
			{Name: "frontendApi", File: "frontend/.env.local", Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
			{Name: "database", File: "backend/appsettings.Development.json", Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "redis", File: "backend/appsettings.Development.json", Type: config.TargetTypeJSON, JSONPath: "redis.url"},
			{Name: "workerQueue", File: "worker/config.yaml", Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
		},
		[]config.Profile{{
			Name:      "Staging",
			Protected: true,
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://staging-secret")},
				{Target: "redis", Value: stringPointer("redis://staging-secret")},
				{Target: "frontendApi", Value: stringPointer("https://api.staging.example.test")},
			},
		}},
	)

	contents, err := application.ProfileContentsByName("Staging", app.PreviewOptions{})
	if err != nil {
		t.Fatalf("ProfileContentsByName returned error: %v", err)
	}

	if contents.ProfileName != "Staging" || !contents.Protected || !contents.Available {
		t.Fatalf("contents = %#v, want protected available Staging profile", contents)
	}
	if !contents.Partial || contents.TargetCount != 3 || contents.TotalTargets != 4 || contents.OmittedTargetCount != 1 {
		t.Fatalf("target counts = partial:%v included:%d total:%d omitted:%d, want 3 of 4 partial", contents.Partial, contents.TargetCount, contents.TotalTargets, contents.OmittedTargetCount)
	}
	if len(contents.Files) != 2 {
		t.Fatalf("len(Files) = %d, want 2", len(contents.Files))
	}
	if contents.Files[0].TargetFile != "frontend/.env.local" || len(contents.Files[0].Targets) != 1 || contents.Files[0].Targets[0].TargetName != "frontendApi" {
		t.Fatalf("Files[0] = %#v, want configured-order frontend group", contents.Files[0])
	}
	if contents.Files[1].TargetFile != "backend/appsettings.Development.json" || len(contents.Files[1].Targets) != 2 || contents.Files[1].Targets[0].TargetName != "database" || contents.Files[1].Targets[1].TargetName != "redis" {
		t.Fatalf("Files[1] = %#v, want backend group with database then redis", contents.Files[1])
	}
	if len(contents.OmittedTargets) != 1 || contents.OmittedTargets[0].TargetName != "workerQueue" {
		t.Fatalf("OmittedTargets = %#v, want workerQueue", contents.OmittedTargets)
	}
	for _, fileGroup := range contents.Files {
		for _, target := range fileGroup.Targets {
			if target.ValueVisible || target.Value != "" {
				t.Fatalf("hidden profile contents exposed value for %s: %#v", target.TargetName, target)
			}
		}
	}
}

func TestApplication_ProfileContentsByName_RevealsOnlyIncludedManagedValuesWhenRequested(t *testing.T) {
	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: "backend/appsettings.Development.json", Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: "frontend/.env.local", Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{{
			Name:      "Database Only",
			Protected: true,
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://selected-managed-secret")},
			},
		}},
	)

	contents, err := application.ProfileContentsByName("Database Only", app.PreviewOptions{ValueVisibility: app.ValueVisibilityShown})
	if err != nil {
		t.Fatalf("ProfileContentsByName returned error: %v", err)
	}

	if len(contents.Files) != 1 || len(contents.Files[0].Targets) != 1 {
		t.Fatalf("Files = %#v, want one included database target", contents.Files)
	}
	target := contents.Files[0].Targets[0]
	if !target.ValueVisible || target.Value != "postgres://selected-managed-secret" {
		t.Fatalf("target value = visible:%v value:%q, want selected managed value", target.ValueVisible, target.Value)
	}
	if len(contents.OmittedTargets) != 1 || contents.OmittedTargets[0].TargetName != "frontendApi" {
		t.Fatalf("OmittedTargets = %#v, want frontendApi descriptor without values", contents.OmittedTargets)
	}
}

func TestApplication_ProfileContentsByName_UnavailableValueReturnsSafeReasonWithoutValue(t *testing.T) {
	t.Setenv("STAGING_DATABASE_URL", "")

	application := app.NewWithTargets(
		[]config.Target{{Name: "database", File: "backend/appsettings.Development.json", Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "database", ValueFromEnv: stringPointer("STAGING_DATABASE_URL")},
			},
		}},
	)

	contents, err := application.ProfileContentsByName("Staging", app.PreviewOptions{ValueVisibility: app.ValueVisibilityShown})
	if err != nil {
		t.Fatalf("ProfileContentsByName returned error: %v", err)
	}

	if contents.Available {
		t.Fatal("contents.Available = true, want false")
	}
	target := contents.Files[0].Targets[0]
	if target.Available || target.ValueVisible || target.Value != "" {
		t.Fatalf("unavailable target = %#v, want no revealed value", target)
	}
	if target.EnvironmentVariableName != "STAGING_DATABASE_URL" || !strings.Contains(target.UnavailableReason, "STAGING_DATABASE_URL") {
		t.Fatalf("unavailable target = %#v, want safe environment reason", target)
	}
}

func TestApplication_ManagedPatchPreviewByName_GroupsHunksAndKeepsDiffCompatible(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old-secret"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")
	workerPath := writeTargetFile(t, projectRoot, "worker/config.yaml", "queue:\n  endpoint: old-queue\n")
	originalDatabaseContents := readFile(t, databasePath)
	originalFrontendContents := readFile(t, frontendPath)
	originalWorkerContents := readFile(t, workerPath)

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
			{Name: "workerQueue", File: workerPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
		},
		[]config.Profile{{
			Name:      "Staging",
			Protected: true,
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://new-secret")},
				{Target: "frontendApi", Value: stringPointer("http://localhost:5173")},
			},
		}},
	)

	preview, err := application.ManagedPatchPreviewByName("Staging", app.PreviewOptions{})
	if err != nil {
		t.Fatalf("ManagedPatchPreviewByName returned error: %v", err)
	}

	if preview.ProfileName != "Staging" || !preview.Protected || !preview.Complete {
		t.Fatalf("preview = %#v, want protected complete Staging preview", preview)
	}
	if !preview.Partial || preview.IncludedTargetCount != 2 || preview.OmittedTargetCount != 1 || preview.TargetCount != 3 {
		t.Fatalf("preview counts = %#v, want 2 included and 1 omitted", preview)
	}
	if len(preview.Files) != 2 {
		t.Fatalf("len(Files) = %d, want 2", len(preview.Files))
	}
	if preview.Files[0].TargetFile != databasePath || preview.Files[0].Hunks[0].Status != app.ManagedPatchStatusWouldUpdate {
		t.Fatalf("Files[0] = %#v, want database would-update hunk", preview.Files[0])
	}
	if preview.Files[1].TargetFile != frontendPath || preview.Files[1].Hunks[0].Status != app.ManagedPatchStatusAlreadyMatches {
		t.Fatalf("Files[1] = %#v, want frontend already-matches hunk", preview.Files[1])
	}
	if preview.Files[0].Hunks[0].CurrentValueVisible || preview.Files[0].Hunks[0].CurrentValue != "" || preview.Files[0].Hunks[0].ProfileValueVisible || preview.Files[0].Hunks[0].ProfileValue != "" {
		t.Fatalf("hidden preview exposed values: %#v", preview.Files[0].Hunks[0])
	}
	if len(preview.OmittedTargets) != 1 || preview.OmittedTargets[0].TargetName != "workerQueue" {
		t.Fatalf("OmittedTargets = %#v, want workerQueue", preview.OmittedTargets)
	}

	diff, err := application.DiffProfileByName("Staging")
	if err != nil {
		t.Fatalf("DiffProfileByName returned error: %v", err)
	}
	if len(diff.WouldUpdate) != 1 || diff.WouldUpdate[0].TargetName != "database" {
		t.Fatalf("WouldUpdate = %#v, want database", diff.WouldUpdate)
	}
	if len(diff.AlreadyMatches) != 1 || diff.AlreadyMatches[0].TargetName != "frontendApi" {
		t.Fatalf("AlreadyMatches = %#v, want frontendApi", diff.AlreadyMatches)
	}
	if len(diff.OmittedTargets) != 1 || diff.OmittedTargets[0].TargetName != "workerQueue" {
		t.Fatalf("OmittedTargets = %#v, want workerQueue", diff.OmittedTargets)
	}
	if len(diff.Unavailable) != 0 {
		t.Fatalf("Unavailable = %#v, want none", diff.Unavailable)
	}

	shownPreview, err := application.ManagedPatchPreviewByName("Staging", app.PreviewOptions{ValueVisibility: app.ValueVisibilityShown})
	if err != nil {
		t.Fatalf("ManagedPatchPreviewByName with values returned error: %v", err)
	}
	shownHunk := shownPreview.Files[0].Hunks[0]
	if !shownHunk.CurrentValueVisible || shownHunk.CurrentValue != "postgres://old-secret" || !shownHunk.ProfileValueVisible || shownHunk.ProfileValue != "postgres://new-secret" {
		t.Fatalf("shown hunk values = %#v, want managed current and profile values", shownHunk)
	}

	if !bytes.Equal(readFile(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed during managed patch preview")
	}
	if !bytes.Equal(readFile(t, frontendPath), originalFrontendContents) {
		t.Fatal("frontend target changed during managed patch preview")
	}
	if !bytes.Equal(readFile(t, workerPath), originalWorkerContents) {
		t.Fatal("worker target changed during managed patch preview")
	}
}

func TestApplication_ManagedPatchPreviewByName_UnavailableValueDoesNotRevealCurrentOrProfileValue(t *testing.T) {
	t.Setenv("STAGING_DATABASE_URL", "")

	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://current-secret"}}`)
	originalDatabaseContents := readFile(t, databasePath)

	application := app.NewWithTargets(
		[]config.Target{{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "database", ValueFromEnv: stringPointer("STAGING_DATABASE_URL")},
			},
		}},
	)

	preview, err := application.ManagedPatchPreviewByName("Staging", app.PreviewOptions{ValueVisibility: app.ValueVisibilityShown})
	if err != nil {
		t.Fatalf("ManagedPatchPreviewByName returned error: %v", err)
	}

	if preview.Complete {
		t.Fatal("preview.Complete = true, want false")
	}
	if len(preview.Files) != 1 || len(preview.Files[0].Hunks) != 1 {
		t.Fatalf("Files = %#v, want one unavailable hunk", preview.Files)
	}
	hunk := preview.Files[0].Hunks[0]
	if hunk.Status != app.ManagedPatchStatusUnavailable {
		t.Fatalf("Status = %q, want unavailable", hunk.Status)
	}
	if hunk.CurrentValueVisible || hunk.CurrentValue != "" || hunk.ProfileValueVisible || hunk.ProfileValue != "" {
		t.Fatalf("unavailable shown hunk exposed values: %#v", hunk)
	}
	if !strings.Contains(hunk.UnavailableReason, "STAGING_DATABASE_URL") || strings.Contains(hunk.UnavailableReason, "postgres://current-secret") {
		t.Fatalf("UnavailableReason = %q, want safe environment reason without current value", hunk.UnavailableReason)
	}
	if !bytes.Equal(readFile(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed during unavailable managed patch preview")
	}
}
