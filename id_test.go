package snowflake

import (
	"encoding/json"
	"testing"
	"time"
)

// TestIDEncodings tests all encoding formats
func TestIDEncodings(t *testing.T) {
	gen, err := New(42)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	id, err := gen.GenerateID()
	if err != nil {
		t.Fatalf("GenerateID() error = %v", err)
	}

	tests := []struct {
		name   string
		encode func(ID) string
		decode func(string) (ID, error)
	}{
		{"String", ID.String, ParseString},
		{"Base2", ID.Base2, ParseBase2},
		{"Base32", ID.Base32, ParseBase32},
		{"Base36", ID.Base36, ParseBase36},
		{"Base58", ID.Base58, ParseBase58},
		{"Base62", ID.Base62, ParseBase62},
		{"Hex", ID.Hex, ParseHex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := tt.encode(id)
			decoded, err := tt.decode(encoded)
			if err != nil {
				t.Fatalf("%s decode error = %v", tt.name, err)
			}
			if decoded != id {
				t.Errorf("%s: decoded = %d, want %d (encoded: %s)",
					tt.name, decoded, id, encoded)
			}
		})
	}
}

// TestIDBase64 tests Base64 encoding/decoding
func TestIDBase64(t *testing.T) {
	gen, _ := New(1)
	id, _ := gen.GenerateID()

	// Standard Base64
	b64 := id.Base64()
	decoded, err := ParseBase64(b64)
	if err != nil {
		t.Fatalf("ParseBase64() error = %v", err)
	}
	if decoded != id {
		t.Errorf("Base64: decoded = %d, want %d", decoded, id)
	}

	// URL-safe Base64
	b64url := id.Base64URL()
	decoded, err = ParseBase64URL(b64url)
	if err != nil {
		t.Fatalf("ParseBase64URL() error = %v", err)
	}
	if decoded != id {
		t.Errorf("Base64URL: decoded = %d, want %d", decoded, id)
	}
}

// TestIDJSON tests JSON marshaling/unmarshaling
func TestIDJSON(t *testing.T) {
	gen, _ := New(1)
	id, _ := gen.GenerateID()

	// Marshal
	data, err := json.Marshal(id)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal
	var decoded ID
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded != id {
		t.Errorf("JSON: decoded = %d, want %d", decoded, id)
	}

	// Test with struct
	type TestStruct struct {
		ID   ID     `json:"id"`
		Name string `json:"name"`
	}

	original := TestStruct{ID: id, Name: "test"}
	data, err = json.Marshal(original)
	if err != nil {
		t.Fatalf("struct marshal error = %v", err)
	}

	var result TestStruct
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("struct unmarshal error = %v", err)
	}

	if result.ID != original.ID {
		t.Errorf("struct ID: got = %d, want %d", result.ID, original.ID)
	}
}

// TestIDBinary tests binary encoding/decoding
func TestIDBinary(t *testing.T) {
	gen, _ := New(1)
	id, _ := gen.GenerateID()

	// IntBytes
	bytes := id.IntBytes()
	decoded := ParseIntBytes(bytes)
	if decoded != id {
		t.Errorf("IntBytes: decoded = %d, want %d", decoded, id)
	}

	// MarshalBinary
	binData, err := id.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary() error = %v", err)
	}

	var decoded2 ID
	err = decoded2.UnmarshalBinary(binData)
	if err != nil {
		t.Fatalf("UnmarshalBinary() error = %v", err)
	}

	if decoded2 != id {
		t.Errorf("Binary: decoded = %d, want %d", decoded2, id)
	}
}

// TestIDComponents tests component extraction
func TestIDComponents(t *testing.T) {
	gen, _ := New(42)
	id, _ := gen.GenerateID()

	// Test Time()
	idTime := id.Time()
	if idTime.After(time.Now()) {
		t.Errorf("ID.Time() is in the future: %v", idTime)
	}
	if idTime.Before(time.Unix(Epoch/1000, 0)) {
		t.Errorf("ID.Time() is before epoch: %v", idTime)
	}

	// Test Timestamp()
	ts := id.Timestamp()
	if ts < Epoch {
		t.Errorf("ID.Timestamp() = %d, should be >= epoch %d", ts, Epoch)
	}

	// Test Worker()
	worker := id.Worker()
	if worker != 42 {
		t.Errorf("ID.Worker() = %d, want 42", worker)
	}

	// Test Sequence()
	seq := id.Sequence()
	if seq < 0 || seq > MaxSequence {
		t.Errorf("ID.Sequence() = %d, out of range [0, %d]", seq, MaxSequence)
	}

	// Test Components()
	timestamp, workerID, sequence := id.Components()
	if workerID != 42 {
		t.Errorf("Components() workerID = %d, want 42", workerID)
	}
	if timestamp != ts {
		t.Errorf("Components() timestamp = %d, want %d", timestamp, ts)
	}
	if sequence != seq {
		t.Errorf("Components() sequence = %d, want %d", sequence, seq)
	}
}

// TestIDValidation tests ID validation
func TestIDValidation(t *testing.T) {
	gen, _ := New(1)
	id, _ := gen.GenerateID()

	if !id.IsValid() {
		t.Error("Valid ID reported as invalid")
	}

	// Test invalid IDs
	invalidIDs := []ID{
		0,   // Zero ID
		-1,  // Negative
		100, // Too small (not a properly structured Snowflake ID)
	}

	for _, invalid := range invalidIDs {
		if invalid.IsValid() {
			t.Errorf("Invalid ID %d reported as valid", invalid)
		}
	}
}

