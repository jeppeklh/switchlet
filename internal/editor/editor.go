package editor

import (
	"fmt"
	"os"
	"sort"
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

// ListConnectionStringNames returns the existing string-valued connection names
// from the target file's ConnectionStrings object.
func ListConnectionStringNames(targetPath string) ([]string, error) {
	if targetPath == "" {
		return nil, fmt.Errorf("target path must be set")
	}

	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("stat target file %q: %w", targetPath, err)
	}
	if targetInfo.IsDir() {
		return nil, fmt.Errorf("target file %q is a directory", targetPath)
	}

	contents, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read target file %q: %w", targetPath, err)
	}

	_, connectionStringsObject, err := parseConnectionStringsObject(contents)
	if err != nil {
		return nil, fmt.Errorf("inspect target file %q: %w", targetPath, err)
	}

	connectionNames := make([]string, 0, len(connectionStringsObject))
	for connectionName, connectionValue := range connectionStringsObject {
		if _, ok := connectionValue.(string); ok {
			connectionNames = append(connectionNames, connectionName)
		}
	}

	if len(connectionNames) == 0 {
		return nil, fmt.Errorf("inspect target file %q: ConnectionStrings does not contain any string-valued connection strings", targetPath)
	}

	sort.Strings(connectionNames)

	return connectionNames, nil
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
