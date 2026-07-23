package editor

import (
	"fmt"
	"os"
)

// UpdateConnectionString replaces one configured connection string and writes the updated JSON safely.
func UpdateConnectionString(targetPath string, connectionName string, replacementValue string) error {
	if targetPath == "" {
		return fmt.Errorf("target path must be set")
	}
	if connectionName == "" {
		return fmt.Errorf("connection name must be set")
	}

	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("stat target file %q: %w", targetPath, err)
	}
	if targetInfo.IsDir() {
		return fmt.Errorf("target file %q is a directory", targetPath)
	}

	contents, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("read target file %q: %w", targetPath, err)
	}

	updatedContents, err := replaceConnectionString(contents, connectionName, replacementValue)
	if err != nil {
		return fmt.Errorf("update target file %q: %w", targetPath, err)
	}

	if err := writeFileAtomically(targetPath, updatedContents, targetInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("write target file %q: %w", targetPath, err)
	}

	return nil
}
