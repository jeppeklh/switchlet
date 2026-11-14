package app_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestProfileImport_AllFormatsAndTypes(t *testing.T) {
	formats := []struct {
		kind           config.TargetType
		file, contents string
	}{
		{config.TargetTypeJSON, "source.json", `{"text":"private-value","flag":false,"count":0,"rate":1.25}`},
		{config.TargetTypeYAML, "source.yaml", "text: private-value\nflag: false\ncount: 0\nrate: 1.25\n"},
		{config.TargetTypeTOML, "source.toml", "text = \"private-value\"\nflag = false\ncount = 0\nrate = 1.25\n"},
		{config.TargetTypeDotenv, ".env.source", "text=private-value\nflag=false\ncount=0\nrate=1.25\n"},
	}
	for _, format := range formats {
		t.Run(string(format.kind), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, format.file)
			if err := os.WriteFile(path, []byte(format.contents), 0600); err != nil {
				t.Fatal(err)
			}
			document := app.ConfigEditDocument{ProjectRoot: root, Profiles: []config.Profile{{Name: "Base", Protected: true}}}
			names := []string{"text", "flag", "count", "rate"}
			kinds := []config.ValueType{config.ValueTypeString, config.ValueTypeBoolean, config.ValueTypeInteger, config.ValueTypeNumber}
			values := []string{"private-value", "false", "0", "1.25"}
			for index, name := range names {
				document.Targets = append(document.Targets, config.Target{Name: name, File: filepath.Join(root, "destination"), Type: format.kind, ValueType: kinds[index]})
				target := &document.Targets[index]
				switch format.kind {
				case config.TargetTypeJSON:
					target.JSONPath = name
				case config.TargetTypeYAML:
					target.YAMLPath = name
				case config.TargetTypeTOML:
					target.TOMLPath = name
				case config.TargetTypeDotenv:
					target.Key = name
				}
				document.Profiles[0].Values = append(document.Profiles[0].Values, config.ProfileValue{Target: name, ValueFromEnv: stringPointer("UNRESOLVED_IMPORT_" + name), ValueType: kinds[index]})
			}
			workflow := app.DefaultConfigEditWorkflow()
			mappings := workflow.ProfileImportMappings(document)
			mappings[0].SourceFile = format.file
			prepared, err := workflow.PrepareProfileImport(document, "Base", mappings)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range prepared.Values {
				if !row.Available || row.Error != "" {
					t.Fatalf("row = %#v", row)
				}
			}
			if strings.Contains(fmt.Sprint(prepared.Values), "private-value") {
				t.Fatal("review metadata leaked value")
			}
			// The source can disappear after review: materializing and saving the draft
			// must continue to use the captured values.
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			draft, err := workflow.ProfileDraftFromImport(document, prepared, "Imported", names)
			if err != nil {
				t.Fatal(err)
			}
			if !draft.Protected {
				t.Fatal("protection not inherited")
			}
			for index, value := range draft.Values {
				if value.LiteralValue != values[index] || value.Source != app.ProfileSourceLiteral || value.EnvironmentVariableName != "" || !value.Included {
					t.Fatalf("value = %#v", value)
				}
				if value.TargetFile != document.Targets[index].File {
					t.Fatal("destination was retargeted")
				}
			}
			partial, err := workflow.ProfileDraftFromImport(document, prepared, "Partial", []string{"flag"})
			if err != nil {
				t.Fatal(err)
			}
			if partial.Values[0].Source != app.ProfileSourceEnvironment || partial.Values[0].EnvironmentVariableName != "UNRESOLVED_IMPORT_text" {
				t.Fatal("base environment reference lost")
			}
		})
	}
}

