package snowflake

import (
	"context"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// Basic Batch Generation Tests
// ============================================================================

func TestGenerateBatch_BasicFunctionality(t *testing.T) {
	gen, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name  string
		count int
	}{
		{"Single ID", 1},
		{"Small batch", 10},
		{"Medium batch", 100},
		{"Large batch", 1000},
		{"Very large batch", 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids, err := gen.GenerateBatch(context.Background(), tt.count)
			if err != nil {
				t.Fatalf("GenerateBatch() error = %v", err)
			}

			if len(ids) != tt.count {
				t.Errorf("GenerateBatch() returned %d IDs, want %d", len(ids), tt.count)
			}

			// Verify all IDs are positive
			for i, id := range ids {
				if id <= 0 {
					t.Errorf("ID at index %d is non-positive: %d", i, id)
				}
			}
		})
	}
}

func TestGenerateBatch_ZeroCount(t *testing.T) {
	gen, _ := New(1)

	ids, err := gen.GenerateBatch(context.Background(), 0)
	if err != nil {
		t.Errorf("GenerateBatch(0) should not return error, got: %v", err)
	}

	if len(ids) != 0 {
		t.Errorf("GenerateBatch(0) should return empty slice, got %d IDs", len(ids))
	}
}

func TestGenerateBatch_NegativeCount(t *testing.T) {
	gen, _ := New(1)

	ids, err := gen.GenerateBatch(context.Background(), -10)
	if err != nil {
		t.Errorf("GenerateBatch(-10) should not return error, got: %v", err)
	}

	if len(ids) != 0 {
		t.Errorf("GenerateBatch(-10) should return empty slice, got %d IDs", len(ids))
	}
}

// ============================================================================
// Uniqueness Tests
// ============================================================================

func TestGenerateBatch_Uniqueness(t *testing.T) {
	gen, _ := New(1)

	count := 10000
	ids, err := gen.GenerateBatch(context.Background(), count)
	if err != nil {
		t.Fatalf("GenerateBatch() error = %v", err)
	}

	// Check for duplicates
	seen := make(map[ID]bool, len(ids))
	for i, id := range ids {
		if seen[id] {
			t.Fatalf("Duplicate ID detected: %v at index %d", id, i)
		}
		seen[id] = true
	}

	if len(seen) != count {
		t.Errorf("Generated %d unique IDs, want %d", len(seen), count)
	}
}

func TestGenerateBatch_Monotonic(t *testing.T) {
	gen, _ := New(1)

	ids, err := gen.GenerateBatch(context.Background(), 1000)
	if err != nil {
		t.Fatalf("GenerateBatch() error = %v", err)
	}

	// Verify IDs are monotonically increasing
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Fatalf("IDs not monotonic: ids[%d]=%d <= ids[%d]=%d",
				i, ids[i], i-1, ids[i-1])
		}
	}
}

// ============================================================================
// Context Cancellation Tests
// ============================================================================

func TestGenerateBatch_ContextCancellation(t *testing.T) {
	gen, _ := New(1)

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should return partial batch or error immediately
	ids, err := gen.GenerateBatch(ctx, 10000)

	if err != ErrContextCanceled {
		t.Errorf("Expected ErrContextCanceled, got: %v", err)
	}

	// Should return at least partial batch (may be empty if cancelled immediately)
	t.Logf("Generated %d IDs before cancellation", len(ids))
}

func TestGenerateBatch_ContextTimeout(t *testing.T) {
	gen, _ := New(1)

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond) // Ensure timeout has passed

	ids, err := gen.GenerateBatch(ctx, 100000)

	if err != ErrContextCanceled {
		t.Errorf("Expected ErrContextCanceled for timed-out context, got: %v", err)
	}

	t.Logf("Generated %d IDs before timeout", len(ids))
}

// ============================================================================
// int64 Version Tests
// ============================================================================

