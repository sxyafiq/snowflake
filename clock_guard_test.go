package snowflake

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// TestGuardKey_StringDistinguishesLayouts verifies that keys differing in any
// scoping dimension (worker, epoch, layout fingerprint) produce distinct
// strings — a shared string would let an incompatible layout's high-water mark
// be compared against this generator's, which is exactly the bug we must avoid.
func TestGuardKey_StringDistinguishesLayouts(t *testing.T) {
	base := GuardKey{WorkerID: 1, Epoch: 100, TimestampBits: 41, WorkerBits: 10, SequenceBits: 12, TimeUnitMs: 1}

	variants := map[string]GuardKey{
		"worker":    {WorkerID: 2, Epoch: 100, TimestampBits: 41, WorkerBits: 10, SequenceBits: 12, TimeUnitMs: 1},
		"epoch":     {WorkerID: 1, Epoch: 200, TimestampBits: 41, WorkerBits: 10, SequenceBits: 12, TimeUnitMs: 1},
		"timestamp": {WorkerID: 1, Epoch: 100, TimestampBits: 40, WorkerBits: 10, SequenceBits: 12, TimeUnitMs: 1},
		"worker_b":  {WorkerID: 1, Epoch: 100, TimestampBits: 41, WorkerBits: 11, SequenceBits: 12, TimeUnitMs: 1},
		"sequence":  {WorkerID: 1, Epoch: 100, TimestampBits: 41, WorkerBits: 10, SequenceBits: 11, TimeUnitMs: 1},
		"timeunit":  {WorkerID: 1, Epoch: 100, TimestampBits: 41, WorkerBits: 10, SequenceBits: 12, TimeUnitMs: 10},
	}

	for name, v := range variants {
		if base.String() == v.String() {
			t.Errorf("%s variant collides with base key: %q", name, base.String())
		}
	}
}

// TestFileClockGuard_RoundTrip verifies a stored high-water mark is read back
// by a fresh guard instance pointed at the same file — the cross-restart
// contract the whole feature depends on.
func TestFileClockGuard_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guard.json")
	key := GuardKey{WorkerID: 5, Epoch: Epoch, TimestampBits: 41, WorkerBits: 10, SequenceBits: 12, TimeUnitMs: 1}
	ctx := context.Background()

	writer := NewFileClockGuard(path)
	if err := writer.Store(ctx, key, 123456); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// A brand-new instance simulates a process restart.
	reader := NewFileClockGuard(path)
	got, found, err := reader.Load(ctx, key)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !found {
		t.Fatal("Load: found=false, want true after Store")
	}
	if got != 123456 {
		t.Fatalf("Load: got %d, want 123456", got)
	}
}

// TestFileClockGuard_LoadMissingFile returns found=false (not an error) when no
// state has ever been persisted, so first-ever startup is not a hard failure.
func TestFileClockGuard_LoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	key := GuardKey{WorkerID: 0, Epoch: Epoch, TimestampBits: 41, WorkerBits: 10, SequenceBits: 12, TimeUnitMs: 1}

	got, found, err := NewFileClockGuard(path).Load(context.Background(), key)
	if err != nil {
		t.Fatalf("Load on missing file: unexpected error %v", err)
	}
	if found {
		t.Fatalf("Load on missing file: found=true (got %d), want false", got)
	}
}

// TestFileClockGuard_KeysIsolated ensures two keys in the same file don't
// clobber each other — multiple workers may share one guard file.
func TestFileClockGuard_KeysIsolated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guard.json")
	ctx := context.Background()
	g := NewFileClockGuard(path)

	keyA := GuardKey{WorkerID: 1, Epoch: Epoch, TimestampBits: 41, WorkerBits: 10, SequenceBits: 12, TimeUnitMs: 1}
	keyB := GuardKey{WorkerID: 2, Epoch: Epoch, TimestampBits: 41, WorkerBits: 10, SequenceBits: 12, TimeUnitMs: 1}

	if err := g.Store(ctx, keyA, 111); err != nil {
		t.Fatalf("Store A: %v", err)
	}
	if err := g.Store(ctx, keyB, 222); err != nil {
		t.Fatalf("Store B: %v", err)
	}

	gotA, _, _ := g.Load(ctx, keyA)
	gotB, _, _ := g.Load(ctx, keyB)
	if gotA != 111 || gotB != 222 {
		t.Fatalf("key isolation broken: A=%d (want 111), B=%d (want 222)", gotA, gotB)
	}
}

// TestFileClockGuard_StoreOverwrites confirms repeated stores advance the
// high-water mark rather than appending.
func TestFileClockGuard_StoreOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guard.json")
	ctx := context.Background()
	g := NewFileClockGuard(path)
	key := GuardKey{WorkerID: 9, Epoch: Epoch, TimestampBits: 41, WorkerBits: 10, SequenceBits: 12, TimeUnitMs: 1}

	for _, v := range []int64{10, 20, 30} {
		if err := g.Store(ctx, key, v); err != nil {
			t.Fatalf("Store %d: %v", v, err)
		}
	}

	got, _, _ := g.Load(ctx, key)
	if got != 30 {
		t.Fatalf("Load after overwrites: got %d, want 30", got)
	}
}

