//go:build !windows

package editor

import (
	"errors"
	"os"
	"syscall"
)

func replaceExistingFile(sourcePath string, targetPath string) error {
	return os.Rename(sourcePath, targetPath)
}

func syncContainingDirectory(directoryPath string) error {
	directory, err := os.Open(directoryPath)
	if err != nil {
		if isUnsupportedDirectorySyncError(err) {
			return nil
		}

		return err
	}
	defer func() { _ = directory.Close() }()

	if err := directory.Sync(); err != nil {
		if isUnsupportedDirectorySyncError(err) {
			return nil
		}

		return err
	}

	return nil
}

func isUnsupportedDirectorySyncError(err error) bool {
	return errors.Is(err, syscall.EINVAL) ||
		errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, syscall.EOPNOTSUPP)
}
