package editor

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDiscoverTargetFileCandidates_ReturnsSortedInspectableJSONFiles(t *testing.T) {
	projectRoot := t.TempDir()

	writeTargetFile(t, projectRoot, "root.json", `{"serviceUrl":"https://old.example.test"}`)
	writeTargetFile(t, projectRoot, ".env.local", "VITE_API_URL=http://localhost:5173\n")
	writeTargetFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)
	writeTargetFile(t, projectRoot, "src/appsettings.Development.json", `{"ConnectionStrings":{"DefaultConnection":"Server=localhost;Database=App;"}}`)
	writeTargetFile(t, projectRoot, "invalid.json", `{`)
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
		"root.json",
		filepath.Join("config", "runtime.json"),
		filepath.Join("src", "appsettings.Development.json"),
	}
	if !reflect.DeepEqual(gotRelativePaths, wantRelativePaths) {
		t.Fatalf("relative paths = %#v, want %#v", gotRelativePaths, wantRelativePaths)
	}
}

func TestDiscoverTargetFileCandidates_SkipsObviousDependencyAndBuildDirectories(t *testing.T) {
	projectRoot := t.TempDir()

	writeTargetFile(t, projectRoot, "config/runtime.json", `{"serviceUrl":"https://runtime.example.test"}`)
	writeTargetFile(t, projectRoot, "src/MyApplication/appsettings.Development.json", `{"ConnectionStrings":{"DefaultConnection":"Server=localhost;Database=App;"}}`)
	writeTargetFile(t, projectRoot, "node_modules/pkg/config.json", `{"serviceUrl":"https://ignored.example.test"}`)
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
		filepath.Join("config", "runtime.json"),
		filepath.Join("src", "MyApplication", "appsettings.Development.json"),
	}
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
					Children: []StringTargetNode{{
						Name:       "url",
						JSONPath:   "database.primary.url",
						Selectable: true,
					}},
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
