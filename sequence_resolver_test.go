package snowflake

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// TestLocalResolver_BasicSequence covers the default-path semantics of the
// in-process resolver: zero on a new time unit, then 1, 2, 3, ...
func TestLocalResolver_BasicSequence(t *testing.T) {
	r := NewLocalResolver(MaxSequence)
	ctx := context.Background()

	for i := int64(0); i < 5; i++ {
		got, err := r.Next(ctx, 100)
		if err != nil {
			t.Fatalf("Next: unexpected error %v", err)
		}
		if got != i {
			t.Fatalf("Next: got %d, want %d", got, i)
		}
	}
}

// TestLocalResolver_TimeUnitAdvanceResetsSequence verifies the resolver
// resets to zero when the time unit advances.
func TestLocalResolver_TimeUnitAdvanceResetsSequence(t *testing.T) {
	r := NewLocalResolver(MaxSequence)
	ctx := context.Background()

	// Burn a few sequences in unit 100.
	for i := 0; i < 3; i++ {
		_, _ = r.Next(ctx, 100)
	}

	// Advancing to a new time unit must reset to zero.
	got, err := r.Next(ctx, 101)
	if err != nil {
		t.Fatalf("Next: unexpected error %v", err)
	}
	if got != 0 {
		t.Fatalf("Next on new time unit: got %d, want 0", got)
	}
}

// TestLocalResolver_ExhaustsAtMax verifies the resolver returns
// ErrSequenceExhausted exactly when maxSequence is reached.
func TestLocalResolver_ExhaustsAtMax(t *testing.T) {
	const maxSeq = int64(7) // 3 bits — easy to exhaust
	r := NewLocalResolver(maxSeq)
	ctx := context.Background()

	// 0..maxSeq should succeed; the next call must signal exhaustion.
	for i := int64(0); i <= maxSeq; i++ {
		got, err := r.Next(ctx, 1)
		if err != nil {
			t.Fatalf("Next at %d: unexpected error %v", i, err)
		}
		if got != i {
			t.Fatalf("Next at %d: got %d, want %d", i, got, i)
		}
	}

	if _, err := r.Next(ctx, 1); !errors.Is(err, ErrSequenceExhausted) {
		t.Fatalf("Next after exhaustion: got %v, want ErrSequenceExhausted", err)
	}
}

// fakeResolver lets tests inject a programmable SequenceResolver.
type fakeResolver struct {
	mu    sync.Mutex
	calls int
	seq   int64
	err   error
}

func (f *fakeResolver) Next(_ context.Context, _ int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.seq, f.err
}

// TestGenerator_UsesCustomResolver verifies that a Config-supplied resolver
// receives Next calls and its return value lands in the ID's sequence bits.
func TestGenerator_UsesCustomResolver(t *testing.T) {
	const wantSeq = int64(42)
	fake := &fakeResolver{seq: wantSeq}

	cfg := DefaultConfig(0)
	cfg.SequenceResolver = fake
	gen, err := NewWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
	}

	id, err := gen.GenerateID()
	if err != nil {
		t.Fatalf("GenerateID: %v", err)
	}

	if fake.calls == 0 {
		t.Fatal("expected resolver to be called at least once")
	}

	_, _, gotSeq := id.Components()
	if gotSeq != wantSeq {
		t.Fatalf("ID sequence = %d, want %d", gotSeq, wantSeq)
	}
}

// TestGenerator_PropagatesResolverError ensures non-Exhausted errors surface
// instead of being mistaken for retriable conditions.
func TestGenerator_PropagatesResolverError(t *testing.T) {
	wantErr := errors.New("redis: connection refused")
	fake := &fakeResolver{err: wantErr}

	cfg := DefaultConfig(0)
	cfg.SequenceResolver = fake
	gen, _ := NewWithConfig(cfg)

	_, err := gen.GenerateID()
	if !errors.Is(err, wantErr) {
		t.Fatalf("GenerateID error = %v, want %v", err, wantErr)
	}
}

// exhaustOnceResolver returns ErrSequenceExhausted on the first call and a
// fresh sequence on the second. Verifies the Generator retries on exhaustion
// rather than surfacing the error to the caller.
type exhaustOnceResolver struct {
	calls atomic.Int64
}

func (r *exhaustOnceResolver) Next(_ context.Context, _ int64) (int64, error) {
	if r.calls.Add(1) == 1 {
		return 0, ErrSequenceExhausted
	}
	return 7, nil
}

func TestGenerator_RetriesOnExhausted(t *testing.T) {
	r := &exhaustOnceResolver{}

	cfg := DefaultConfig(0)
	cfg.SequenceResolver = r
	gen, _ := NewWithConfig(cfg)

	id, err := gen.GenerateID()
	if err != nil {
		t.Fatalf("GenerateID: %v", err)
	}
	if got := r.calls.Load(); got != 2 {
		t.Fatalf("resolver call count = %d, want 2 (first exhausted, second succeeded)", got)
	}

	_, _, gotSeq := id.Components()
	if gotSeq != 7 {
		t.Fatalf("ID sequence = %d, want 7", gotSeq)
	}

	// The retry path must increment the sequence-overflow metric so that
	// resolver-side starvation is observable in production.
	if m := gen.GetMetrics(); m.SequenceOverflow != 1 {
		t.Fatalf("SequenceOverflow metric = %d, want 1", m.SequenceOverflow)
	}
}

// TestGenerator_DefaultBehaviorUnchanged is a smoke test confirming nil
// resolver still produces unique, monotonically-increasing IDs.
func TestGenerator_DefaultBehaviorUnchanged(t *testing.T) {
	gen, err := New(0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const n = 1000
	seen := make(map[ID]struct{}, n)
	var prev ID
	for i := 0; i < n; i++ {
		id, err := gen.GenerateID()
		if err != nil {
			t.Fatalf("GenerateID #%d: %v", i, err)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ID %d at iter %d", id, i)
		}
		seen[id] = struct{}{}
		if id < prev {
			t.Fatalf("non-monotonic ID at iter %d: prev=%d cur=%d", i, prev, id)
		}
		prev = id
	}
}

// TestGenerator_ResolverConcurrent stresses the resolver path under
// concurrent goroutines to catch any locking regressions.
func TestGenerator_ResolverConcurrent(t *testing.T) {
	cfg := DefaultConfig(0)
	cfg.SequenceResolver = NewLocalResolver(MaxSequence)
	gen, err := NewWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
	}

	const goroutines = 16
	const perGoroutine = 200

	var wg sync.WaitGroup
	results := make(chan ID, goroutines*perGoroutine)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				id, err := gen.GenerateID()
				if err != nil {
					t.Errorf("GenerateID: %v", err)
					return
				}
				results <- id
			}
		}()
	}
	wg.Wait()
	close(results)

	seen := make(map[ID]struct{}, goroutines*perGoroutine)
	for id := range results {
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ID under concurrent load: %d", id)
		}
		seen[id] = struct{}{}
	}
}
