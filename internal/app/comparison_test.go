package app_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestApplication_CompareStatus_ReportsOneTargetExactCurrentProfileMatch(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://local"}}`)

	application := app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{
			{
				Name: "Local",
				Values: []config.ProfileValue{
					{Target: "database", Value: stringPointer("postgres://local")},
				},
			},
			{
				Name: "Staging",
				Values: []config.ProfileValue{
					{Target: "database", Value: stringPointer("postgres://staging")},
				},
			},
		},
	)

	status, err := application.CompareStatus()
	if err != nil {
		t.Fatalf("CompareStatus returned error: %v", err)
	}

	if status.Status != app.StatusComparisonMatched {
		t.Fatalf("Status = %q, want matched", status.Status)
	}
	if status.CurrentProfile != "Local" {
		t.Fatalf("CurrentProfile = %q, want Local", status.CurrentProfile)
	}
	if !status.Complete || status.TargetCount != 1 {
		t.Fatalf("Complete/TargetCount = %v/%d, want true/1", status.Complete, status.TargetCount)
	}
	if len(status.Matches) != 1 || status.Matches[0].ProfileName != "Local" {
		t.Fatalf("Matches = %#v, want Local", status.Matches)
	}
	if len(status.MatchedTargets) != 1 || status.MatchedTargets[0].TargetName != "database" || status.MatchedTargets[0].Selector != "database.url" {
		t.Fatalf("MatchedTargets = %#v, want database target descriptor", status.MatchedTargets)
	}
}

func TestApplication_CompareStatus_ReportsMultiTargetExactCompleteProfileMatch(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://local"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{
			{
				Name: "Local",
				Values: []config.ProfileValue{
					{Target: "database", Value: stringPointer("postgres://local")},
					{Target: "frontendApi", Value: stringPointer("http://localhost:5173")},
				},
			},
			{
				Name: "Database Only",
				Values: []config.ProfileValue{
					{Target: "database", Value: stringPointer("postgres://local")},
				},
			},
		},
	)

	status, err := application.CompareStatus()
	if err != nil {
		t.Fatalf("CompareStatus returned error: %v", err)
	}

	if status.Status != app.StatusComparisonMatched || status.CurrentProfile != "Local" {
		t.Fatalf("status = %#v, want Local exact match", status)
	}
	if len(status.MatchedTargets) != 2 {
		t.Fatalf("len(MatchedTargets) = %d, want 2", len(status.MatchedTargets))
	}
	if status.MatchedTargets[0].TargetName != "database" || status.MatchedTargets[1].TargetName != "frontendApi" {
		t.Fatalf("MatchedTargets = %#v, want configured target order", status.MatchedTargets)
	}
	if len(status.PartialMatches) != 1 || status.PartialMatches[0].ProfileName != "Database Only" {
		t.Fatalf("PartialMatches = %#v, want Database Only advisory match", status.PartialMatches)
	}
}

func TestApplication_CompareStatus_ReportsSameFileMultiTargetMatch(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "backend/config.json", `{"database":{"url":"postgres://local"},"redis":{"url":"redis://local"}}`)

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "redis", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "redis.url"},
		},
		[]config.Profile{{
			Name: "Local",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://local")},
				{Target: "redis", Value: stringPointer("redis://local")},
			},
		}},
	)

	status, err := application.CompareStatus()
	if err != nil {
		t.Fatalf("CompareStatus returned error: %v", err)
	}

	if status.Status != app.StatusComparisonMatched || status.CurrentProfile != "Local" {
		t.Fatalf("status = %#v, want Local exact same-file match", status)
	}
	if len(status.MatchedTargets) != 2 || status.MatchedTargets[0].TargetName != "database" || status.MatchedTargets[1].TargetName != "redis" {
		t.Fatalf("MatchedTargets = %#v, want configured same-file target order", status.MatchedTargets)
	}
}

func TestApplication_CompareStatus_RejectsMixedTargetTypesInOneFile(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "backend/config.json", `{"database":{"url":"postgres://local"},"queue":{"endpoint":"local"}}`)

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "workerQueue", File: targetPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
		},
		[]config.Profile{{
			Name: "Local",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://local")},
				{Target: "workerQueue", Value: stringPointer("local")},
			},
		}},
	)

	_, err := application.CompareStatus()
	if err == nil {
		t.Fatal("CompareStatus returned nil error, want mixed same-file target-type failure")
	}
	if !strings.Contains(err.Error(), `target "workerQueue"`) || !strings.Contains(err.Error(), `target file already has "json" reads queued`) {
		t.Fatalf("CompareStatus returned error %q, want grouped read mixed-type context", err)
	}
}

