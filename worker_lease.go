package snowflake

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ErrWorkerLeaseHeld is returned when a worker identity is already held by
// another live process. Check with errors.Is.
var ErrWorkerLeaseHeld = errors.New("worker ID lease held by another process")

// WorkerLease guards against two live processes sharing a worker ID — a
// misconfiguration or deploy/crash overlap that would mint duplicate IDs
// regardless of clock handling, because uniqueness in Snowflake depends on
// each worker ID having a single live owner.
//
// A WorkerLease takes exclusive ownership of a worker identity at startup. This
// is complementary to ClockGuard: a lease covers two SIMULTANEOUS holders,
// while a ClockGuard covers SEQUENTIAL restarts of one holder under a regressed
// clock. Production deployments that need both should configure both.
//
// Implementations MUST be safe for concurrent use.
type WorkerLease interface {
	// Acquire takes an exclusive, process-lifetime lease on key. It returns
	// ErrWorkerLeaseHeld if another live process holds it. The returned handle
	// must be released (via Generator.Close) to free the lease, though a robust
	// implementation also frees it automatically when the process exits.
	Acquire(ctx context.Context, key string) (LeaseHandle, error)
}

// LeaseHandle represents ownership of a worker identity. Release frees it.
type LeaseHandle interface {
	// Release frees the lease. It must be safe to call more than once.
	Release() error
}

// FileWorkerLease is the default WorkerLease: an OS advisory file lock (flock)
// per worker identity, stored under a directory.
//
// flock is the right primitive for a single-host lease: the lock is bound to
// an open file descriptor and released automatically when that descriptor — or
// the whole process — closes. A crash therefore frees the identity instantly,
// with no TTL to tune and no dependence on the wall clock.
//
// FileWorkerLease is supported on unix-family platforms (Linux, macOS, the
// BSDs). On other platforms Acquire returns an error directing you to supply a
// custom WorkerLease (e.g. one backed by Redis or etcd) — that is also the
// route for coordinating across multiple hosts, since a file lock is local.
type FileWorkerLease struct {
	dir string
}

// NewFileWorkerLease returns a FileWorkerLease that stores lock files under dir.
// The directory is created on first Acquire.
func NewFileWorkerLease(dir string) *FileWorkerLease {
	return &FileWorkerLease{dir: dir}
}

// Acquire opens (creating if needed) a per-key lock file and takes a
// non-blocking exclusive lock on it.
func (l *FileWorkerLease) Acquire(_ context.Context, key string) (LeaseHandle, error) {
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return nil, fmt.Errorf("worker lease mkdir %s: %w", l.dir, err)
	}
	path := filepath.Join(l.dir, sanitizeLeaseKey(key)+".lock")

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("worker lease open %s: %w", path, err)
	}

	locked, err := lockFileExclusive(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("worker lease lock %s: %w", path, err)
	}
	if !locked {
		_ = f.Close()
		return nil, fmt.Errorf("%w: key=%q", ErrWorkerLeaseHeld, key)
	}
	return &fileLeaseHandle{file: f}, nil
}

// fileLeaseHandle holds the locked descriptor. Closing it releases the flock.
type fileLeaseHandle struct {
	mu   sync.Mutex
	file *os.File
}

// Release unlocks and closes the descriptor. Safe to call more than once.
func (h *fileLeaseHandle) Release() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.file == nil {
		return nil
	}
	f := h.file
	h.file = nil
	// Closing the descriptor releases the lock; the explicit unlock is
	// best-effort belt-and-suspenders and its error is intentionally ignored.
	_ = unlockFile(f)
	return f.Close()
}

// sanitizeLeaseKey maps an arbitrary identity string to a single safe filename,
// replacing path separators and characters that are invalid on common
// filesystems (including Windows) so the key never escapes the lease directory.
func sanitizeLeaseKey(key string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', 0:
			return '_'
		}
		return r
	}, key)
}
