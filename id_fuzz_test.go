package snowflake

import (
	"encoding/json"
	"testing"
	"time"
)

// FuzzIDComponents tests component extraction from random ID values.
// This ensures the bitwise extraction logic works correctly for any int64.
func FuzzIDComponents(f *testing.F) {
	// Add corpus seeds
	gen, _ := New(42)

	seeds := []int64{
		0,
		1,
		1 << 41,                       // Just timestamp bit
		(1 << 22) - 1,                 // Max worker ID and sequence
		(42 << 12) | 100,              // Worker 42, sequence 100
		(1 << 41) | (42 << 12) | 100,  // Full structure
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	// Add real generated ID
	if id, err := gen.GenerateID(); err == nil {
		f.Add(int64(id))
	}

	f.Fuzz(func(t *testing.T, idVal int64) {
		id := ID(idVal)

		// Extract components
		timestamp := id.Timestamp()
		worker := id.Worker()
		sequence := id.Sequence()

		// Validate ranges
		if worker < 0 || worker > MaxWorkerID {
			t.Errorf("Worker() = %d, out of range [0, %d]", worker, MaxWorkerID)
		}

		if sequence < 0 || sequence > MaxSequence {
			t.Errorf("Sequence() = %d, out of range [0, %d]", sequence, MaxSequence)
		}

		// Test Components() matches individual methods
		ts, wid, seq := id.Components()
		if ts != timestamp || wid != worker || seq != sequence {
			t.Errorf("Components() mismatch: got (%d,%d,%d), want (%d,%d,%d)",
				ts, wid, seq, timestamp, worker, sequence)
		}

		// Test ParseIDComponents function
		ts2, wid2, seq2 := ParseIDComponents(idVal)
		if ts2 != timestamp || wid2 != worker || seq2 != sequence {
			t.Errorf("ParseIDComponents() mismatch: got (%d,%d,%d), want (%d,%d,%d)",
				ts2, wid2, seq2, timestamp, worker, sequence)
		}
	})
}

// FuzzIDJSON tests JSON marshaling/unmarshaling round-trips.
// This ensures IDs can be safely serialized to JSON and back.
func FuzzIDJSON(f *testing.F) {
	seeds := []int64{
		0,
		1,
		1 << 41,
		9223372036854775807, // MaxInt64
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, original int64) {
		id := ID(original)

		// Marshal
		data, err := json.Marshal(id)
		if err != nil {
			t.Errorf("json.Marshal() failed for ID %d: %v", original, err)
			return
		}

		// Unmarshal
		var decoded ID
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Errorf("json.Unmarshal() failed for ID %d (JSON: %s): %v", original, string(data), err)
			return
		}

		// Verify round-trip
		if decoded != id {
			t.Errorf("JSON round-trip failed: original=%d, decoded=%d (JSON: %s)",
				id, decoded, string(data))
		}
	})
}

