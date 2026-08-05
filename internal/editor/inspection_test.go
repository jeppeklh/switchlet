package editor

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
)

func TestDiscoverTargetFileCandidates_ReturnsSortedInspectableTargetFiles(t *testing.T) {
	projectRoot := t.TempDir()

	writeTargetFile(t, projectRoot, "root.json", `{"serviceUrl":"https://old.example.test"}`)
	writeTargetFile(t, projectRoot, "worker.yaml", "queue:\n  endpoint: https://old.example.test\n")
	writeTargetFile(t, projectRoot, ".env.local", "VITE_API_URL=http://localhost:5173\n")
	writeTargetFile(t, projectRoot, "services/prod.env", "SERVICE_URL=https://old.example.test\n")
	writeTargetFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)
	writeTargetFile(t, projectRoot, "src/appsettings.Development.json", `{"ConnectionStrings":{"DefaultConnection":"Server=localhost;Database=App;"}}`)
	writeTargetFile(t, projectRoot, "invalid.json", `{`)
	writeTargetFile(t, projectRoot, "numbers.yaml", "queue:\n  retries: 3\n")
	writeTargetFile(t, projectRoot, "arrays-only.json", `{"services":[{"baseUrl":"https://old.example.test"}]}`)
	writeTargetFile(t, projectRoot, ".hidden/ignored.json", `{"serviceUrl":"https://hidden.example.test"}`)

	candidates, err := DiscoverTargetFileCandidates(projectRoot)
	if err != nil {
		t.Fatalf("DiscoverTargetFileCandidates returned error: %v", err)
	}

	gotRelativePaths := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		gotRelativePaths = append(gotRelativePaths, candidate.RelativePath)

		wantPath := filepath.Join(projectRoot, filepath.FromSlash(candidate.RelativePath))
		if candidate.Path != wantPath {
			t.Fatalf("candidate path = %q, want %q", candidate.Path, wantPath)
		}
	}

	wantRelativePaths := []string{
		".env.local",
		filepath.Join("services", "prod.env"),
		filepath.Join("src", "appsettings.Development.json"),
		filepath.Join("config", "runtime.json"),
		"root.json",
		"worker.yaml",
	}
	if !reflect.DeepEqual(gotRelativePaths, wantRelativePaths) {
		t.Fatalf("relative paths = %#v, want %#v", gotRelativePaths, wantRelativePaths)
	}
	for _, candidate := range candidates {
		if candidate.RelativePath == "worker.yaml" && candidate.Type != config.TargetTypeYAML {
			t.Fatalf("worker.yaml candidate type = %q, want %q", candidate.Type, config.TargetTypeYAML)
		}
		if candidate.RelativePath == filepath.Join("services", "prod.env") && candidate.Type != config.TargetTypeDotenv {
			t.Fatalf("services/prod.env candidate type = %q, want %q", candidate.Type, config.TargetTypeDotenv)
		}
	}
}

func TestDiscoverTargetFileCandidates_SkipsObviousDependencyAndBuildDirectories(t *testing.T) {
	projectRoot := t.TempDir()

	writeTargetFile(t, projectRoot, "config/runtime.json", `{"serviceUrl":"https://runtime.example.test"}`)
	writeTargetFile(t, projectRoot, "src/MyApplication/appsettings.Development.json", `{"ConnectionStrings":{"DefaultConnection":"Server=localhost;Database=App;"}}`)
	writeTargetFile(t, projectRoot, "bower_components/pkg/config.json", `{"serviceUrl":"https://ignored.example.test"}`)
	writeTargetFile(t, projectRoot, "build/config.json", `{"serviceUrl":"https://ignored.example.test"}`)
	writeTargetFile(t, projectRoot, "coverage/config.json", `{"serviceUrl":"https://ignored.example.test"}`)
	writeTargetFile(t, projectRoot, "dist/config.json", `{"serviceUrl":"https://ignored.example.test"}`)
	writeTargetFile(t, projectRoot, "generated/config.json", `{"serviceUrl":"https://ignored.example.test"}`)
	writeTargetFile(t, projectRoot, "node_modules/pkg/config.json", `{"serviceUrl":"https://ignored.example.test"}`)
	writeTargetFile(t, projectRoot, "out/config.json", `{"serviceUrl":"https://ignored.example.test"}`)
	writeTargetFile(t, projectRoot, "target/classes/application.yaml", "serviceUrl: https://ignored.example.test\n")
	writeTargetFile(t, projectRoot, "vendor/pkg/config.json", `{"serviceUrl":"https://ignored.example.test"}`)
	writeTargetFile(t, projectRoot, "src/MyApplication/bin/Debug/net8.0/appsettings.json", `{"ConnectionStrings":{"DefaultConnection":"Server=ignored;Database=Bin;"}}`)
	writeTargetFile(t, projectRoot, "src/MyApplication/obj/Debug/net8.0/appsettings.json", `{"ConnectionStrings":{"DefaultConnection":"Server=ignored;Database=Obj;"}}`)

	candidates, err := DiscoverTargetFileCandidates(projectRoot)
	if err != nil {
		t.Fatalf("DiscoverTargetFileCandidates returned error: %v", err)
	}

	gotRelativePaths := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		gotRelativePaths = append(gotRelativePaths, candidate.RelativePath)
	}

	wantRelativePaths := []string{
		filepath.Join("src", "MyApplication", "appsettings.Development.json"),
		filepath.Join("config", "runtime.json"),
	}
	if !reflect.DeepEqual(gotRelativePaths, wantRelativePaths) {
		t.Fatalf("relative paths = %#v, want %#v", gotRelativePaths, wantRelativePaths)
	}
}

