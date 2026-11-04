//go:build !windows

package history

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func checkPrivate(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return fmt.Errorf("undo storage must use regular files and directories")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("undo storage %q must be owned by the current user and accessible only to that user", path)
	}
	return nil
}
func secureDirectory(path string) error { return checkPrivate(path) }
func lockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
func unlockFile(file *os.File) error           { return syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }
func replaceState(source, target string) error { return os.Rename(source, target) }
func syncStateDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	err = directory.Sync()
	if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP) {
		return nil
	}
	return err
}

func secureNewFile(path string) error { return checkPrivate(path) }
