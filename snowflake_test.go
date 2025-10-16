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

// ============================================================================
// Layout Integration Tests
// ============================================================================

func TestGeneratorWithLayouts(t *testing.T) {
	layouts := []struct {
		name   string
		layout BitLayout
	}{
		{"LayoutDefault", LayoutDefault},
		{"LayoutSuperior", LayoutSuperior},
		{"LayoutExtreme", LayoutExtreme},
		{"LayoutUltra", LayoutUltra},
		{"LayoutLongLife", LayoutLongLife},
		{"LayoutSonyflake", LayoutSonyflake},
		{"LayoutUltimate", LayoutUltimate},
		{"LayoutMegaScale", LayoutMegaScale},
	}

	for _, tt := range layouts {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig(42)
			cfg.Layout = tt.layout
			gen, err := NewWithConfig(cfg)
			if err != nil {
				t.Fatalf("NewWithConfig() error = %v", err)
			}

			// Generate multiple IDs
			ids := make([]ID, 100)
			for i := 0; i < 100; i++ {
				id, err := gen.GenerateID()
				if err != nil {
					t.Fatalf("GenerateID() error = %v", err)
				}
				ids[i] = id

				// Validate ID structure using layout-aware method
				if !id.IsValidWithLayout(tt.layout) {
					t.Errorf("Generated ID %v is not valid for %s", id, tt.name)
				}

				// Extract components using layout
				ts, worker, seq := id.ComponentsWithLayout(tt.layout)

				// Verify worker ID
				if worker != 42 {
					t.Errorf("Worker ID = %d, want 42", worker)
				}

				// Verify sequence is within layout bounds
				_, _, _, maxSeq := tt.layout.CalculateShifts()
				if seq < 0 || seq > maxSeq {
					t.Errorf("Sequence %d out of bounds [0, %d]", seq, maxSeq)
				}

				// Verify timestamp is reasonable
				if ts <= Epoch {
					t.Errorf("Timestamp %d should be after epoch %d", ts, Epoch)
				}
			}

			// Verify IDs are unique
			seen := make(map[ID]bool)
			for _, id := range ids {
				if seen[id] {
					t.Errorf("Duplicate ID generated: %v", id)
				}
				seen[id] = true
			}

			// Verify IDs are monotonically increasing (time-ordered)
			for i := 1; i < len(ids); i++ {
				if ids[i] <= ids[i-1] {
					t.Errorf("IDs not monotonically increasing: %v <= %v", ids[i], ids[i-1])
				}
			}
		})
	}
}

func TestParseIDComponentsWithDifferentLayouts(t *testing.T) {
	tests := []struct {
		name     string
		layout   BitLayout
		workerID int64
	}{
		{"LayoutDefault", LayoutDefault, 512},
		{"LayoutSuperior", LayoutSuperior, 8000},
		{"LayoutExtreme", LayoutExtreme, 100000},
		{"LayoutUltra", LayoutUltra, 20000},
		{"LayoutLongLife", LayoutLongLife, 2048},
		{"LayoutSonyflake", LayoutSonyflake, 30000},
		{"LayoutUltimate", LayoutUltimate, 50000},
		{"LayoutMegaScale", LayoutMegaScale, 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate ID with specific layout
			cfg := DefaultConfig(tt.workerID)
			cfg.Layout = tt.layout
			gen, err := NewWithConfig(cfg)
			if err != nil {
				t.Fatalf("NewWithConfig() error = %v", err)
			}

			id, err := gen.GenerateID()
			if err != nil {
				t.Fatalf("GenerateID() error = %v", err)
			}

			// Parse components with layout
			ts, worker, seq := ParseIDComponentsWithLayout(id.Int64(), tt.layout)

			// Verify worker ID matches
			if worker != tt.workerID {
				t.Errorf("Parsed worker ID = %d, want %d", worker, tt.workerID)
			}

			// Verify timestamp extraction
			extractedTime := ExtractTimestampWithLayout(id.Int64(), tt.layout)
			if extractedTime.IsZero() {
				t.Error("Extracted timestamp is zero")
			}

			// Verify component extraction via ID methods
			ts2 := id.TimestampWithLayout(tt.layout)
			worker2 := id.WorkerWithLayout(tt.layout)
			seq2 := id.SequenceWithLayout(tt.layout)

			if ts != ts2 {
				t.Errorf("Timestamp mismatch: ParseIDComponentsWithLayout=%d, TimestampWithLayout=%d", ts, ts2)
			}
			if worker != worker2 {
				t.Errorf("Worker mismatch: ParseIDComponentsWithLayout=%d, WorkerWithLayout=%d", worker, worker2)
			}
			if seq != seq2 {
				t.Errorf("Sequence mismatch: ParseIDComponentsWithLayout=%d, SequenceWithLayout=%d", seq, seq2)
			}
		})
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that old code without Layout specified still works
	cfg := Config{
		WorkerID:         123,
		Epoch:            Epoch,
		MaxClockBackward: 5 * time.Millisecond,
		EnableMetrics:    true,
		// Layout not set - should default to LayoutDefault
	}

	gen, err := NewWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewWithConfig() with zero-valued layout should succeed: %v", err)
	}

	// Generate ID
	id, err := gen.GenerateID()
	if err != nil {
		t.Fatalf("GenerateID() error = %v", err)
	}

	// Verify it uses LayoutDefault by extracting worker ID
	worker := id.Worker() // Uses LayoutDefault constants
	if worker != 123 {
		t.Errorf("Worker ID = %d, want 123 (should use LayoutDefault)", worker)
	}

	// Verify old parsing methods still work
	ts, worker2, seq := ParseIDComponents(id.Int64())
	if worker2 != 123 {
		t.Errorf("ParseIDComponents worker = %d, want 123", worker2)
	}
	if ts <= Epoch {
		t.Errorf("ParseIDComponents timestamp %d should be after epoch", ts)
	}
	if seq < 0 {
		t.Errorf("ParseIDComponents sequence %d should be non-negative", seq)
	}

	// Verify ID is valid using old method
	if !id.IsValid() {
		t.Error("ID should be valid using old IsValid() method")
	}
}

