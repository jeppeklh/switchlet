package editor

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
)

func TestValidateTarget_AcceptsExistingDotenvKey(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "frontend/.env.local", strings.TrimSpace(`
# local frontend settings
VITE_API_URL=http://localhost:5173
VITE_FEATURES=local
`)+"\n")

	err := ValidateTarget(config.Target{
		Name: "frontendApi",
		File: targetPath,
		Type: config.TargetTypeDotenv,
		Key:  "VITE_API_URL",
	})
	if err != nil {
		t.Fatalf("ValidateTarget returned error: %v", err)
	}
}

func TestReadTargetValue_ReturnsDotenvCurrentValueWithoutWriting(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "frontend/.env.local", strings.TrimSpace(`
# local frontend settings
VITE_API_URL=http://localhost:5173
VITE_FEATURES=local
`)+"\n")
	originalContents := readFile(t, targetPath)

	value, err := ReadTargetValue(config.Target{
		Name: "frontendApi",
		File: targetPath,
		Type: config.TargetTypeDotenv,
		Key:  "VITE_API_URL",
	})
	if err != nil {
		t.Fatalf("ReadTargetValue returned error: %v", err)
	}
	if value != "http://localhost:5173" {
		t.Fatalf("value = %q, want current dotenv value", value)
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("dotenv target changed during current-value read")
	}
	if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
		t.Fatal("current-value read created a temporary file")
	}
}

func TestReadTargetValue_ReturnsQuotedDotenvCurrentValueWithoutQuotes(t *testing.T) {
	testCases := []struct {
		name           string
		targetContents string
		wantValue      string
	}{
		{
			name:           "single quoted",
			targetContents: "VITE_API_URL='https://api.example.test'\n",
			wantValue:      "https://api.example.test",
		},
		{
			name:           "double quoted",
			targetContents: "VITE_API_URL=\"https://api.example.test/path?mode=qa\" # keep\n",
			wantValue:      "https://api.example.test/path?mode=qa",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			targetPath := writeTargetFile(t, projectRoot, "frontend/.env.local", testCase.targetContents)
			originalContents := readFile(t, targetPath)

			value, err := ReadTargetValue(config.Target{
				Name: "frontendApi",
				File: targetPath,
				Type: config.TargetTypeDotenv,
				Key:  "VITE_API_URL",
			})
			if err != nil {
				t.Fatalf("ReadTargetValue returned error: %v", err)
			}
			if value != testCase.wantValue {
				t.Fatalf("value = %q, want %q", value, testCase.wantValue)
			}
			if !bytes.Equal(readFile(t, targetPath), originalContents) {
				t.Fatal("dotenv target changed during quoted current-value read")
			}
		})
	}
}

func TestApplyTargetChanges_ReplacesDotenvKeyAndPreservesUnrelatedLines(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "frontend/.env.local", strings.TrimSpace(`
# local frontend settings
VITE_API_URL=http://localhost:5173
VITE_FEATURES=local

# end
`)+"\n")

	err := ApplyTargetChanges([]TargetChange{{
		Target: config.Target{Name: "frontendApi", File: targetPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		Value:  "https://api.example.test",
	}})
	if err != nil {
		t.Fatalf("ApplyTargetChanges returned error: %v", err)
	}

	wantContents := strings.TrimSpace(`
# local frontend settings
VITE_API_URL=https://api.example.test
VITE_FEATURES=local

# end
`) + "\n"
	if string(readFile(t, targetPath)) != wantContents {
		t.Fatalf("dotenv contents = %q, want %q", readFile(t, targetPath), wantContents)
	}
}

