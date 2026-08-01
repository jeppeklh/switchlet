//go:build !windows

package config

import (
	"errors"
	"os"
	"syscall"
)

func replaceExistingConfigFile(sourcePath string, targetPath string) error {
	return os.Rename(sourcePath, targetPath)
}

func syncConfigContainingDirectory(directoryPath string) error {
	directory, err := os.Open(directoryPath)
	if err != nil {
		if isUnsupportedConfigDirectorySyncError(err) {
			return nil
		}

		return err
	}
	defer func() { _ = directory.Close() }()

	if err := directory.Sync(); err != nil {
		if isUnsupportedConfigDirectorySyncError(err) {
			return nil
		}

		return err
	}

	return nil
}

func isUnsupportedConfigDirectorySyncError(err error) bool {
	return errors.Is(err, syscall.EINVAL) ||
		errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, syscall.EOPNOTSUPP)
}
