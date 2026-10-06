//go:build windows

package sshsrv

import (
	"fmt"
	"io"
	"os"
	"syscall"
)

func duplicateInputReader(input io.Reader) (*os.File, error) {
	file, ok := input.(*os.File)
	if !ok {
		return nil, fmt.Errorf("SSH connector input is not an OS file")
	}

	process, err := syscall.GetCurrentProcess()
	if err != nil {
		return nil, fmt.Errorf("get process handle: %w", err)
	}
	var handle syscall.Handle
	if err := syscall.DuplicateHandle(process, syscall.Handle(file.Fd()), process, &handle, 0, false, syscall.DUPLICATE_SAME_ACCESS); err != nil {
		return nil, fmt.Errorf("duplicate SSH connector input: %w", err)
	}
	return os.NewFile(uintptr(handle), file.Name()), nil
}