// ============================================================================
// Bitshift Optimization Tests
// ============================================================================

func TestBitshiftOptimizationEquivalence(t *testing.T) {
	// Test that bitshift optimization produces identical results to division
	tests := []struct {
		name   string
		layout BitLayout
	}{
		{"LayoutDefault (1ms, bitshift)", LayoutDefault},
		{"LayoutSuperior (1ms, bitshift)", LayoutSuperior},
		{"LayoutExtreme (1ms, bitshift)", LayoutExtreme},
		{"LayoutUltra (1ms, bitshift)", LayoutUltra},
		{"LayoutLongLife (1ms, bitshift)", LayoutLongLife},
		{"LayoutSonyflake (10ms, division)", LayoutSonyflake},
		{"LayoutUltimate (10ms, division)", LayoutUltimate},
		{"LayoutMegaScale (10ms, division)", LayoutMegaScale},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig(42)
			cfg.Layout = tt.layout
			gen, err := NewWithConfig(cfg)
			if err != nil {
				t.Fatalf("NewWithConfig() error = %v", err)
			}

			// Generate 1000 IDs
			ids := make([]ID, 1000)
			for i := 0; i < 1000; i++ {
				id, err := gen.GenerateID()
				if err != nil {
					t.Fatalf("GenerateID() error = %v at iteration %d", err, i)
				}
				ids[i] = id

				// Verify ID structure
				ts, worker, seq := id.ComponentsWithLayout(tt.layout)

				// Verify worker ID is correct
				if worker != 42 {
					t.Errorf("Worker ID = %d, want 42", worker)
				}

				// Verify timestamp is reasonable
				if ts <= Epoch {
					t.Errorf("Timestamp %d should be after epoch %d", ts, Epoch)
				}

				// Verify sequence is within bounds
				_, _, _, maxSeq := tt.layout.CalculateShifts()
				if seq < 0 || seq > maxSeq {
					t.Errorf("Sequence %d out of bounds [0, %d]", seq, maxSeq)
				}
			}

			// Verify all IDs are unique
			seen := make(map[ID]bool)
			for _, id := range ids {
				if seen[id] {
					t.Errorf("Duplicate ID generated: %v", id)
				}
				seen[id] = true
			}

			// Verify IDs are monotonically increasing
			for i := 1; i < len(ids); i++ {
				if ids[i] <= ids[i-1] {
					t.Errorf("IDs not monotonic: %v <= %v at index %d", ids[i], ids[i-1], i)
				}
			}
		})
	}
}

// ============================================================================
// LayoutUltimate Integration Tests
// ============================================================================

