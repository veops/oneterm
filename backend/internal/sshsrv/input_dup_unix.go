//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package sshsrv

import (
	"fmt"
	"io"
	"os"
	"syscall"
)

// Keep the connector relay's read end independent from Bubble Tea's input
// file so closing the relay cannot steal or close the next local command.
func duplicateInputReader(input io.Reader) (*os.File, error) {
	file, ok := input.(*os.File)
	if !ok {
		return nil, fmt.Errorf("SSH connector input is not an OS file")
	}

	fd, err := syscall.Dup(int(file.Fd()))
	if err != nil {
		return nil, fmt.Errorf("duplicate SSH connector input: %w", err)
	}
	return os.NewFile(uintptr(fd), file.Name()), nil
}
