//go:build windows

package editor

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	moveFileReplaceExisting = 0x1
	moveFileWriteThrough    = 0x8
)

var moveFileExW = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func replaceExistingFile(sourcePath string, targetPath string) error {
	sourcePathPointer, err := syscall.UTF16PtrFromString(sourcePath)
	if err != nil {
		return fmt.Errorf("encode source path %q for replacement: %w", sourcePath, err)
	}

	targetPathPointer, err := syscall.UTF16PtrFromString(targetPath)
	if err != nil {
		return fmt.Errorf("encode target path %q for replacement: %w", targetPath, err)
	}

	result, _, callErr := moveFileExW.Call(
		uintptr(unsafe.Pointer(sourcePathPointer)),
		uintptr(unsafe.Pointer(targetPathPointer)),
		uintptr(moveFileReplaceExisting|moveFileWriteThrough),
	)
	if result != 0 {
		return nil
	}
	if callErr != syscall.Errno(0) {
		return callErr
	}

	return syscall.EINVAL
}

func syncContainingDirectory(string) error {
	return nil
}
