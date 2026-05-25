package snowflake

import (
	"context"
	"errors"
	"runtime"
	"testing"
)

// skipIfNoFileLease skips tests that rely on the OS-backed FileWorkerLease on
// platforms where it is unsupported (everything outside the unix family).
func skipIfNoFileLease(t *testing.T) {
	t.Helper()
	switch runtime.GOOS {
	case "windows", "js", "plan9":
		t.Skipf("FileWorkerLease unsupported on %s", runtime.GOOS)
	}
}

// TestFileWorkerLease_AcquireFree acquires a lease on a free key.
func TestFileWorkerLease_AcquireFree(t *testing.T) {
	skipIfNoFileLease(t)
	lease := NewFileWorkerLease(t.TempDir())

	h, err := lease.Acquire(context.Background(), "worker-1")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if h == nil {
		t.Fatal("Acquire returned nil handle without error")
	}
	if err := h.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

// TestFileWorkerLease_SecondHolderRejected verifies the core guarantee: while
// one holder owns a key, a second acquirer (simulating a second live process)
// is rejected with ErrWorkerLeaseHeld.
func TestFileWorkerLease_SecondHolderRejected(t *testing.T) {
	skipIfNoFileLease(t)
	dir := t.TempDir()

	first := NewFileWorkerLease(dir)
	h1, err := first.Acquire(context.Background(), "worker-1")
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer func() { _ = h1.Release() }()

	second := NewFileWorkerLease(dir)
	if _, err := second.Acquire(context.Background(), "worker-1"); !errors.Is(err, ErrWorkerLeaseHeld) {
		t.Fatalf("second Acquire error = %v, want ErrWorkerLeaseHeld", err)
	}
}

// TestFileWorkerLease_ReacquireAfterRelease confirms a released key can be
// taken again — restart after a clean shutdown must work.
func TestFileWorkerLease_ReacquireAfterRelease(t *testing.T) {
	skipIfNoFileLease(t)
	dir := t.TempDir()

	h1, err := NewFileWorkerLease(dir).Acquire(context.Background(), "worker-1")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := h1.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}

	h2, err := NewFileWorkerLease(dir).Acquire(context.Background(), "worker-1")
	if err != nil {
		t.Fatalf("re-Acquire after release: %v", err)
	}
	_ = h2.Release()
}

// TestFileWorkerLease_DistinctKeysIndependent ensures different worker
// identities do not block each other.
func TestFileWorkerLease_DistinctKeysIndependent(t *testing.T) {
	skipIfNoFileLease(t)
	dir := t.TempDir()
	lease := NewFileWorkerLease(dir)

	h1, err := lease.Acquire(context.Background(), "worker-1")
	if err != nil {
		t.Fatalf("Acquire worker-1: %v", err)
	}
	defer func() { _ = h1.Release() }()

	h2, err := lease.Acquire(context.Background(), "worker-2")
	if err != nil {
		t.Fatalf("Acquire worker-2 should not conflict: %v", err)
	}
	_ = h2.Release()
}

// TestGenerator_WorkerLeaseRejectsDuplicate verifies the Generator refuses to
// start when another live generator already holds the same worker identity,
// and that Close releases the lease so a later start succeeds.
func TestGenerator_WorkerLeaseRejectsDuplicate(t *testing.T) {
	skipIfNoFileLease(t)
	dir := t.TempDir()

	cfg1 := DefaultConfig(42)
	cfg1.WorkerLease = NewFileWorkerLease(dir)
	gen1, err := NewWithConfig(cfg1)
	if err != nil {
		t.Fatalf("first NewWithConfig: %v", err)
	}

	cfg2 := DefaultConfig(42) // same worker, layout, epoch -> same lease key
	cfg2.WorkerLease = NewFileWorkerLease(dir)
	if _, err := NewWithConfig(cfg2); !errors.Is(err, ErrWorkerLeaseHeld) {
		t.Fatalf("duplicate worker NewWithConfig error = %v, want ErrWorkerLeaseHeld", err)
	}

	// Releasing the first generator's lease frees the identity.
	if err := gen1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	cfg3 := DefaultConfig(42)
	cfg3.WorkerLease = NewFileWorkerLease(dir)
	gen3, err := NewWithConfig(cfg3)
	if err != nil {
		t.Fatalf("NewWithConfig after Close: %v", err)
	}
	_ = gen3.Close()
}

// TestGenerator_CloseWithoutLeaseIsNoop verifies Close is safe (and nil-error)
// on a generator created without a WorkerLease — preserving backward
// compatibility for callers that never call Close.
func TestGenerator_CloseWithoutLeaseIsNoop(t *testing.T) {
	gen, err := New(0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := gen.Close(); err != nil {
		t.Fatalf("Close on lease-less generator: %v", err)
	}
	// Double Close must also be safe.
	if err := gen.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