// memGuard is an in-memory ClockGuard for exercising Generator integration
// without touching the filesystem. It records every Store so tests can assert
// the persist-ahead invariant.
type memGuard struct {
	hw       int64
	found    bool
	loadErr  error
	storeErr error
	stores   []int64
}

func (m *memGuard) Load(_ context.Context, _ GuardKey) (int64, bool, error) {
	return m.hw, m.found, m.loadErr
}

func (m *memGuard) Store(_ context.Context, _ GuardKey, hw int64) error {
	if m.storeErr != nil {
		return m.storeErr
	}
	m.hw = hw
	m.found = true
	m.stores = append(m.stores, hw)
	return nil
}

// TestGenerator_GuardPersistsAhead verifies the core safety invariant: the
// persisted high-water mark is always strictly greater than the last emitted
// logical timestamp, so a crash can never leave emitted IDs unprotected.
func TestGenerator_GuardPersistsAhead(t *testing.T) {
	g := &memGuard{}
	cfg := DefaultConfig(7)
	cfg.ClockGuard = g

	gen, err := NewWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
	}

	if len(g.stores) == 0 {
		t.Fatal("expected an initial persist-ahead Store during construction")
	}

	for i := 0; i < 100; i++ {
		if _, err := gen.GenerateID(); err != nil {
			t.Fatalf("GenerateID #%d: %v", i, err)
		}
	}

	if g.hw <= gen.lastTimestamp {
		t.Fatalf("persist-ahead violated: persisted=%d, lastTimestamp=%d (want persisted > last)", g.hw, gen.lastTimestamp)
	}
}

// TestGenerator_GuardRefusesLargeRegression confirms that a persisted mark far
// ahead of the current clock (the clock-corrected-backward-on-restart case)
// fails construction rather than risking duplicate IDs.
func TestGenerator_GuardRefusesLargeRegression(t *testing.T) {
	// 100s ahead in 1ms units — far beyond MaxWait, so the wait loop gives up.
	g := &memGuard{found: true, hw: nowLogicalMs() + 100_000}
	cfg := DefaultConfig(7)
	cfg.ClockGuard = g
	cfg.ClockGuardInterval = 10 * time.Millisecond
	cfg.ClockGuardMaxWait = 50 * time.Millisecond

	_, err := NewWithConfig(cfg)
	if err == nil {
		t.Fatal("expected error for large clock regression, got nil")
	}
	if !errors.Is(err, ErrClockMovedBack) {
		t.Fatalf("error = %v, want wrapped ErrClockMovedBack", err)
	}
}

// TestGenerator_GuardWaitsOutSmallRegression verifies a small regression within
// MaxWait is waited out and construction then succeeds.
func TestGenerator_GuardWaitsOutSmallRegression(t *testing.T) {
	// A few ms ahead — should be waited out within a generous MaxWait.
	g := &memGuard{found: true, hw: nowLogicalMs() + 3}
	cfg := DefaultConfig(7)
	cfg.ClockGuard = g
	cfg.ClockGuardInterval = 10 * time.Millisecond
	cfg.ClockGuardMaxWait = 2 * time.Second

	gen, err := NewWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
	}
	id, err := gen.GenerateID()
	if err != nil {
		t.Fatalf("GenerateID: %v", err)
	}
	if int64(id) == 0 {
		t.Fatal("expected a non-zero ID after waiting out regression")
	}
}

// TestGenerator_GuardStoreFailureFailsClosed verifies a persistence failure at
// startup aborts construction — the guard exists for safety, so a guard that
// cannot persist must not silently degrade to the unprotected path.
func TestGenerator_GuardStoreFailureFailsClosed(t *testing.T) {
	g := &memGuard{storeErr: errors.New("disk full")}
	cfg := DefaultConfig(7)
	cfg.ClockGuard = g

	if _, err := NewWithConfig(cfg); err == nil {
		t.Fatal("expected construction to fail when guard Store fails, got nil")
	}
}

// nowLogicalMs returns the current wall-clock millisecond, matching the logical
// timestamp units of LayoutDefault (1ms). Tests use it to seed regressions
// relative to "now".
func nowLogicalMs() int64 {
	return time.Now().UnixMilli()
}

// BenchmarkGenerate_NoGuard establishes the per-ID baseline with no ClockGuard
// configured — the default zero-value path that existing callers use.
func BenchmarkGenerate_NoGuard(b *testing.B) {
	gen, err := New(1)
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := gen.GenerateID(); err != nil {
			b.Fatalf("GenerateID: %v", err)
		}
	}
}

// BenchmarkGenerate_WithGuard measures the same path with a ClockGuard
// configured. The additional per-ID cost is a nil-pointer check and an int64
// comparison; periodic Store calls amortize to near-zero at the default
// interval. memGuard's Store is in-memory so this isolates the in-process
// overhead from any I/O backend.
func BenchmarkGenerate_WithGuard(b *testing.B) {
	cfg := DefaultConfig(1)
	cfg.ClockGuard = &memGuard{}
	gen, err := NewWithConfig(cfg)
	if err != nil {
		b.Fatalf("NewWithConfig: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := gen.GenerateID(); err != nil {
			b.Fatalf("GenerateID: %v", err)
		}
	}
}