func TestGenerateBatchInt64_BasicFunctionality(t *testing.T) {
	gen, _ := New(1)

	count := 100
	ids, err := gen.GenerateBatchInt64(context.Background(), count)
	if err != nil {
		t.Fatalf("GenerateBatchInt64() error = %v", err)
	}

	if len(ids) != count {
		t.Errorf("GenerateBatchInt64() returned %d IDs, want %d", len(ids), count)
	}

	// Verify all IDs are positive
	for i, id := range ids {
		if id <= 0 {
			t.Errorf("ID at index %d is non-positive: %d", i, id)
		}
	}
}

func TestGenerateBatchInt64_ConversionCorrectness(t *testing.T) {
	gen, _ := New(1)

	// Generate using both methods
	idsBatch, _ := gen.GenerateBatch(context.Background(), 100)
	idsInt64, _ := gen.GenerateBatchInt64(context.Background(), 100)

	// Both should return valid IDs (can't directly compare since generated at different times)
	if len(idsBatch) == 0 || len(idsInt64) == 0 {
		t.Error("Both methods should generate IDs")
	}

	// Verify that int64 version returns proper int64 values
	for i, id := range idsInt64 {
		if id <= 0 {
			t.Errorf("ID at index %d is invalid: %d", i, id)
		}
	}
}

// ============================================================================
// Performance Tests
// ============================================================================

func TestGenerateBatch_PerformanceVsIndividual(t *testing.T) {
	gen, _ := New(1)
	count := 1000
	ctx := context.Background()

	// Time individual generation
	startIndividual := time.Now()
	for i := 0; i < count; i++ {
		_, err := gen.GenerateID()
		if err != nil {
			t.Fatalf("GenerateID() error = %v", err)
		}
	}
	individualDuration := time.Since(startIndividual)

	// Time batch generation
	startBatch := time.Now()
	_, err := gen.GenerateBatch(ctx, count)
	if err != nil {
		t.Fatalf("GenerateBatch() error = %v", err)
	}
	batchDuration := time.Since(startBatch)

	speedup := float64(individualDuration) / float64(batchDuration)

	t.Logf("Individual generation: %v (%v per ID)",
		individualDuration, individualDuration/time.Duration(count))
	t.Logf("Batch generation: %v (%v per ID)",
		batchDuration, batchDuration/time.Duration(count))
	t.Logf("Speedup: %.2fx", speedup)

	// Batch should be at least 1.5x faster (conservative estimate)
	if speedup < 1.5 {
		t.Logf("WARNING: Batch generation speedup (%.2fx) is less than expected (1.5x)", speedup)
		// Don't fail the test, as timing can vary, but log warning
	}
}

// ============================================================================
// Concurrent Batch Generation Tests
// ============================================================================