func TestApplication_CompareStatus_ReportsClosestProfilesWhenNoCompleteProfileMatches(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/config.json", `{"database":{"url":"postgres://current"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://current\n")

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{
			{
				Name: "Local",
				Values: []config.ProfileValue{
					{Target: "database", Value: stringPointer("postgres://current")},
					{Target: "frontendApi", Value: stringPointer("http://local")},
				},
			},
			{
				Name: "Staging",
				Values: []config.ProfileValue{
					{Target: "database", Value: stringPointer("postgres://staging")},
					{Target: "frontendApi", Value: stringPointer("http://staging")},
				},
			},
		},
	)

	status, err := application.CompareStatus()
	if err != nil {
		t.Fatalf("CompareStatus returned error: %v", err)
	}

	if status.Status != app.StatusComparisonUnmatched || status.CurrentProfile != "" {
		t.Fatalf("status = %#v, want unmatched without current profile", status)
	}
	if len(status.ClosestProfiles) != 2 {
		t.Fatalf("len(ClosestProfiles) = %d, want 2", len(status.ClosestProfiles))
	}
	if status.ClosestProfiles[0].ProfileName != "Local" || status.ClosestProfiles[0].MatchedTargets != 1 || status.ClosestProfiles[0].TargetCount != 2 {
		t.Fatalf("ClosestProfiles[0] = %#v, want Local with 1 of 2 matches", status.ClosestProfiles[0])
	}
	if status.ClosestProfiles[1].ProfileName != "Staging" || status.ClosestProfiles[1].MatchedTargets != 0 {
		t.Fatalf("ClosestProfiles[1] = %#v, want Staging with 0 matches", status.ClosestProfiles[1])
	}
}

func TestApplication_CompareStatus_ReportsAmbiguousCompleteMatches(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"serviceUrl":"http://localhost:8080"}`)

	application := app.NewWithTargets(
		[]config.Target{{Name: "serviceEndpoint", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "serviceUrl"}},
		[]config.Profile{
			{
				Name: "Local",
				Values: []config.ProfileValue{
					{Target: "serviceEndpoint", Value: stringPointer("http://localhost:8080")},
				},
			},
			{
				Name: "Local Copy",
				Values: []config.ProfileValue{
					{Target: "serviceEndpoint", Value: stringPointer("http://localhost:8080")},
				},
			},
		},
	)

	status, err := application.CompareStatus()
	if err != nil {
		t.Fatalf("CompareStatus returned error: %v", err)
	}

	if status.Status != app.StatusComparisonAmbiguous {
		t.Fatalf("Status = %q, want ambiguous", status.Status)
	}
	if status.CurrentProfile != "" {
		t.Fatalf("CurrentProfile = %q, want empty for ambiguous matches", status.CurrentProfile)
	}
	if len(status.Matches) != 2 || status.Matches[0].ProfileName != "Local" || status.Matches[1].ProfileName != "Local Copy" {
		t.Fatalf("Matches = %#v, want both exact matches", status.Matches)
	}
}

func TestApplication_CompareStatus_ReportsScopedCurrentProfileNames(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/config.json", `{"database":{"url":"postgres://local"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://current\n")

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{
			{
				Name: "Database Only",
				Values: []config.ProfileValue{
					{Target: "database", Value: stringPointer("postgres://local")},
				},
			},
			{
				Name: "Complete Local",
				Values: []config.ProfileValue{
					{Target: "database", Value: stringPointer("postgres://local")},
					{Target: "frontendApi", Value: stringPointer("http://different")},
				},
			},
		},
	)

	status, err := application.CompareStatus()
	if err != nil {
		t.Fatalf("CompareStatus returned error: %v", err)
	}

	if status.Status != app.StatusComparisonUnmatched || status.CurrentProfile != "" {
		t.Fatalf("status = %#v, want unmatched whole-project status with no single exact current profile", status)
	}
	currentNames := status.CurrentProfileNames()
	if len(currentNames) != 1 || currentNames[0] != "Database Only" {
		t.Fatalf("CurrentProfileNames() = %#v, want Database Only", currentNames)
	}
	if len(status.PartialMatches) != 1 {
		t.Fatalf("len(PartialMatches) = %d, want 1", len(status.PartialMatches))
	}
	partialMatch := status.PartialMatches[0]
	if partialMatch.ProfileName != "Database Only" || partialMatch.MatchedTargets != 1 || partialMatch.IncludedTargets != 1 || partialMatch.OmittedTargets != 1 {
		t.Fatalf("PartialMatches[0] = %#v, want 1 included match and 1 omitted target", partialMatch)
	}
}

