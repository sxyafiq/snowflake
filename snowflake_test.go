package snowflake

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestNew tests basic generator creation
func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		workerID int64
		wantErr  bool
	}{
		{"Valid worker ID 0", 0, false},
		{"Valid worker ID 512", 512, false},
		{"Valid worker ID 1023", 1023, false},
		{"Invalid worker ID -1", -1, true},
		{"Invalid worker ID 1024", 1024, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen, err := New(tt.workerID)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && gen == nil {
				t.Error("New() returned nil generator without error")
			}
			if !tt.wantErr && gen.WorkerID() != tt.workerID {
				t.Errorf("WorkerID() = %v, want %v", gen.WorkerID(), tt.workerID)
			}
		})
	}
}

// TestGenerate tests basic ID generation
func TestGenerate(t *testing.T) {
	gen, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	id, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if id <= 0 {
		t.Errorf("Generate() returned non-positive ID: %d", id)
	}

	// Verify ID structure
	timestamp, workerID, sequence := ParseIDComponents(id)
	if workerID != 1 {
		t.Errorf("ParseIDComponents() workerID = %d, want 1", workerID)
	}
	if timestamp <= Epoch {
		t.Errorf("ParseIDComponents() timestamp = %d, should be > epoch %d", timestamp, Epoch)
	}
	if sequence < 0 || sequence > MaxSequence {
		t.Errorf("ParseIDComponents() sequence = %d, want 0-%d", sequence, MaxSequence)
	}
}

// TestUniqueness tests that generated IDs are unique
func TestUniqueness(t *testing.T) {
	gen, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	count := 100000
	ids := make(map[int64]bool, count)

	for i := 0; i < count; i++ {
		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v at iteration %d", err, i)
		}

		if ids[id] {
			t.Fatalf("Duplicate ID detected: %d at iteration %d", id, i)
		}
		ids[id] = true
	}

	if len(ids) != count {
		t.Errorf("Generated %d unique IDs, want %d", len(ids), count)
	}
}

// TestOrdering tests that IDs are monotonically increasing
func TestOrdering(t *testing.T) {
	gen, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var prev int64
	for i := 0; i < 10000; i++ {
		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}

		if id <= prev {
			t.Fatalf("IDs not monotonic: prev=%d, current=%d at iteration %d", prev, id, i)
		}
		prev = id
	}
}

// TestConcurrency tests concurrent ID generation
func TestConcurrency(t *testing.T) {
	gen, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	goroutines := 100
	idsPerGoroutine := 1000
	totalIDs := goroutines * idsPerGoroutine

	ids := sync.Map{}
	var wg sync.WaitGroup
	errors := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				id, err := gen.Generate()
				if err != nil {
					errors <- err
					return
				}

				if _, exists := ids.LoadOrStore(id, true); exists {
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
		t.Fatalf("Concurrent generation error: %v", err)
	}

	// Count unique IDs
	count := 0
	ids.Range(func(_, _ interface{}) bool {
		count++
		return true
	})

	if count != totalIDs {
		t.Errorf("Generated %d unique IDs, want %d", count, totalIDs)
	}
}

// TestSequenceOverflow tests behavior when sequence overflows
func TestSequenceOverflow(t *testing.T) {
	gen, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Generate enough IDs in rapid succession to cause sequence overflow
	// This should trigger a wait for the next millisecond
	count := 5000
	for i := 0; i < count; i++ {
		_, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v at iteration %d", err, i)
		}
	}

	metrics := gen.GetMetrics()
	if metrics.Generated != int64(count) {
		t.Errorf("Metrics.Generated = %d, want %d", metrics.Generated, count)
	}

	// We likely triggered at least one sequence overflow
	if metrics.SequenceOverflow > 0 {
		t.Logf("Sequence overflows: %d (expected for rapid generation)", metrics.SequenceOverflow)
	}
}

// TestMetrics tests that metrics are recorded correctly
func TestMetrics(t *testing.T) {
	gen, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Generate some IDs
	count := 1000
	for i := 0; i < count; i++ {
		_, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
	}

	metrics := gen.GetMetrics()
	if metrics.Generated != int64(count) {
		t.Errorf("Metrics.Generated = %d, want %d", metrics.Generated, count)
	}

	// Reset metrics
	gen.ResetMetrics()
	metrics = gen.GetMetrics()
	if metrics.Generated != 0 {
		t.Errorf("After reset, Metrics.Generated = %d, want 0", metrics.Generated)
	}
}

