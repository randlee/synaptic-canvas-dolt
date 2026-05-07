//go:build !windows

package installer

import (
	"os"

	"golang.org/x/sys/unix"
)

func lockFileExclusive(file *os.File) error {
	return setFileLock(file, unix.F_WRLCK)
}

func unlockFile(file *os.File) error {
	return setFileLock(file, unix.F_UNLCK)
}

func setFileLock(file *os.File, lockType int16) error {
	return unix.FcntlFlock(file.Fd(), unix.F_SETLKW, &unix.Flock_t{
		Type:   lockType,
		Whence: 0,
		Start:  0,
		Len:    0,
	})
}
