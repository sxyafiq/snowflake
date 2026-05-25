//go:build !unix

package snowflake

import (
	"errors"
	"os"
)

// errFileLeaseUnsupported is returned by FileWorkerLease on platforms without
// flock-style advisory locking. Supply a custom WorkerLease instead.
var errFileLeaseUnsupported = errors.New(
	"FileWorkerLease is unsupported on this platform; supply a custom WorkerLease (e.g. Redis or etcd backed)")

func lockFileExclusive(_ *os.File) (bool, error) {
	return false, errFileLeaseUnsupported
}

func unlockFile(_ *os.File) error {
	return nil
}