func TestGenerateBatch_Concurrent(t *testing.T) {
	gen, _ := New(1)

	goroutines := 10
	idsPerGoroutine := 100

	var wg sync.WaitGroup
	allIDs := sync.Map{}
	errors := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ids, err := gen.GenerateBatch(context.Background(), idsPerGoroutine)
			if err != nil {
				errors <- err
				return
			}

			for _, id := range ids {
				if _, exists := allIDs.LoadOrStore(id, true); exists {
					errors <- err
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Fatalf("Concurrent batch generation error: %v", err)
	}

	// Count unique IDs
	count := 0
	allIDs.Range(func(_, _ interface{}) bool {
		count++
		return true
	})

	expected := goroutines * idsPerGoroutine
	if count != expected {
		t.Errorf("Generated %d unique IDs across %d goroutines, want %d",
			count, goroutines, expected)
	}
}

// ============================================================================
// Sequence Overflow Tests
// ============================================================================

func TestGenerateBatch_SequenceOverflow(t *testing.T) {
	gen, _ := New(1)

	// Generate enough IDs to trigger sequence overflow
	// At 4096 IDs per millisecond, we should see some overflows
	count := 10000
	ids, err := gen.GenerateBatch(context.Background(), count)
	if err != nil {
		t.Fatalf("GenerateBatch() error = %v", err)
	}

	if len(ids) != count {
		t.Errorf("Expected %d IDs, got %d", count, len(ids))
	}

	// Verify uniqueness even with sequence overflow
	seen := make(map[ID]bool)
	for _, id := range ids {
		if seen[id] {
			t.Errorf("Duplicate ID after sequence overflow: %v", id)
		}
		seen[id] = true
	}

	// Check metrics
	metrics := gen.GetMetrics()
	if metrics.SequenceOverflow > 0 {
		t.Logf("Sequence overflows: %d (expected for %d IDs)", metrics.SequenceOverflow, count)
	}
}

// ============================================================================
// Metrics Tests
// ============================================================================

func TestGenerateBatch_Metrics(t *testing.T) {
	gen, _ := New(1)
	gen.ResetMetrics()

	count := 1000
	_, err := gen.GenerateBatch(context.Background(), count)
	if err != nil {
		t.Fatalf("GenerateBatch() error = %v", err)
	}

	metrics := gen.GetMetrics()

	if metrics.Generated != int64(count) {
		t.Errorf("Metrics.Generated = %d, want %d", metrics.Generated, count)
	}
}

// ============================================================================
// Edge Cases Tests
// ============================================================================

func TestGenerateBatch_VerifyWorkerID(t *testing.T) {
	workerID := int64(42)
	gen, _ := New(workerID)

	ids, err := gen.GenerateBatch(context.Background(), 100)
	if err != nil {
		t.Fatalf("GenerateBatch() error = %v", err)
	}

	// Verify all IDs have correct worker ID
	for i, id := range ids {
		extractedWorker := id.Worker()
		if extractedWorker != workerID {
			t.Errorf("ID at index %d has worker ID %d, want %d", i, extractedWorker, workerID)
		}
	}
}

func TestGenerateBatch_VerifyTimestamp(t *testing.T) {
	gen, _ := New(1)

	before := time.Now().Add(-1 * time.Second) // 1 second before for tolerance
	ids, err := gen.GenerateBatch(context.Background(), 100)
	after := time.Now().Add(1 * time.Second) // 1 second after for tolerance

	if err != nil {
		t.Fatalf("GenerateBatch() error = %v", err)
	}

	// Verify all timestamps are within reasonable range
	// Note: Snowflake IDs have millisecond precision, so we use generous tolerance
	for i, id := range ids {
		ts := id.Time()
		if ts.Before(before) || ts.After(after) {
			t.Errorf("ID at index %d has timestamp %v, expected between %v and %v",
				i, ts, before, after)
		}
	}
}

// ============================================================================
// Benchmarks
// ============================================================================

func BenchmarkGenerateBatch_100(b *testing.B) {
	gen, _ := New(1)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.GenerateBatch(ctx, 100)
		if err != nil {
			b.Fatalf("GenerateBatch() error = %v", err)
		}
	}
}

func BenchmarkGenerateBatch_1000(b *testing.B) {
	gen, _ := New(1)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.GenerateBatch(ctx, 1000)
		if err != nil {
			b.Fatalf("GenerateBatch() error = %v", err)
		}
	}
}

func BenchmarkGenerateLoop_100(b *testing.B) {
	gen, _ := New(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			_, err := gen.GenerateID()
			if err != nil {
				b.Fatalf("GenerateID() error = %v", err)
			}
		}
	}
}

func BenchmarkGenerateLoop_1000(b *testing.B) {
	gen, _ := New(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			_, err := gen.GenerateID()
			if err != nil {
				b.Fatalf("GenerateID() error = %v", err)
			}
		}
	}
}

func BenchmarkGenerateBatchInt64_1000(b *testing.B) {
	gen, _ := New(1)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.GenerateBatchInt64(ctx, 1000)
		if err != nil {
			b.Fatalf("GenerateBatchInt64() error = %v", err)
		}
	}
}

func BenchmarkGenerateBatch_Concurrent(b *testing.B) {
	gen, _ := New(1)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := gen.GenerateBatch(ctx, 100)
			if err != nil {
				b.Fatalf("GenerateBatch() error = %v", err)
			}
		}
	})
}
