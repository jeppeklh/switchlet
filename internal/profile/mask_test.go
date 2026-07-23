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