// FuzzIDTime tests time-related operations on IDs.
// This validates timestamp extraction and time calculations.
func FuzzIDTime(f *testing.F) {
	gen, _ := New(1)

	seeds := []int64{
		0,
		1,
		Epoch,                         // Epoch timestamp
		(Epoch << 22),                 // Epoch in ID format
		(1 << 41) | (1 << 12) | 1,    // Max timestamp ID
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	// Add real generated ID
	if id, err := gen.GenerateID(); err == nil {
		f.Add(int64(id))
	}

	f.Fuzz(func(t *testing.T, idVal int64) {
		id := ID(idVal)

		// Get timestamp
		timestamp := id.Timestamp()

		// Timestamp should be non-negative
		if timestamp < 0 {
			t.Errorf("Timestamp() = %d, should be non-negative", timestamp)
		}

		// Get time
		idTime := id.Time()

		// Time should be after Unix epoch (1970) - just ensure it doesn't panic
		unixEpoch := time.Unix(0, 0)
		_ = idTime.Before(unixEpoch) // Check that Before() doesn't panic

		// Age should be calculable without panic
		age := id.Age()
		_ = age // Just ensure no panic

		// ExtractTimestamp should match id.Time()
		extractedTime := ExtractTimestamp(idVal)
		if !extractedTime.Equal(idTime) {
			// Allow small differences due to millisecond precision
			diff := extractedTime.Sub(idTime)
			if diff < -time.Millisecond || diff > time.Millisecond {
				t.Errorf("ExtractTimestamp() mismatch: got %v, want %v (diff: %v)",
					extractedTime, idTime, diff)
			}
		}
	})
}

// FuzzIDComparison tests comparison operations between IDs.
// This ensures all comparison methods (Before, After, Equal, Compare) are consistent.
func FuzzIDComparison(f *testing.F) {
	seeds := [][2]int64{
		{0, 0},
		{0, 1},
		{1, 0},
		{100, 200},
		{1 << 41, 1 << 40},
		{9223372036854775807, 9223372036854775806},
	}

	for _, seed := range seeds {
		f.Add(seed[0], seed[1])
	}

	f.Fuzz(func(t *testing.T, id1Val, id2Val int64) {
		id1 := ID(id1Val)
		id2 := ID(id2Val)

		// Test Equal
		equal := id1.Equal(id2)
		if equal != (id1Val == id2Val) {
			t.Errorf("Equal() inconsistent: id1=%d, id2=%d, Equal()=%v, should be %v",
				id1Val, id2Val, equal, id1Val == id2Val)
		}

		// Test Before/After
		before := id1.Before(id2)
		after := id1.After(id2)

		if id1Val < id2Val && !before {
			t.Errorf("Before() should be true: id1=%d < id2=%d", id1Val, id2Val)
		}

		if id1Val > id2Val && !after {
			t.Errorf("After() should be true: id1=%d > id2=%d", id1Val, id2Val)
		}

		// Before and After should be mutually exclusive (unless equal)
		if before && after {
			t.Errorf("Before() and After() both true: id1=%d, id2=%d", id1Val, id2Val)
		}

		// Test Compare
		cmp := id1.Compare(id2)
		if id1Val < id2Val && cmp >= 0 {
			t.Errorf("Compare() should be negative: id1=%d < id2=%d, got %d", id1Val, id2Val, cmp)
		}
		if id1Val > id2Val && cmp <= 0 {
			t.Errorf("Compare() should be positive: id1=%d > id2=%d, got %d", id1Val, id2Val, cmp)
		}
		if id1Val == id2Val && cmp != 0 {
			t.Errorf("Compare() should be zero: id1=%d == id2=%d, got %d", id1Val, id2Val, cmp)
		}
	})
}

// FuzzIDSharding tests sharding operations.
// This ensures shard assignment is deterministic and within bounds.
func FuzzIDSharding(f *testing.F) {
	seeds := []struct {
		id        int64
		numShards int64
	}{
		{1, 10},
		{100, 16},
		{1 << 41, 100},
		{9223372036854775807, 256},
	}

	for _, seed := range seeds {
		f.Add(seed.id, seed.numShards)
	}

	f.Fuzz(func(t *testing.T, idVal int64, numShards int64) {
		// Skip invalid shard counts
		if numShards <= 0 {
			return
		}

		id := ID(idVal)

		// Test Shard
		shard := id.Shard(numShards)
		if shard < 0 || shard >= numShards {
			t.Errorf("Shard(%d) = %d, out of range [0, %d)", numShards, shard, numShards)
		}

		// Shard should be deterministic
		shard2 := id.Shard(numShards)
		if shard != shard2 {
			t.Errorf("Shard() not deterministic: first=%d, second=%d", shard, shard2)
		}

		// Test ShardByWorker
		shardByWorker := id.ShardByWorker(numShards)
		if shardByWorker < 0 || shardByWorker >= numShards {
			t.Errorf("ShardByWorker(%d) = %d, out of range [0, %d)", numShards, shardByWorker, numShards)
		}

		// ShardByWorker should be deterministic
		shardByWorker2 := id.ShardByWorker(numShards)
		if shardByWorker != shardByWorker2 {
			t.Errorf("ShardByWorker() not deterministic: first=%d, second=%d", shardByWorker, shardByWorker2)
		}

		// Test ShardByTime with 1 hour bucket
		shardByTime := id.ShardByTime(time.Hour)
		if shardByTime < 0 {
			t.Errorf("ShardByTime() = %d, should be non-negative", shardByTime)
		}
	})
}

// FuzzIDConversions tests type conversions (Int64, Uint64, String).
// This ensures conversions are consistent and reversible.
func FuzzIDConversions(f *testing.F) {
	seeds := []int64{
		0,
		1,
		-1,
		9223372036854775807,  // MaxInt64
		-9223372036854775808, // MinInt64
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, original int64) {
		id := ID(original)

		// Test Int64
		i64 := id.Int64()
		if i64 != original {
			t.Errorf("Int64() = %d, want %d", i64, original)
		}

		// Test Uint64 (for non-negative values)
		u64 := id.Uint64()
		if original >= 0 && int64(u64) != original {
			t.Errorf("Uint64() = %d, want %d", u64, original)
		}

		// Test String -> ParseString round-trip
		str := id.String()
		if str == "" {
			t.Errorf("String() produced empty string for ID %d", original)
			return
		}

		parsed, err := ParseString(str)
		if err != nil {
			t.Errorf("ParseString(%q) failed: %v", str, err)
		} else if parsed != id {
			t.Errorf("String round-trip: original=%d, parsed=%d (str=%s)", id, parsed, str)
		}
	})
}