// TestContext tests context cancellation
func TestContext(t *testing.T) {
	gen, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Test with canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = gen.GenerateWithContext(ctx)
	if err != ErrContextCanceled {
		t.Errorf("GenerateWithContext() with canceled context error = %v, want %v", err, ErrContextCanceled)
	}

	// Test with timeout context
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, err := gen.GenerateWithContext(ctx)
	if err != nil {
		t.Errorf("GenerateWithContext() with valid context error = %v", err)
	}
	if id <= 0 {
		t.Errorf("GenerateWithContext() returned non-positive ID: %d", id)
	}
}

// TestConfig tests custom configuration
func TestConfig(t *testing.T) {
	cfg := Config{
		WorkerID:         42,
		Epoch:            Epoch,
		MaxClockBackward: 10 * time.Millisecond,
		EnableMetrics:    true,
	}

	gen, err := NewWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewWithConfig() error = %v", err)
	}

	if gen.WorkerID() != 42 {
		t.Errorf("WorkerID() = %d, want 42", gen.WorkerID())
	}

	// Test invalid config
	invalidCfg := Config{
		WorkerID: -1,
		Epoch:    Epoch,
	}

	_, err = NewWithConfig(invalidCfg)
	if err == nil {
		t.Error("NewWithConfig() with invalid config should return error")
	}
}

// TestDefaultGenerator tests the package-level default generator
func TestDefaultGenerator(t *testing.T) {
	id, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if id <= 0 {
		t.Errorf("Generate() returned non-positive ID: %d", id)
	}

	// Test MustGenerate
	id2 := MustGenerate()
	if id2 <= id {
		t.Errorf("MustGenerate() = %d, should be > %d", id2, id)
	}
}

// TestParseID tests ID parsing
func TestParseID(t *testing.T) {
	gen, err := New(42)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	id, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	timestamp, workerID, sequence := ParseIDComponents(id)

	if workerID != 42 {
		t.Errorf("ParseIDComponents() workerID = %d, want 42", workerID)
	}

	if timestamp <= Epoch {
		t.Errorf("ParseIDComponents() timestamp = %d, should be > epoch %d", timestamp, Epoch)
	}

	if sequence < 0 || sequence > MaxSequence {
		t.Errorf("ParseIDComponents() sequence = %d, out of valid range [0, %d]", sequence, MaxSequence)
	}

	// Test ExtractTimestamp function
	ts := ExtractTimestamp(id)
	expectedTS := time.Unix(timestamp/1000, (timestamp%1000)*1000000)
	if !ts.Equal(expectedTS) {
		t.Errorf("ExtractTimestamp() = %v, want %v", ts, expectedTS)
	}
}

// TestMultipleWorkers tests multiple generators with different worker IDs
func TestMultipleWorkers(t *testing.T) {
	workers := 10
	idsPerWorker := 1000

	var wg sync.WaitGroup
	ids := sync.Map{}
	errors := make(chan error, workers)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		workerID := int64(w)

		go func(wid int64) {
			defer wg.Done()

			gen, err := New(wid)
			if err != nil {
				errors <- err
				return
			}

			for i := 0; i < idsPerWorker; i++ {
				id, err := gen.Generate()
				if err != nil {
					errors <- err
					return
				}

				if _, exists := ids.LoadOrStore(id, wid); exists {
					errors <- err
					return
				}
			}
		}(workerID)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Fatalf("Multi-worker generation error: %v", err)
	}

	// Count unique IDs
	count := 0
	ids.Range(func(_, _ interface{}) bool {
		count++
		return true
	})

	expected := workers * idsPerWorker
	if count != expected {
		t.Errorf("Generated %d unique IDs across %d workers, want %d", count, workers, expected)
	}
}

// BenchmarkGenerate benchmarks ID generation
func BenchmarkGenerate(b *testing.B) {
	gen, err := New(1)
	if err != nil {
		b.Fatalf("New() error = %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.Generate()
		if err != nil {
			b.Fatalf("Generate() error = %v", err)
		}
	}
}

// BenchmarkGenerateConcurrent benchmarks concurrent ID generation
func BenchmarkGenerateConcurrent(b *testing.B) {
	gen, err := New(1)
	if err != nil {
		b.Fatalf("New() error = %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := gen.Generate()
			if err != nil {
				b.Fatalf("Generate() error = %v", err)
			}
		}
	})
}

// BenchmarkParseIDComponents benchmarks ID parsing
func BenchmarkParseIDComponents(b *testing.B) {
	gen, _ := New(1)
	id, _ := gen.Generate()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseIDComponents(id)
	}
}

// BenchmarkDefaultGenerate benchmarks the default generator
func BenchmarkDefaultGenerate(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Generate()
		if err != nil {
			b.Fatalf("Generate() error = %v", err)
		}
	}
}