func TestApplication_CompareStatus_ReportsUnavailableProfilesWithoutBlockingComparableProfiles(t *testing.T) {
	t.Setenv("STAGING_DATABASE_URL", "")

	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/config.json", `{"database":{"url":"postgres://local"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{
			{
				Name: "Local",
				Values: []config.ProfileValue{
					{Target: "database", Value: stringPointer("postgres://local")},
					{Target: "frontendApi", Value: stringPointer("http://localhost:5173")},
				},
			},
			{
				Name: "Staging",
				Values: []config.ProfileValue{
					{Target: "database", ValueFromEnv: stringPointer("STAGING_DATABASE_URL")},
					{Target: "frontendApi", Value: stringPointer("https://api.staging.example.test")},
				},
			},
		},
	)

	status, err := application.CompareStatus()
	if err != nil {
		t.Fatalf("CompareStatus returned error: %v", err)
	}

	if status.Status != app.StatusComparisonMatched || status.CurrentProfile != "Local" {
		t.Fatalf("status = %#v, want Local match despite unavailable Staging", status)
	}
	if len(status.UnavailableProfiles) != 1 || status.UnavailableProfiles[0].ProfileName != "Staging" {
		t.Fatalf("UnavailableProfiles = %#v, want Staging", status.UnavailableProfiles)
	}
	unavailableValues := status.UnavailableProfiles[0].Values
	if len(unavailableValues) != 1 || unavailableValues[0].TargetName != "database" || unavailableValues[0].EnvironmentVariableName != "STAGING_DATABASE_URL" {
		t.Fatalf("unavailable values = %#v, want Staging database environment value", unavailableValues)
	}
}

func TestApplication_DiffProfileByName_GroupsAlreadyMatchingAndWouldUpdateTargets(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/config.json", `{"database":{"url":"postgres://old"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://staging")},
				{Target: "frontendApi", Value: stringPointer("http://localhost:5173")},
			},
		}},
	)

	diff, err := application.DiffProfileByName("Staging")
	if err != nil {
		t.Fatalf("DiffProfileByName returned error: %v", err)
	}

	if diff.ProfileName != "Staging" || !diff.Complete {
		t.Fatalf("diff = %#v, want complete Staging diff", diff)
	}
	if len(diff.WouldUpdate) != 1 || diff.WouldUpdate[0].TargetName != "database" {
		t.Fatalf("WouldUpdate = %#v, want database", diff.WouldUpdate)
	}
	if len(diff.AlreadyMatches) != 1 || diff.AlreadyMatches[0].TargetName != "frontendApi" {
		t.Fatalf("AlreadyMatches = %#v, want frontendApi", diff.AlreadyMatches)
	}
	if len(diff.Unavailable) != 0 || len(diff.OmittedTargets) != 0 {
		t.Fatalf("Unavailable/OmittedTargets = %#v/%#v, want none", diff.Unavailable, diff.OmittedTargets)
	}
}

func TestApplication_DiffProfileByName_UsesGroupedReadsForSameFileTargets(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "backend/config.json", `{"database":{"url":"postgres://old"},"redis":{"url":"redis://staging"}}`)

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "redis", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "redis.url"},
		},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://staging")},
				{Target: "redis", Value: stringPointer("redis://staging")},
			},
		}},
	)

	diff, err := application.DiffProfileByName("Staging")
	if err != nil {
		t.Fatalf("DiffProfileByName returned error: %v", err)
	}

	if len(diff.WouldUpdate) != 1 || diff.WouldUpdate[0].TargetName != "database" {
		t.Fatalf("WouldUpdate = %#v, want database", diff.WouldUpdate)
	}
	if len(diff.AlreadyMatches) != 1 || diff.AlreadyMatches[0].TargetName != "redis" {
		t.Fatalf("AlreadyMatches = %#v, want redis", diff.AlreadyMatches)
	}
}

func TestApplication_DiffProfileByName_RejectsMixedTargetTypesInOneFile(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "backend/config.json", `{"database":{"url":"postgres://old"},"queue":{"endpoint":"old"}}`)

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "workerQueue", File: targetPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
		},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://staging")},
				{Target: "workerQueue", Value: stringPointer("staging")},
			},
		}},
	)

	_, err := application.DiffProfileByName("Staging")
	if err == nil {
		t.Fatal("DiffProfileByName returned nil error, want mixed same-file target-type failure")
	}
	if !strings.Contains(err.Error(), `target "workerQueue"`) || !strings.Contains(err.Error(), `target file already has "json" reads queued`) {
		t.Fatalf("DiffProfileByName returned error %q, want grouped read mixed-type context", err)
	}
}

