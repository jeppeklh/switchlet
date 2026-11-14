package app_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestHealthChecks_EnvironmentReadinessGroupsTypedUses(t *testing.T) {
	t.Setenv("READINESS_MISSING", "")
	if err := os.Unsetenv("READINESS_MISSING"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("READINESS_EMPTY", "")
	t.Setenv("READINESS_SHARED", "ENV_SECRET_SENTINEL")
	t.Setenv("READINESS_FALSE", "false")
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	if err := os.WriteFile(path, []byte(`{"text":"current","flag":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	targets := []config.Target{{Name: "text", File: path, Type: config.TargetTypeJSON, JSONPath: "text"}, {Name: "flag", File: path, Type: config.TargetTypeJSON, JSONPath: "flag", ValueType: config.ValueTypeBoolean}}
	profiles := []config.Profile{
		{Name: "Missing", Values: []config.ProfileValue{{Target: "text", ValueFromEnv: stringPointer("READINESS_MISSING")}}},
		{Name: "Empty", Values: []config.ProfileValue{{Target: "text", ValueFromEnv: stringPointer("READINESS_EMPTY")}}},
		{Name: "Shared", Values: []config.ProfileValue{{Target: "text", ValueFromEnv: stringPointer("READINESS_SHARED")}, {Target: "flag", ValueFromEnv: stringPointer("READINESS_SHARED")}}},
		{Name: "Ready", Values: []config.ProfileValue{{Target: "flag", ValueFromEnv: stringPointer("READINESS_FALSE")}}},
	}
	application := app.NewWithTargets(targets, profiles)
	var check app.HealthCheck
	for _, candidate := range application.HealthChecks() {
		if candidate.Name == "profile_availability" {
			check = candidate
		}
	}
	if check.Status != app.HealthCheckWarning || len(check.Environment) != 4 {
		t.Fatalf("readiness = %#v", check)
	}
	states := []app.EnvironmentState{app.EnvironmentMissing, app.EnvironmentEmpty, app.EnvironmentInvalidType, app.EnvironmentReady}
	for index, want := range states {
		if check.Environment[index].State != want {
			t.Fatalf("state = %s, want %s", check.Environment[index].State, want)
		}
	}
	shared := check.Environment[2]
	if len(shared.Uses) != 2 || shared.Uses[0].State != app.EnvironmentReady || shared.Uses[1].State != app.EnvironmentInvalidType || shared.Uses[1].ValueType != config.ValueTypeBoolean {
		t.Fatalf("shared uses = %#v", shared.Uses)
	}
	if strings.Contains(fmt.Sprint(check), "ENV_SECRET_SENTINEL") {
		t.Fatal("readiness leaked environment value")
	}
	for _, item := range application.Profiles() {
		for _, value := range item.Values {
			for _, requirement := range check.Environment {
				for _, use := range requirement.Uses {
					if use.ProfileName == item.Name && use.TargetName == value.TargetName && value.Available != (use.State == app.EnvironmentReady) {
						t.Fatal("readiness disagrees with resolver")
					}
				}
			}
		}
	}
}