func TestProfileImport_PartialErrorsAndValidation(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.json")
	if err := os.WriteFile(source, []byte(`{"good":"source-secret","wrong":true,"empty":""}`), 0600); err != nil {
		t.Fatal(err)
	}
	document := app.ConfigEditDocument{ProjectRoot: root, Profiles: []config.Profile{{Name: "Base", Values: []config.ProfileValue{{Target: "good", Value: stringPointer("base")}}}}}
	for _, name := range []string{"good", "missing", "wrong", "empty"} {
		document.Targets = append(document.Targets, config.Target{Name: name, File: filepath.Join(root, "destination.json"), Type: config.TargetTypeJSON, JSONPath: name})
	}
	workflow := app.DefaultConfigEditWorkflow()
	mappings := workflow.ProfileImportMappings(document)
	mappings[0].SourceFile = "source.json"
	prepared, err := workflow.PrepareProfileImport(document, "Base", mappings)
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.Values[0].Available || prepared.Values[1].Error == "" || prepared.Values[2].Error == "" || prepared.Values[3].Error == "" {
		t.Fatalf("rows = %#v", prepared.Values)
	}
	draft, err := workflow.ProfileDraftFromImport(document, prepared, "Imported", []string{"good"})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Values[1].Included || draft.Values[2].Included {
		t.Fatal("omitted base entries were added")
	}
	for _, tc := range []struct {
		name     string
		selected []string
	}{{"Base", []string{"good"}}, {" ", []string{"good"}}, {"Imported", nil}, {"Imported", []string{"missing"}}, {"Imported", []string{"wrong"}}, {"Imported", []string{"empty"}}, {"Imported", []string{"unknown"}}} {
		if _, err := workflow.ProfileDraftFromImport(document, prepared, tc.name, tc.selected); err == nil {
			t.Fatalf("accepted invalid selection/name: %#v", tc)
		}
	}
	changed := document
	changed.Targets = append([]config.Target(nil), document.Targets...)
	changed.Targets[0].JSONPath = "retargeted"
	if _, err := workflow.ProfileDraftFromImport(changed, prepared, "Imported", []string{"good"}); err == nil {
		t.Fatal("accepted changed destinations")
	}
	changed = document
	changed.Profiles = []config.Profile{{Name: "Base", Protected: true, Values: document.Profiles[0].Values}}
	if _, err := workflow.ProfileDraftFromImport(changed, prepared, "Imported", []string{"good"}); err == nil {
		t.Fatal("accepted changed base")
	}
	mappings[0].SourceFile = "source.yaml"
	wrongFormat, err := workflow.PrepareProfileImport(document, "Base", mappings)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range wrongFormat.Values {
		if row.Available || !strings.Contains(row.Error, "format") {
			t.Fatalf("cross-format accepted: %#v", row)
		}
	}
	for _, bad := range [][]app.ConfigEditImportMapping{{{TargetFile: "unknown", SourceFile: source}}, {mappings[0], mappings[0]}} {
		if _, err := workflow.PrepareProfileImport(document, "Base", bad); err == nil {
			t.Fatal("accepted invalid mappings")
		}
	}
}

func TestProfileImport_SaveUsesSnapshotAndExistingSafeguards(t *testing.T) {
	for _, failure := range []string{"none", "stale config", "gitignore"} {
		t.Run(failure, func(t *testing.T) {
			workflow := app.DefaultConfigEditWorkflow()
			if failure == "gitignore" {
				workflow = app.NewConfigEditWorkflow(app.ConfigEditDependencies{EnsureConfigIgnored: func(string) (bool, error) { return false, errors.New("blocked ignore write") }})
			}
			root, path := singleTargetConfig(t, "base-secret")
			document := loadConfigEditDocument(t, workflow, root, path)
			source := filepath.Join(root, "source.json")
			originalSource := `{"service":{"baseUrl":"imported-secret"}}`
			if err := os.WriteFile(source, []byte(originalSource), 0600); err != nil {
				t.Fatal(err)
			}
			originalTarget := readAppTestFile(t, document.Targets[0].File)
			originalConfig := readAppTestFile(t, path)
			mappings := workflow.ProfileImportMappings(document)
			mappings[0].SourceFile = source
			prepared, err := workflow.PrepareProfileImport(document, "Local", mappings)
			if err != nil {
				t.Fatal(err)
			}
			draft, err := workflow.ProfileDraftFromImport(document, prepared, "Imported", []string{"service"})
			if err != nil {
				t.Fatal(err)
			}
			updated, err := workflow.AddProfileDraft(document, draft)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(document.Targets, updated.Targets) {
				t.Fatal("targets changed")
			}
			changedSource := `{"service":{"baseUrl":"later-secret"}}`
			if err := os.WriteFile(source, []byte(changedSource), 0600); err != nil {
				t.Fatal(err)
			}
			expectedConfig := originalConfig
			if failure == "stale config" {
				expectedConfig = append(append([]byte(nil), originalConfig...), []byte("\n# external edit\n")...)
				if err := os.WriteFile(path, expectedConfig, 0600); err != nil {
					t.Fatal(err)
				}
			}
			save, err := workflow.PrepareSave(updated)
			if err == nil {
				err = save.Commit()
			}
			if failure == "none" {
				if err != nil {
					t.Fatal(err)
				}
				saved, err := workflow.LoadDocument(path)
				if err != nil {
					t.Fatal(err)
				}
				savedDraft, err := workflow.ProfileDraft(saved, "Imported")
				if err != nil || savedDraft.Values[0].LiteralValue != "imported-secret" {
					t.Fatalf("saved snapshot = %#v, %v", savedDraft, err)
				}
				if !strings.Contains(string(readAppTestFile(t, filepath.Join(root, ".gitignore"))), ".switchlet.yaml") {
					t.Fatal("missing ignore safeguard")
				}
			} else {
				if err == nil {
					t.Fatal("expected save failure")
				}
				if string(readAppTestFile(t, path)) != string(expectedConfig) {
					t.Fatal("failed save changed config")
				}
			}
			if string(readAppTestFile(t, source)) != changedSource || string(readAppTestFile(t, document.Targets[0].File)) != string(originalTarget) {
				t.Fatal("import/save wrote source or destination")
			}
		})
	}
}