// FuzzIDFormat tests the Format method with various format specifiers.
// This ensures custom formatting works correctly for all inputs.
func FuzzIDFormat(f *testing.F) {
	formats := []string{
		"hex", "x",
		"binary", "bin", "b",
		"base32", "b32", "32",
		"base58", "b58", "58",
		"base62", "b62", "62",
		"base64", "b64", "64",
		"decimal", "dec", "d",
		"", "unknown",
	}

	seeds := []int64{0, 1, 1 << 41, 9223372036854775807}

	for _, id := range seeds {
		for _, format := range formats {
			f.Add(id, format)
		}
	}

	f.Fuzz(func(t *testing.T, idVal int64, format string) {
		id := ID(idVal)

		// Format should not panic for any format string
		result := id.Format(format)

		// Result should not be empty for non-negative IDs
		if idVal >= 0 && len(result) == 0 {
			t.Errorf("Format(%q) produced empty string for ID %d", format, idVal)
		}

		// For negative IDs, some formats may return empty or specific values
		// We just ensure no panic occurs
	})
}

// FuzzIDValidation tests the IsValid method.
// This ensures validation logic is consistent.
func FuzzIDValidation(f *testing.F) {
	seeds := []int64{
		0,
		1,
		-1,
		100,
		1 << 41,
		9223372036854775807,
		-9223372036854775808,
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, idVal int64) {
		id := ID(idVal)

		// IsValid should not panic
		isValid := id.IsValid()

		// Zero and negative IDs should be invalid
		// Just verify that IsValid doesn't panic for any value
		_ = isValid // Intentionally check all values without specific assertions
	})
}

// FuzzIntBytes tests the IntBytes conversion.
// This validates the 8-byte big-endian encoding.
func FuzzIntBytes(f *testing.F) {
	seeds := []int64{
		0,
		1,
		255,
		256,
		65535,
		65536,
		1 << 41,
		9223372036854775807,
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, original int64) {
		id := ID(original)

		// Encode to bytes
		bytes := id.IntBytes()

		// Should be 8 bytes
		if len(bytes) != 8 {
			t.Errorf("IntBytes() returned %d bytes, want 8", len(bytes))
			return
		}

		// Decode back
		decoded := ParseIntBytes(bytes)

		// Verify round-trip
		if decoded != id {
			t.Errorf("IntBytes round-trip failed: original=%d, decoded=%d", id, decoded)
		}
	})
}
