package editor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var replaceFile = replaceExistingFile

func writeFileAtomically(targetPath string, contents []byte, permissions fs.FileMode) (returnErr error) {
	targetDirectory := filepath.Dir(targetPath)
	temporaryFile, err := os.CreateTemp(targetDirectory, tempFilePattern(targetPath))
	if err != nil {
		return fmt.Errorf("create temporary file in %q: %w", targetDirectory, err)
	}

	temporaryFilePath := temporaryFile.Name()
	defer func() {
		if returnErr == nil {
			return
		}

		if temporaryFile != nil {
			_ = temporaryFile.Close()
		}

		if err := os.Remove(temporaryFilePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			returnErr = fmt.Errorf("%w; remove temporary file %q: %v", returnErr, temporaryFilePath, err)
		}
	}()

	if _, err := temporaryFile.Write(contents); err != nil {
		return fmt.Errorf("write temporary file %q: %w", temporaryFilePath, err)
	}

	if err := temporaryFile.Close(); err != nil {
		return fmt.Errorf("close temporary file %q: %w", temporaryFilePath, err)
	}
	temporaryFile = nil

	if err := os.Chmod(temporaryFilePath, permissions); err != nil {
		return fmt.Errorf("apply permissions to temporary file %q: %w", temporaryFilePath, err)
	}

	if err := replaceFile(temporaryFilePath, targetPath); err != nil {
		return fmt.Errorf("replace target file with temporary file %q: %w", temporaryFilePath, err)
	}

	return nil
}

func tempFilePattern(targetPath string) string {
	return tempFilePrefix(targetPath) + "*"
}

func tempFilePrefix(targetPath string) string {
	return "." + filepath.Base(targetPath) + ".switchlet-"
}
