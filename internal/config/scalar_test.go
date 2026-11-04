package config_test

import (
	"github.com/jeppeklh/switchlet/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseScalar_ValidatesAndPreservesExactNumericValues(t *testing.T) {
	tests := []struct {
		kind        config.ValueType
		input, want string
	}{
		{config.ValueTypeBoolean, "false", "false"}, {config.ValueTypeInteger, "0", "0"},
		{config.ValueTypeInteger, "-9223372036854775808", "-9223372036854775808"},
		{config.ValueTypeInteger, "9223372036854775807", "9223372036854775807"},
		{config.ValueTypeNumber, "0.12345678901234567890123456789", "0.12345678901234567890123456789"},
		{config.ValueTypeNumber, "1.250e2", "125.0"}, {config.ValueTypeNumber, "-0.000", "0.0"},
		{config.ValueTypeNumber, "1e-308", "0." + strings.Repeat("0", 307) + "1"},
		{config.ValueTypeString, "false", "false"},
	}
	for _, test := range tests {
		t.Run(string(test.kind)+test.input, func(t *testing.T) {
			value, err := config.ParseScalar(test.kind, test.input)
			if err != nil || value.Text() != test.want || value.Type() != test.kind {
				t.Fatalf("type=%s text=%q error=%v", value.Type(), value.Text(), err)
			}
		})
	}
	for _, test := range []struct {
		kind  config.ValueType
		input string
	}{
		{config.ValueTypeBoolean, "1"}, {config.ValueTypeBoolean, "False"}, {config.ValueTypeInteger, "1.0"},
		{config.ValueTypeInteger, "9223372036854775808"}, {config.ValueTypeInteger, "-9223372036854775809"},
		{config.ValueTypeNumber, "NaN"}, {config.ValueTypeNumber, "Infinity"}, {config.ValueTypeNumber, "1e309"}, {config.ValueTypeNumber, "1e-400"}, {config.ValueTypeNumber, "01"},
	} {
		if _, err := config.ParseScalar(test.kind, test.input); err == nil {
			t.Fatalf("accepted invalid %s", test.kind)
		}
	}
	if _, err := config.ParseScalar(config.ValueTypeInteger, "secret-value"); err == nil || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestLoad_VersionFourUsesStrictLiteralTypes(t *testing.T) {
	for _, test := range []struct {
		kind, literal string
		valid         bool
	}{
		{"boolean", "false", true}, {"boolean", `"false"`, false}, {"integer", "0", true}, {"integer", `"8080"`, false},
		{"integer", "1.5", false}, {"number", "0.125", true}, {"number", "30", true}, {"number", `"0.125"`, false},
		{"string", `"8080"`, true}, {"string", "8080", false}, {"boolean", "null", false}, {"number", ".nan", false},
	} {
		t.Run(test.kind+test.literal, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".switchlet.yaml")
			content := "version: 4\ntargets:\n  - name: setting\n    file: config.json\n    jsonPath: value\n    valueType: " + test.kind + "\nprofiles:\n  - name: Local\n    values:\n      - target: setting\n        value: " + test.literal + "\n"
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			loaded, err := config.Load(path)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
			if err == nil && (loaded.Version != 4 || loaded.Targets[0].ValueType != config.ValueType(test.kind) || loaded.Profiles[0].Values[0].ValueType != config.ValueType(test.kind)) {
				t.Fatalf("type metadata lost: %#v", loaded)
			}
		})
	}
}

func TestCreate_TypedValuesRoundTripAsNativeScalars(t *testing.T) {
	root := t.TempDir()
	literal := "false"
	path, loaded, err := config.Create(root, []config.Target{{Name: "flag", File: "settings.json", Type: config.TargetTypeJSON, JSONPath: "flag", ValueType: config.ValueTypeBoolean}}, []config.Profile{{Name: "Local", Values: []config.ProfileValue{{Target: "flag", Value: &literal}}}})
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != 4 || !strings.Contains(string(contents), "value: false") || strings.Contains(string(contents), `value: "false"`) {
		t.Fatalf("incorrect schema output: %s", contents)
	}
	snapshot, err := config.LoadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := config.PrepareReplacementFromSnapshot(snapshot, loaded.Targets, loaded.Profiles)
	if err != nil {
		t.Fatal(err)
	}
	if err := replacement.Commit(); err != nil {
		t.Fatal(err)
	}
	again, err := config.Load(path)
	if err != nil || again.Version != 4 || *again.Profiles[0].Values[0].Value != "false" {
		t.Fatalf("roundtrip failed: %#v %v", again, err)
	}
}

func TestLoad_OlderVersionsRejectValueType(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".switchlet.yaml")
	text := "version: 3\ntargets:\n  - name: flag\n    file: settings.json\n    jsonPath: flag\n    valueType: boolean\nprofiles:\n  - name: Local\n    values:\n      - target: flag\n        value: false\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(path); err == nil || !strings.Contains(err.Error(), "requires version 4") {
		t.Fatalf("expected version error: %v", err)
	}
}

func FuzzParseNumberRoundTrip(f *testing.F) {
	for _, seed := range []string{"0", "-0", "0.1", "1e-308", "5e-324", "1e308", "0.12345678901234567890123456789", "1e999999999999", "0e10000000", "-0e-10000000", "1." + strings.Repeat("1", 400) + "e-308"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		value, err := config.ParseScalar(config.ValueTypeNumber, text)
		if err != nil {
			return
		}
		again, err := config.ParseScalar(config.ValueTypeNumber, value.Text())
		if err != nil || again != value {
			t.Fatalf("numeric normalization does not round trip: %q -> %q: %v", text, value.Text(), err)
		}
	})
}
