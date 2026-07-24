package editor

import (
	"fmt"
	"io/fs"
	"os"
	"sort"
)

func connectionStringJSONPath(connectionName string) string {
	return "ConnectionStrings." + connectionName
}

// ValidateStringTarget verifies that the configured target file exists and
// contains the configured string value at the expected JSON path.
func ValidateStringTarget(targetPath string, jsonPath string) error {
	if targetPath == "" {
		return fmt.Errorf("target path must be set")
	}
	if jsonPath == "" {
		return fmt.Errorf("JSON path must be set")
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

	if _, _, _, err := parseStringTarget(contents, jsonPath); err != nil {
		return fmt.Errorf("validate target file %q: %w", targetPath, err)
	}

	return nil
}

// ValidateConnectionStringTarget verifies that the configured target file exists
// and contains the configured connection string in the expected JSON structure.
func ValidateConnectionStringTarget(targetPath string, connectionName string) error {
	if connectionName == "" {
		return fmt.Errorf("connection name must be set")
	}

	return ValidateStringTarget(targetPath, connectionStringJSONPath(connectionName))
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
	if connectionName == "" {
		return fmt.Errorf("connection name must be set")
	}

	return UpdateStringValue(targetPath, connectionStringJSONPath(connectionName), replacementValue)
}

// UpdateStringValue replaces one configured string value and writes the updated JSON safely.
func UpdateStringValue(targetPath string, jsonPath string, replacementValue string) error {
	updatedContents, permissions, err := prepareStringValueUpdate(targetPath, jsonPath, replacementValue)
	if err != nil {
		return err
	}

	if err := writeFileAtomically(targetPath, updatedContents, permissions); err != nil {
		return fmt.Errorf("write target file %q: %w", targetPath, err)
	}

	return nil
}

// PreviewStringValueUpdate validates that a configured string value can be
// updated without writing the target file.
func PreviewStringValueUpdate(targetPath string, jsonPath string, replacementValue string) error {
	_, _, err := prepareStringValueUpdate(targetPath, jsonPath, replacementValue)
	return err
}

func prepareStringValueUpdate(targetPath string, jsonPath string, replacementValue string) ([]byte, fs.FileMode, error) {
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return nil, 0, fmt.Errorf("stat target file %q: %w", targetPath, err)
	}
	if targetInfo.IsDir() {
		return nil, 0, fmt.Errorf("target file %q is a directory", targetPath)
	}

	contents, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, 0, fmt.Errorf("read target file %q: %w", targetPath, err)
	}

	updatedContents, err := replaceStringValue(contents, jsonPath, replacementValue)
	if err != nil {
		return nil, 0, fmt.Errorf("update target file %q: %w", targetPath, err)
	}

	return updatedContents, targetInfo.Mode().Perm(), nil
}
