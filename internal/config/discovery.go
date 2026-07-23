package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const configFileName = ".switchlet.yaml"

// ErrConfigNotFound indicates that configuration discovery reached the filesystem root without finding .switchlet.yaml.
var ErrConfigNotFound = errors.New("configuration file not found")

// Discover searches upward from startDir until it finds .switchlet.yaml or reaches the filesystem root.
func Discover(startDir string) (string, error) {
	if strings.TrimSpace(startDir) == "" {
		return "", fmt.Errorf("start directory must be set")
	}

	resolvedStartDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolve start directory %q: %w", startDir, err)
	}

	startInfo, err := os.Stat(resolvedStartDir)
	if err != nil {
		return "", fmt.Errorf("stat start directory %q: %w", resolvedStartDir, err)
	}
	if !startInfo.IsDir() {
		return "", fmt.Errorf("start directory %q is not a directory", resolvedStartDir)
	}

	currentDir := resolvedStartDir
	for {
		candidatePath := filepath.Join(currentDir, configFileName)

		candidateInfo, err := os.Stat(candidatePath)
		switch {
		case err == nil && candidateInfo.IsDir():
			return "", fmt.Errorf("configuration path %q is a directory", candidatePath)
		case err == nil:
			return candidatePath, nil
		case errors.Is(err, fs.ErrNotExist):
		default:
			return "", fmt.Errorf("stat configuration path %q: %w", candidatePath, err)
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return "", fmt.Errorf("discover %s from %q: %w", configFileName, resolvedStartDir, ErrConfigNotFound)
		}

		currentDir = parentDir
	}
}