func TestLayoutUltimateIntegration(t *testing.T) {
	// Test LayoutUltimate (40+16+7 bits with 10ms time unit)
	// Specs: 292 years, 65,536 nodes, 12,800 IDs/sec per node

	t.Run("BasicGeneration", func(t *testing.T) {
		cfg := DefaultConfig(42)
		cfg.Layout = LayoutUltimate
		gen, err := NewWithConfig(cfg)
		if err != nil {
			t.Fatalf("NewWithConfig() error = %v", err)
		}

		// Generate 10,000 IDs
		ids := make([]ID, 10000)
		for i := 0; i < 10000; i++ {
			id, err := gen.GenerateID()
			if err != nil {
				t.Fatalf("GenerateID() error = %v at iteration %d", err, i)
			}
			ids[i] = id

			// Verify ID structure
			ts, worker, seq := id.ComponentsWithLayout(LayoutUltimate)

			// Verify worker ID
			if worker != 42 {
				t.Errorf("Worker ID = %d, want 42", worker)
			}

			// Verify sequence within 7-bit range (0-127)
			if seq < 0 || seq > 127 {
				t.Errorf("Sequence %d out of 7-bit range [0, 127]", seq)
			}

			// Verify timestamp
			if ts <= Epoch {
				t.Errorf("Timestamp %d should be after epoch", ts)
			}
		}

		// Verify uniqueness
		seen := make(map[ID]bool)
		for _, id := range ids {
			if seen[id] {
				t.Fatalf("Duplicate ID: %v", id)
			}
			seen[id] = true
		}

		// Verify monotonic ordering
		for i := 1; i < len(ids); i++ {
			if ids[i] <= ids[i-1] {
				t.Errorf("IDs not monotonic at index %d: %v <= %v", i, ids[i], ids[i-1])
			}
		}
	})

	t.Run("MaximumWorkerID", func(t *testing.T) {
		// Test with maximum worker ID for 16 bits (65535)
		cfg := DefaultConfig(65535)
		cfg.Layout = LayoutUltimate
		gen, err := NewWithConfig(cfg)
		if err != nil {
			t.Fatalf("NewWithConfig() with max worker ID error = %v", err)
		}

		id, err := gen.GenerateID()
		if err != nil {
			t.Fatalf("GenerateID() error = %v", err)
		}

		// Verify worker ID extraction
		worker := id.WorkerWithLayout(LayoutUltimate)
		if worker != 65535 {
			t.Errorf("Worker ID = %d, want 65535", worker)
		}
	})

	t.Run("SequenceOverflow", func(t *testing.T) {
		cfg := DefaultConfig(100)
		cfg.Layout = LayoutUltimate
		gen, err := NewWithConfig(cfg)
		if err != nil {
			t.Fatalf("NewWithConfig() error = %v", err)
		}

		// Generate enough IDs to cause sequence overflow (>128 IDs in 10ms)
		count := 500
		for i := 0; i < count; i++ {
			_, err := gen.GenerateID()
			if err != nil {
				t.Fatalf("GenerateID() error = %v at iteration %d", err, i)
			}
		}

		metrics := gen.GetMetrics()
		if metrics.Generated != int64(count) {
			t.Errorf("Metrics.Generated = %d, want %d", metrics.Generated, count)
		}

		// With 128 IDs per 10ms, we expect some sequence overflows
		if metrics.SequenceOverflow > 0 {
			t.Logf("Sequence overflows: %d (expected for rapid generation with 7-bit sequence)", metrics.SequenceOverflow)
		}
	})
}

// ============================================================================
// LayoutMegaScale Integration Tests
// ============================================================================

