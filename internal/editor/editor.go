package editor

import (
	"fmt"
	"os"
)

// ValidateConnectionStringTarget verifies that the configured target file exists
// and contains the configured connection string in the expected JSON structure.
func ValidateConnectionStringTarget(targetPath string, connectionName string) error {
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

	if _, _, err := parseConnectionStringTarget(contents, connectionName); err != nil {
		return fmt.Errorf("validate target file %q: %w", targetPath, err)
	}

	return nil
}

// UpdateConnectionString replaces one configured connection string and writes the updated JSON safely.
func UpdateConnectionString(targetPath string, connectionName string, replacementValue string) error {
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