func TestApplyTargetChanges_UpdatesMultipleDotenvKeysInOneFile(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "frontend/.env.local", strings.TrimSpace(`
VITE_API_URL=http://localhost:5173
VITE_FEATURES=local
`)+"\n")
	var writtenTargets []string
	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		writtenTargets = append(writtenTargets, newPath)
		return originalReplaceFile(oldPath, newPath)
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
	})

	err := ApplyTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "frontendApi", File: targetPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
			Value:  "https://api.example.test",
		},
		{
			Target: config.Target{Name: "features", File: targetPath, Type: config.TargetTypeDotenv, Key: "VITE_FEATURES"},
			Value:  "staging",
		},
	})
	if err != nil {
		t.Fatalf("ApplyTargetChanges returned error: %v", err)
	}
	if len(writtenTargets) != 1 || writtenTargets[0] != targetPath {
		t.Fatalf("written targets = %#v, want one write to %q", writtenTargets, targetPath)
	}

	wantContents := strings.TrimSpace(`
VITE_API_URL=https://api.example.test
VITE_FEATURES=staging
`) + "\n"
	if string(readFile(t, targetPath)) != wantContents {
		t.Fatalf("dotenv contents = %q, want %q", readFile(t, targetPath), wantContents)
	}
}

func TestApplyTargetChanges_PreservesSingleQuotedDotenvStyleCommentsAndLineEndings(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "# local frontend settings\r\nVITE_API_URL='http://localhost:5173' # keep API quoted\r\nVITE_FEATURES=local\r\n")

	err := ApplyTargetChanges([]TargetChange{{
		Target: config.Target{Name: "frontendApi", File: targetPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		Value:  "https://api.example.test",
	}})
	if err != nil {
		t.Fatalf("ApplyTargetChanges returned error: %v", err)
	}

	wantContents := "# local frontend settings\r\nVITE_API_URL='https://api.example.test' # keep API quoted\r\nVITE_FEATURES=local\r\n"
	if string(readFile(t, targetPath)) != wantContents {
		t.Fatalf("dotenv contents = %q, want %q", readFile(t, targetPath), wantContents)
	}
}

func TestApplyTargetChanges_PreservesDoubleQuotedDotenvStyleAndEscapesReplacement(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=\"http://localhost:5173\" # keep API quoted\n")
	replacement := `say "hello" \\ again`

	err := ApplyTargetChanges([]TargetChange{{
		Target: config.Target{Name: "frontendApi", File: targetPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		Value:  replacement,
	}})
	if err != nil {
		t.Fatalf("ApplyTargetChanges returned error: %v", err)
	}

	wantContents := "VITE_API_URL=" + strconv.Quote(replacement) + " # keep API quoted\n"
	if string(readFile(t, targetPath)) != wantContents {
		t.Fatalf("dotenv contents = %q, want %q", readFile(t, targetPath), wantContents)
	}
}

func TestApplyTargetChanges_PreservesDotenvFilePermissions(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")

	if err := os.Chmod(targetPath, 0o600); err != nil {
		t.Fatalf("set file permissions: %v", err)
	}
	originalInfo, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat original file: %v", err)
	}

	err = ApplyTargetChanges([]TargetChange{{
		Target: config.Target{Name: "frontendApi", File: targetPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		Value:  "https://api.example.test",
	}})
	if err != nil {
		t.Fatalf("ApplyTargetChanges returned error: %v", err)
	}

	updatedInfo, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat updated file: %v", err)
	}
	if updatedInfo.Mode().Perm() != originalInfo.Mode().Perm() {
		t.Fatalf("file permissions = %o, want %o", updatedInfo.Mode().Perm(), originalInfo.Mode().Perm())
	}
}

