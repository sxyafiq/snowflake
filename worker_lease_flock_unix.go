//go:build unix

package snowflake

import (
	"errors"
	"os"
	"syscall"
)

// lockFileExclusive takes a non-blocking exclusive advisory lock on f.
//
// It returns (true, nil) on success, (false, nil) if another holder owns the
// lock, and (false, err) on any other failure. The lock is associated with the
// open file description and is released by unlockFile or when the descriptor
// (or the process) closes — so a crash frees it automatically, with no clock
// dependence.
func lockFileExclusive(f *os.File) (bool, error) {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, syscall.EWOULDBLOCK), errors.Is(err, syscall.EAGAIN):
		return false, nil
	default:
		return false, err
	}
}

// unlockFile releases an advisory lock held on f.
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
