package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func writeTargetFile(t *testing.T, rootDir string, relativePath string, contents string) string {
	t.Helper()

	fullPath := filepath.Join(rootDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create parent directories for %q: %v", fullPath, err)
	}

	if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write target file %q: %v", fullPath, err)
	}

	return fullPath
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %q: %v", path, err)
	}

	return contents
}

func stringPointer(value string) *string {
	return &value
}

func runeKey(value rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}}
}

func lineContains(view string, values ...string) bool {
	for _, line := range strings.Split(view, "\n") {
		matched := true
		for _, value := range values {
			if !strings.Contains(line, value) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}

	return false
}

func visibleLines(view string) []string {
	trimmedView := strings.TrimSuffix(view, "\n")
	if trimmedView == "" {
		return nil
	}

	return strings.Split(trimmedView, "\n")
}

func assertVisibleWidth(t *testing.T, view string, width int) {
	t.Helper()

	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > width {
			t.Fatalf("line %q has width %d, want at most %d", line, lipgloss.Width(line), width)
		}
	}
}

func assertVisibleHeight(t *testing.T, view string, height int) {
	t.Helper()

	lines := visibleLines(view)
	if len(lines) > height {
		t.Fatalf("View() rendered %d lines, want at most %d", len(lines), height)
	}
}