func TestApplyTargetChanges_DotenvValidationFailuresLeaveFileUnchangedAndHideSecrets(t *testing.T) {
	tests := []struct {
		name           string
		targetContents string
		key            string
		wantError      string
	}{
		{
			name:           "missing key",
			targetContents: "VITE_FEATURES=local\n",
			key:            "VITE_API_URL",
			wantError:      "dotenv key does not exist",
		},
		{
			name:           "duplicate key",
			targetContents: "VITE_API_URL=http://one.example.test\nVITE_API_URL=http://two.example.test\n",
			key:            "VITE_API_URL",
			wantError:      "dotenv key appears more than once",
		},
		{
			name:           "unsupported line",
			targetContents: "VITE_API_URL=http://localhost:5173\nnot an assignment\n",
			key:            "VITE_API_URL",
			wantError:      "not a supported KEY=value assignment",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			targetPath := writeTargetFile(t, projectRoot, "frontend/.env.local", testCase.targetContents)
			originalContents := readFile(t, targetPath)

			err := ApplyTargetChanges([]TargetChange{{
				Target: config.Target{Name: "frontendApi", File: targetPath, Type: config.TargetTypeDotenv, Key: testCase.key},
				Value:  "https://secret-value.example.test",
			}})
			if err == nil {
				t.Fatal("ApplyTargetChanges returned nil error, want dotenv validation failure")
			}
			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("ApplyTargetChanges returned error %q, want substring %q", err, testCase.wantError)
			}
			if !strings.Contains(err.Error(), `target "frontendApi"`) || !strings.Contains(err.Error(), `key "`+testCase.key+`"`) {
				t.Fatalf("ApplyTargetChanges returned error %q, want target and key context", err)
			}
			if strings.Contains(err.Error(), "secret-value") {
				t.Fatalf("ApplyTargetChanges leaked secret in error %q", err)
			}

			if !bytes.Equal(readFile(t, targetPath), originalContents) {
				t.Fatal("dotenv file changed after validation failure")
			}
		})
	}
}

func TestApplyTargetChanges_RejectsDotenvReplacementWithNewlineBeforeWriting(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")
	originalContents := readFile(t, targetPath)

	err := ApplyTargetChanges([]TargetChange{{
		Target: config.Target{Name: "frontendApi", File: targetPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		Value:  "https://secret.example.test\nNEXT=value",
	}})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want newline validation failure")
	}
	if !strings.Contains(err.Error(), "replacement value must not contain newline characters") {
		t.Fatalf("ApplyTargetChanges returned error %q, want newline validation error", err)
	}
	if strings.Contains(err.Error(), "secret.example.test") {
		t.Fatalf("ApplyTargetChanges leaked secret in error %q", err)
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("dotenv file changed after newline validation failure")
	}
}

func TestApplyTargetChanges_RejectsUnsupportedQuotedDotenvCasesWithoutWriting(t *testing.T) {
	testCases := []struct {
		name           string
		targetContents string
		replacement    string
		wantError      string
	}{
		{
			name:           "missing single quote terminator",
			targetContents: "VITE_API_URL='http://localhost:5173\n",
			replacement:    "https://api.example.test",
			wantError:      "single-quoted value without a closing quote",
		},
		{
			name:           "unsupported trailing content after double quote",
			targetContents: "VITE_API_URL=\"http://localhost:5173\" trailing\n",
			replacement:    "https://api.example.test",
			wantError:      "unsupported trailing content after the quoted value",
		},
		{
			name:           "single-quoted replacement cannot preserve quote style",
			targetContents: "VITE_API_URL='http://localhost:5173'\n",
			replacement:    "https://api.example.test/it's-quoted",
			wantError:      "cannot preserve the existing single-quoted dotenv style safely",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			targetPath := writeTargetFile(t, projectRoot, "frontend/.env.local", testCase.targetContents)
			originalContents := readFile(t, targetPath)

			err := ApplyTargetChanges([]TargetChange{{
				Target: config.Target{Name: "frontendApi", File: targetPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
				Value:  testCase.replacement,
			}})
			if err == nil {
				t.Fatal("ApplyTargetChanges returned nil error, want quoted dotenv failure")
			}
			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("ApplyTargetChanges returned error %q, want substring %q", err, testCase.wantError)
			}
			if strings.Contains(err.Error(), "it's-quoted") {
				t.Fatalf("ApplyTargetChanges leaked secret in error %q", err)
			}
			if !bytes.Equal(readFile(t, targetPath), originalContents) {
				t.Fatal("dotenv file changed after quoted dotenv failure")
			}
		})
	}
}
