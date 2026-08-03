package profile_test

import (
	"testing"

	"github.com/jeppeklh/switchlet/internal/profile"
)

func TestMaskConnectionString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Password key",
			input: "Password=secret",
			want:  "Password=****",
		},
		{
			name:  "password key",
			input: "password=secret",
			want:  "password=****",
		},
		{
			name:  "Pwd key",
			input: "Pwd=secret",
			want:  "Pwd=****",
		},
		{
			name:  "PWD key",
			input: "PWD=secret",
			want:  "PWD=****",
		},
		{
			name:  "values without passwords",
			input: "Server=localhost;Database=App;Trusted_Connection=True;",
			want:  "Server=localhost;Database=App;Trusted_Connection=True;",
		},
		{
			name:  "empty segments",
			input: "Server=localhost;;Password=secret",
			want:  "Server=localhost;;Password=****",
		},
		{
			name:  "values containing equals sign",
			input: "Password=secret=value;User Id=test",
			want:  "Password=****;User Id=test",
		},
		{
			name:  "trailing semicolons",
			input: "Server=localhost;Password=secret;;",
			want:  "Server=localhost;Password=****;;",
		},
		{
			name:  "token key",
			input: "AuthToken=secret;User Id=app",
			want:  "AuthToken=****;User Id=app",
		},
		{
			name:  "access key",
			input: "AccessKey=secret;Endpoint=https://example.test",
			want:  "AccessKey=****;Endpoint=https://example.test",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			originalInput := testCase.input

			maskedValue := profile.MaskConnectionString(testCase.input)

			if maskedValue != testCase.want {
				t.Fatalf("MaskConnectionString(%q) = %q, want %q", testCase.input, maskedValue, testCase.want)
			}
			if testCase.input != originalInput {
				t.Fatalf("input changed from %q to %q", originalInput, testCase.input)
			}
		})
	}
}

func TestMaskManagedValue(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		context profile.ManagedValueMaskContext
		want    string
	}{
		{
			name:  "ordinary value masks full value",
			value: "https://api.staging.example.test",
			context: profile.ManagedValueMaskContext{
				TargetName: "frontendApi",
				Selector:   "VITE_API_URL",
			},
			want: "****",
		},
		{
			name:  "target name masks full value",
			value: "https://example.test/plain-value",
			context: profile.ManagedValueMaskContext{
				TargetName: "apiKey",
				Selector:   "services.api.url",
			},
			want: "****",
		},
		{
			name:  "empty value remains empty",
			value: "",
			context: profile.ManagedValueMaskContext{
				TargetName: "apiKey",
			},
			want: "",
		},
		{
			name:  "selector masks full value",
			value: "postgres://plain-value",
			context: profile.ManagedValueMaskContext{
				TargetName: "database",
				Selector:   "database.password",
			},
			want: "****",
		},
		{
			name:  "environment variable masks full value",
			value: "https://example.test/plain-value",
			context: profile.ManagedValueMaskContext{
				TargetName:              "serviceEndpoint",
				Selector:                "services.api.url",
				EnvironmentVariableName: "STAGING_SERVICE_API_KEY",
			},
			want: "****",
		},
		{
			name:  "non-sensitive context still masks full value",
			value: "banana",
			context: profile.ManagedValueMaskContext{
				TargetName: "monkeyMode",
				Selector:   "settings.monkeyMode",
			},
			want: "****",
		},
		{
			name:  "connection string masks full value",
			value: "Server=db;Database=App;Pwd=secret;",
			context: profile.ManagedValueMaskContext{
				TargetName: "database",
				Selector:   "ConnectionStrings.DefaultConnection",
			},
			want: "****",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			maskedValue := profile.MaskManagedValue(testCase.value, testCase.context)

			if maskedValue != testCase.want {
				t.Fatalf("MaskManagedValue(%q, %#v) = %q, want %q", testCase.value, testCase.context, maskedValue, testCase.want)
			}
		})
	}
}