// TestIDComparison tests ID comparison methods
func TestIDComparison(t *testing.T) {
	gen, _ := New(1)
	id1, _ := gen.GenerateID()
	time.Sleep(1 * time.Millisecond) // Ensure different timestamp
	id2, _ := gen.GenerateID()

	if !id1.Before(id2) {
		t.Error("id1 should be before id2")
	}

	if !id2.After(id1) {
		t.Error("id2 should be after id1")
	}

	if !id1.Equal(id1) {
		t.Error("id1 should equal itself")
	}

	if id1.Compare(id2) >= 0 {
		t.Error("id1.Compare(id2) should be negative")
	}

	if id2.Compare(id1) <= 0 {
		t.Error("id2.Compare(id1) should be positive")
	}

	if id1.Compare(id1) != 0 {
		t.Error("id1.Compare(id1) should be zero")
	}
}

// TestIDAge tests Age method
func TestIDAge(t *testing.T) {
	gen, _ := New(1)
	id, _ := gen.GenerateID()

	age := id.Age()
	if age < 0 {
		t.Errorf("ID.Age() = %v, should be >= 0", age)
	}

	if age > time.Second {
		t.Errorf("ID.Age() = %v, should be < 1 second", age)
	}
}

// TestIDSharding tests sharding methods
func TestIDSharding(t *testing.T) {
	gen, _ := New(42)
	id, _ := gen.GenerateID()

	// Test Shard
	numShards := int64(10)
	shard := id.Shard(numShards)
	if shard < 0 || shard >= numShards {
		t.Errorf("ID.Shard(%d) = %d, out of range", numShards, shard)
	}

	// Test ShardByWorker
	shardByWorker := id.ShardByWorker(numShards)
	expectedShard := int64(42) % numShards
	if shardByWorker != expectedShard {
		t.Errorf("ID.ShardByWorker(%d) = %d, want %d", numShards, shardByWorker, expectedShard)
	}

	// Test ShardByTime
	bucketSize := 1 * time.Hour
	shardByTime := id.ShardByTime(bucketSize)
	if shardByTime < 0 {
		t.Errorf("ID.ShardByTime() = %d, should be >= 0", shardByTime)
	}
}

// TestIDFormat tests custom formatting
func TestIDFormat(t *testing.T) {
	gen, _ := New(1)
	id, _ := gen.GenerateID()

	tests := []struct {
		format   string
		expected string
	}{
		{"hex", id.Hex()},
		{"x", id.Hex()},
		{"binary", id.Base2()},
		{"bin", id.Base2()},
		{"b", id.Base2()},
		{"base32", id.Base32()},
		{"b32", id.Base32()},
		{"32", id.Base32()},
		{"base58", id.Base58()},
		{"b58", id.Base58()},
		{"58", id.Base58()},
		{"base62", id.Base62()},
		{"b62", id.Base62()},
		{"62", id.Base62()},
		{"base64", id.Base64()},
		{"b64", id.Base64()},
		{"64", id.Base64()},
		{"decimal", id.String()},
		{"dec", id.String()},
		{"d", id.String()},
		{"", id.String()},
		{"unknown", id.String()}, // Default to string
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			result := id.Format(tt.format)
			if result != tt.expected {
				t.Errorf("Format(%q) = %q, want %q", tt.format, result, tt.expected)
			}
		})
	}
}

// TestIDConversions tests basic type conversions
func TestIDConversions(t *testing.T) {
	gen, _ := New(1)
	id, _ := gen.GenerateID()

	// Int64
	i64 := id.Int64()
	if ID(i64) != id {
		t.Errorf("Int64() round-trip failed")
	}

	// Uint64
	u64 := id.Uint64()
	if ID(u64) != id {
		t.Errorf("Uint64() round-trip failed")
	}

	// String
	str := id.String()
	parsed, err := ParseString(str)
	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}
	if parsed != id {
		t.Errorf("String() round-trip failed")
	}
}

// TestInvalidEncodings tests parsing invalid encoded strings
func TestInvalidEncodings(t *testing.T) {
	tests := []struct {
		name   string
		parser func(string) (ID, error)
		input  string
	}{
		{"Base32 invalid char", ParseBase32, "!!!"},
		{"Base58 invalid char", ParseBase58, "0OIl"},
		{"Base62 invalid char", ParseBase62, "!!!"},
		{"Hex invalid char", ParseHex, "zzz"},
		{"Base64 invalid", ParseBase64, "!!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.parser(tt.input)
			if err == nil {
				t.Errorf("%s should return error for invalid input", tt.name)
			}
		})
	}
}

// BenchmarkIDEncodings benchmarks various encoding methods
func BenchmarkIDEncodings(b *testing.B) {
	gen, _ := New(1)
	id, _ := gen.GenerateID()

	b.Run("String", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = id.String()
		}
	})

	b.Run("Base32", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = id.Base32()
		}
	})

	b.Run("Base58", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = id.Base58()
		}
	})

	b.Run("Base62", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = id.Base62()
		}
	})

	b.Run("Hex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = id.Hex()
		}
	})
}

// BenchmarkIDParsing benchmarks parsing methods
func BenchmarkIDParsing(b *testing.B) {
	gen, _ := New(1)
	id, _ := gen.GenerateID()

	b.Run("ParseString", func(b *testing.B) {
		str := id.String()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = ParseString(str)
		}
	})

	b.Run("ParseBase32", func(b *testing.B) {
		str := id.Base32()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = ParseBase32(str)
		}
	})

	b.Run("ParseBase58", func(b *testing.B) {
		str := id.Base58()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = ParseBase58(str)
		}
	})
}