func TestApplication_DiffProfileByName_ReportsUnavailableValuesAndOmittedTargets(t *testing.T) {
	t.Setenv("MISSING_DATABASE_URL", "")

	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/config.json", `{"database":{"url":"postgres://old"}}`)
	missingFrontendPath := filepath.Join(projectRoot, "frontend", ".env.local")

	application := app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: missingFrontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{{
			Name: "Database Only",
			Values: []config.ProfileValue{
				{Target: "database", ValueFromEnv: stringPointer("MISSING_DATABASE_URL")},
			},
		}},
	)

	diff, err := application.DiffProfileByName("Database Only")
	if err != nil {
		t.Fatalf("DiffProfileByName returned error: %v", err)
	}

	if diff.Complete {
		t.Fatal("Complete = true, want false for unresolved environment value")
	}
	if len(diff.Unavailable) != 1 {
		t.Fatalf("len(Unavailable) = %d, want 1", len(diff.Unavailable))
	}
	if diff.Unavailable[0].TargetName != "database" || diff.Unavailable[0].EnvironmentVariableName != "MISSING_DATABASE_URL" {
		t.Fatalf("Unavailable[0] = %#v, want database environment value", diff.Unavailable[0])
	}
	if len(diff.OmittedTargets) != 1 || diff.OmittedTargets[0].TargetName != "frontendApi" {
		t.Fatalf("OmittedTargets = %#v, want frontendApi", diff.OmittedTargets)
	}
	if len(diff.WouldUpdate) != 0 || len(diff.AlreadyMatches) != 0 {
		t.Fatalf("WouldUpdate/AlreadyMatches = %#v/%#v, want none", diff.WouldUpdate, diff.AlreadyMatches)
	}
}

func TestApplication_DiffProfileByName_AllowsProtectedProfilesWithoutApproval(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "services/config.json", `{"api":{"url":"https://api.production.example.test"}}`)

	application := app.NewWithTargets(
		[]config.Target{{Name: "serviceEndpoint", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "api.url"}},
		[]config.Profile{{
			Name:      "Production",
			Protected: true,
			Values: []config.ProfileValue{
				{Target: "serviceEndpoint", Value: stringPointer("https://api.production.example.test")},
			},
		}},
	)

	diff, err := application.DiffProfileByName("Production")
	if err != nil {
		t.Fatalf("DiffProfileByName returned error: %v", err)
	}

	if !diff.Protected {
		t.Fatal("Protected = false, want true")
	}
	if len(diff.AlreadyMatches) != 1 || diff.AlreadyMatches[0].TargetName != "serviceEndpoint" {
		t.Fatalf("AlreadyMatches = %#v, want protected service endpoint match", diff.AlreadyMatches)
	}
}

func TestApplication_DiffProfileByName_ReturnsProfileNotFound(t *testing.T) {
	application := app.NewWithTargets(
		[]config.Target{{Name: "database", File: "config.json", Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{{
			Name: "Local",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://local")},
			},
		}},
	)

	_, err := application.DiffProfileByName("Missing")
	if err == nil {
		t.Fatal("DiffProfileByName returned nil error, want profile-not-found error")
	}
	if !errors.Is(err, app.ErrProfileNotFound) {
		t.Fatalf("DiffProfileByName returned error %v, want ErrProfileNotFound", err)
	}
}

func TestApplication_DiffProfileByName_TargetReadErrorsDoNotLeakValuesOrWrite(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "backend/config.json", `{"database":{"password":"current-secret"}}`)
	originalContents := readFile(t, targetPath)

	application := app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("resolved-secret")},
			},
		}},
	)

	_, err := application.DiffProfileByName("Staging")
	if err == nil {
		t.Fatal("DiffProfileByName returned nil error, want target-read error")
	}
	if !strings.Contains(err.Error(), `target "database"`) || !strings.Contains(err.Error(), `missing segment "url"`) {
		t.Fatalf("DiffProfileByName returned error %q, want target-read context", err)
	}
	for _, forbidden := range []string{"current-secret", "resolved-secret"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("DiffProfileByName returned error %q, must not contain %q", err, forbidden)
		}
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("target file changed during failed diff comparison")
	}
}