func TestDiscoverTargetFileCandidates_SkipsGitignoredDirectories(t *testing.T) {
	projectRoot := t.TempDir()

	writeTargetFile(t, projectRoot, ".gitignore", "generated-config/\n")
	writeTargetFile(t, projectRoot, "config/runtime.json", `{"serviceUrl":"https://runtime.example.test"}`)
	writeTargetFile(t, projectRoot, "generated-config/appsettings.Development.json", `{"ConnectionStrings":{"DefaultConnection":"Server=ignored;Database=App;"}}`)

	candidates, err := DiscoverTargetFileCandidates(projectRoot)
	if err != nil {
		t.Fatalf("DiscoverTargetFileCandidates returned error: %v", err)
	}

	gotRelativePaths := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		gotRelativePaths = append(gotRelativePaths, candidate.RelativePath)
	}

	wantRelativePaths := []string{filepath.Join("config", "runtime.json")}
	if !reflect.DeepEqual(gotRelativePaths, wantRelativePaths) {
		t.Fatalf("relative paths = %#v, want %#v", gotRelativePaths, wantRelativePaths)
	}
}

func TestDiscoverTargetFileCandidates_DoesNotHideGitignoredLocalTargetFiles(t *testing.T) {
	projectRoot := t.TempDir()

	writeTargetFile(t, projectRoot, ".gitignore", "appsettings.Development.json\n")
	writeTargetFile(t, projectRoot, "appsettings.Development.json", `{"ConnectionStrings":{"DefaultConnection":"Server=local;Database=App;"}}`)

	candidates, err := DiscoverTargetFileCandidates(projectRoot)
	if err != nil {
		t.Fatalf("DiscoverTargetFileCandidates returned error: %v", err)
	}

	gotRelativePaths := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		gotRelativePaths = append(gotRelativePaths, candidate.RelativePath)
	}

	wantRelativePaths := []string{"appsettings.Development.json"}
	if !reflect.DeepEqual(gotRelativePaths, wantRelativePaths) {
		t.Fatalf("relative paths = %#v, want %#v", gotRelativePaths, wantRelativePaths)
	}
}

func TestInspectStringTargets_ReturnsHierarchicalNodes(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=App;",
    "RetryCount": 3
  },
  "AllowedHosts": "*",
  "FeatureFlags": {
    "beta": true
  },
  "database": {
    "primary": {
      "api.url": "https://api.example.test",
      "url": "postgres://old",
      "port": 5432
    },
    "replicas": [
      "postgres://replica"
    ],
    "secondary": {
      "credentials": {
        "username": "postgres",
        "password": "secret"
      }
    }
  }
}
`)+"\n")

	nodes, err := InspectStringTargets(targetPath)
	if err != nil {
		t.Fatalf("InspectStringTargets returned error: %v", err)
	}

	wantNodes := []StringTargetNode{
		{
			Name:     "ConnectionStrings",
			JSONPath: "ConnectionStrings",
			Children: []StringTargetNode{{
				Name:       "DefaultConnection",
				JSONPath:   "ConnectionStrings.DefaultConnection",
				Selectable: true,
			}},
		},
		{
			Name:     "database",
			JSONPath: "database",
			Children: []StringTargetNode{
				{
					Name:     "primary",
					JSONPath: "database.primary",
					Children: []StringTargetNode{
						{
							Name:       "api.url",
							JSONPath:   `database.primary.api\.url`,
							Selectable: true,
						},
						{
							Name:       "url",
							JSONPath:   "database.primary.url",
							Selectable: true,
						},
					},
				},
				{
					Name:     "secondary",
					JSONPath: "database.secondary",
					Children: []StringTargetNode{{
						Name:     "credentials",
						JSONPath: "database.secondary.credentials",
						Children: []StringTargetNode{
							{
								Name:       "password",
								JSONPath:   "database.secondary.credentials.password",
								Selectable: true,
							},
							{
								Name:       "username",
								JSONPath:   "database.secondary.credentials.username",
								Selectable: true,
							},
						},
					}},
				},
			},
		},
		{
			Name:       "AllowedHosts",
			JSONPath:   "AllowedHosts",
			Selectable: true,
		},
	}
	if !reflect.DeepEqual(nodes, wantNodes) {
		t.Fatalf("nodes = %#v, want %#v", nodes, wantNodes)
	}
}

func TestInspectStringTargets_ReturnsErrorWhenNoSelectablePathsExist(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"services":[{"baseUrl":"https://old.example.test"}],"featureFlags":{"beta":true}}`)

	_, err := InspectStringTargets(targetPath)
	if err == nil {
		t.Fatal("InspectStringTargets returned nil error, want no-selectable-paths error")
	}
	if !strings.Contains(err.Error(), "does not contain any existing string-valued JSON paths") {
		t.Fatalf("InspectStringTargets returned error %q, want no-selectable-paths error", err)
	}
}

func TestInspectStringTargets_ReturnsErrorForNonObjectRoot(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `[]`)

	_, err := InspectStringTargets(targetPath)
	if err == nil {
		t.Fatal("InspectStringTargets returned nil error, want root-object error")
	}
	if !strings.Contains(err.Error(), "must contain a JSON object at the root") {
		t.Fatalf("InspectStringTargets returned error %q, want root-object error", err)
	}
}
