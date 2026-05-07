//go:build !windows

package installer

import (
	"os"

	"golang.org/x/sys/unix"
)

func lockFileExclusive(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_EX)
}

func unlockFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
