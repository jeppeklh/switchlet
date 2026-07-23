//go:build !windows

package editor

import "os"

func replaceExistingFile(sourcePath string, targetPath string) error {
	return os.Rename(sourcePath, targetPath)
}
