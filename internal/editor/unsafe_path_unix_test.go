//go:build !windows

package editor

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
)

func TestReadTargetValue_RejectsNamedPipeTargetPathWithoutReading(t *testing.T) {
	projectRoot := t.TempDir()
	pipePath := filepath.Join(projectRoot, "config.pipe")
	if err := syscall.Mkfifo(pipePath, 0o644); err != nil {
		t.Skipf("named pipe creation not supported in this environment: %v", err)
	}

	_, err := ReadTargetValue(config.Target{
		Name:     "database",
		File:     pipePath,
		Type:     config.TargetTypeJSON,
		JSONPath: "database.url",
	})
	if err == nil {
		t.Fatal("ReadTargetValue returned nil error, want named-pipe target-path failure")
	}
	for _, expected := range []string{`target "database"`, `jsonPath "database.url"`, `named pipe`} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("ReadTargetValue returned error %q, want substring %q", err, expected)
		}
	}
	if containsTempFile(t, filepath.Dir(pipePath), tempFilePrefix(pipePath)) {
		t.Fatal("named-pipe target-path read created a temporary file")
	}
}
