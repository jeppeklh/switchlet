package editor

import (
	"fmt"
	"io/fs"
	"os"
)

func statRegularTargetFile(targetPath string) (fs.FileInfo, error) {
	if targetPath == "" {
		return nil, fmt.Errorf("target path must be set")
	}

	targetInfo, err := os.Lstat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("stat target file %q: %w", targetPath, err)
	}
	if err := validateRegularTargetFileInfo(targetPath, targetInfo); err != nil {
		return nil, err
	}

	return targetInfo, nil
}

func validateRegularTargetFileInfo(targetPath string, targetInfo fs.FileInfo) error {
	mode := targetInfo.Mode()

	switch {
	case mode&fs.ModeSymlink != 0:
		return fmt.Errorf("target file %q is a symlink; Switchlet only supports regular files", targetPath)
	case targetInfo.IsDir():
		return fmt.Errorf("target file %q is a directory", targetPath)
	case mode.IsRegular():
		return nil
	case mode&fs.ModeNamedPipe != 0:
		return fmt.Errorf("target file %q is a named pipe; Switchlet only supports regular files", targetPath)
	case mode&fs.ModeSocket != 0:
		return fmt.Errorf("target file %q is a socket; Switchlet only supports regular files", targetPath)
	case mode&fs.ModeDevice != 0 && mode&fs.ModeCharDevice != 0:
		return fmt.Errorf("target file %q is a character device; Switchlet only supports regular files", targetPath)
	case mode&fs.ModeDevice != 0:
		return fmt.Errorf("target file %q is a device; Switchlet only supports regular files", targetPath)
	default:
		return fmt.Errorf("target file %q is not a regular file; Switchlet only supports regular files", targetPath)
	}
}
