//go:build !windows

package config

import "os"

func replaceExistingConfigFile(sourcePath string, targetPath string) error {
	return os.Rename(sourcePath, targetPath)
}