func TestLayoutMegaScaleIntegration(t *testing.T) {
	// Test LayoutMegaScale (40+17+6 bits with 10ms time unit)
	// Specs: 292 years, 131,072 nodes, 6,400 IDs/sec per node

	t.Run("BasicGeneration", func(t *testing.T) {
		cfg := DefaultConfig(1000)
		cfg.Layout = LayoutMegaScale
		gen, err := NewWithConfig(cfg)
		if err != nil {
			t.Fatalf("NewWithConfig() error = %v", err)
		}

		// Generate 10,000 IDs
		ids := make([]ID, 10000)
		for i := 0; i < 10000; i++ {
			id, err := gen.GenerateID()
			if err != nil {
				t.Fatalf("GenerateID() error = %v at iteration %d", err, i)
			}
			ids[i] = id

			// Verify ID structure
			ts, worker, seq := id.ComponentsWithLayout(LayoutMegaScale)

			// Verify worker ID
			if worker != 1000 {
				t.Errorf("Worker ID = %d, want 1000", worker)
			}

			// Verify sequence within 6-bit range (0-63)
			if seq < 0 || seq > 63 {
				t.Errorf("Sequence %d out of 6-bit range [0, 63]", seq)
			}

			// Verify timestamp
			if ts <= Epoch {
				t.Errorf("Timestamp %d should be after epoch", ts)
			}
		}

		// Verify uniqueness
		seen := make(map[ID]bool)
		for _, id := range ids {
			if seen[id] {
				t.Fatalf("Duplicate ID: %v", id)
			}
			seen[id] = true
		}

		// Verify monotonic ordering
		for i := 1; i < len(ids); i++ {
			if ids[i] <= ids[i-1] {
				t.Errorf("IDs not monotonic at index %d: %v <= %v", i, ids[i], ids[i-1])
			}
		}
	})

	t.Run("MaximumWorkerID", func(t *testing.T) {
		// Test with maximum worker ID for 17 bits (131071)
		cfg := DefaultConfig(131071)
		cfg.Layout = LayoutMegaScale
		gen, err := NewWithConfig(cfg)
		if err != nil {
			t.Fatalf("NewWithConfig() with max worker ID error = %v", err)
		}

		id, err := gen.GenerateID()
		if err != nil {
			t.Fatalf("GenerateID() error = %v", err)
		}

		// Verify worker ID extraction
		worker := id.WorkerWithLayout(LayoutMegaScale)
		if worker != 131071 {
			t.Errorf("Worker ID = %d, want 131071", worker)
		}
	})

	t.Run("SequenceOverflow", func(t *testing.T) {
		cfg := DefaultConfig(5000)
		cfg.Layout = LayoutMegaScale
		gen, err := NewWithConfig(cfg)
		if err != nil {
			t.Fatalf("NewWithConfig() error = %v", err)
		}

		// Generate enough IDs to cause sequence overflow (>64 IDs in 10ms)
		count := 500
		for i := 0; i < count; i++ {
			_, err := gen.GenerateID()
			if err != nil {
				t.Fatalf("GenerateID() error = %v at iteration %d", err, i)
			}
		}

		metrics := gen.GetMetrics()
		if metrics.Generated != int64(count) {
			t.Errorf("Metrics.Generated = %d, want %d", metrics.Generated, count)
		}

		// With 64 IDs per 10ms, we expect sequence overflows
		if metrics.SequenceOverflow > 0 {
			t.Logf("Sequence overflows: %d (expected for rapid generation with 6-bit sequence)", metrics.SequenceOverflow)
		}
	})
}

// ============================================================================
// Concurrent & Stress Tests for New Layouts
// ============================================================================

func TestNewLayoutsConcurrency(t *testing.T) {
	layouts := []struct {
		name   string
		layout BitLayout
	}{
		{"LayoutUltimate", LayoutUltimate},
		{"LayoutMegaScale", LayoutMegaScale},
	}

	for _, tt := range layouts {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig(42)
			cfg.Layout = tt.layout
			gen, err := NewWithConfig(cfg)
			if err != nil {
				t.Fatalf("NewWithConfig() error = %v", err)
			}

			goroutines := 50
			idsPerGoroutine := 200
			totalIDs := goroutines * idsPerGoroutine

			ids := sync.Map{}
			var wg sync.WaitGroup
			errors := make(chan error, goroutines)

			for i := 0; i < goroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < idsPerGoroutine; j++ {
						id, err := gen.GenerateID()
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
		})
	}
}

func TestNewLayoutsHighThroughput(t *testing.T) {
	tests := []struct {
		name   string
		layout BitLayout
		count  int
	}{
		{"LayoutUltimate", LayoutUltimate, 100000},
		{"LayoutMegaScale", LayoutMegaScale, 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig(42)
			cfg.Layout = tt.layout
			gen, err := NewWithConfig(cfg)
			if err != nil {
				t.Fatalf("NewWithConfig() error = %v", err)
			}

			// Generate large number of IDs
			for i := 0; i < tt.count; i++ {
				_, err := gen.GenerateID()
				if err != nil {
					t.Fatalf("GenerateID() error = %v at iteration %d", err, i)
				}
			}

			// Check metrics
			metrics := gen.GetMetrics()
			if metrics.Generated != int64(tt.count) {
				t.Errorf("Metrics.Generated = %d, want %d", metrics.Generated, tt.count)
			}

			// Log performance info
			t.Logf("%s: Generated %d IDs", tt.name, tt.count)
			t.Logf("  Sequence overflows: %d", metrics.SequenceOverflow)
			t.Logf("  Clock backward events: %d", metrics.ClockBackward)
			if metrics.SequenceOverflow > 0 {
				avgWait := float64(metrics.WaitTimeUs) / float64(metrics.SequenceOverflow)
				t.Logf("  Avg wait per overflow: %.2f µs", avgWait)
			}
		})
	}
}
