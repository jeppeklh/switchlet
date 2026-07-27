//go:build windows

package config

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	moveConfigFileReplaceExisting = 0x1
	moveConfigFileWriteThrough    = 0x8
)

var moveConfigFileExW = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func replaceExistingConfigFile(sourcePath string, targetPath string) error {
	sourcePathPointer, err := syscall.UTF16PtrFromString(sourcePath)
	if err != nil {
		return fmt.Errorf("encode source path %q for replacement: %w", sourcePath, err)
	}

	targetPathPointer, err := syscall.UTF16PtrFromString(targetPath)
	if err != nil {
		return fmt.Errorf("encode target path %q for replacement: %w", targetPath, err)
	}

	result, _, callErr := moveConfigFileExW.Call(
		uintptr(unsafe.Pointer(sourcePathPointer)),
		uintptr(unsafe.Pointer(targetPathPointer)),
		uintptr(moveConfigFileReplaceExisting|moveConfigFileWriteThrough),
	)
	if result != 0 {
		return nil
	}
	if callErr != syscall.Errno(0) {
		return callErr
	}

	return syscall.EINVAL
}
