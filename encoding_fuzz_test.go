package snowflake

import (
	"testing"
)

// FuzzBase32RoundTrip tests Base32 encoding/decoding round-trip with random int64 values.
// This ensures that any valid int64 can be encoded and decoded without data loss.
func FuzzBase32RoundTrip(f *testing.F) {
	// Add corpus seeds for better coverage
	seeds := []int64{
		0,                   // Zero
		1,                   // Minimum positive
		31,                  // Max single digit in base32
		32,                  // Two digits in base32
		1<<41 - 1,           // Max timestamp (41 bits)
		1 << 41,             // Just over max timestamp
		1 << 62,             // Large value
		9223372036854775807, // MaxInt64
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, original int64) {
		// Skip negative values (encoding functions expect non-negative int64)
		if original < 0 {
			return
		}

		// Encode
		encoded := encodeBase32(original)

		// Decode
		decoded, err := decodeBase32(encoded)
		if err != nil {
			t.Errorf("decodeBase32() failed for %d (encoded: %s): %v", original, encoded, err)
			return
		}

		// Verify round-trip
		if decoded != original {
			t.Errorf("Base32 round-trip failed: original=%d, decoded=%d (encoded: %s)",
				original, decoded, encoded)
		}

		// Verify encoded string is not empty
		if len(encoded) == 0 {
			t.Errorf("encodeBase32(%d) produced empty string", original)
		}
	})
}

// FuzzBase58RoundTrip tests Base58 encoding/decoding round-trip.
// Base58 uses Bitcoin-style alphabet without confusing characters (0, O, I, l).
func FuzzBase58RoundTrip(f *testing.F) {
	seeds := []int64{
		0,
		1,
		57,                  // Max single digit in base58
		58,                  // Two digits in base58
		1<<41 - 1,           // Max timestamp
		9223372036854775807, // MaxInt64
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, original int64) {
		// Skip negative values (encoding functions expect non-negative int64)
		if original < 0 {
			return
		}

		encoded := encodeBase58(original)
		decoded, err := decodeBase58(encoded)
		if err != nil {
			t.Errorf("decodeBase58() failed for %d (encoded: %s): %v", original, encoded, err)
			return
		}

		if decoded != original {
			t.Errorf("Base58 round-trip failed: original=%d, decoded=%d (encoded: %s)",
				original, decoded, encoded)
		}

		if len(encoded) == 0 {
			t.Errorf("encodeBase58(%d) produced empty string", original)
		}
	})
}

// FuzzBase62RoundTrip tests Base62 encoding/decoding round-trip.
// Base62 uses all alphanumeric characters (0-9, a-z, A-Z) and is URL-safe.
func FuzzBase62RoundTrip(f *testing.F) {
	seeds := []int64{
		0,
		1,
		61,                  // Max single digit in base62
		62,                  // Two digits in base62
		1<<41 - 1,           // Max timestamp
		9223372036854775807, // MaxInt64
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, original int64) {
		// Skip negative values (encoding functions expect non-negative int64)
		if original < 0 {
			return
		}

		encoded := encodeBase62(original)
		decoded, err := decodeBase62(encoded)
		if err != nil {
			t.Errorf("decodeBase62() failed for %d (encoded: %s): %v", original, encoded, err)
			return
		}

		if decoded != original {
			t.Errorf("Base62 round-trip failed: original=%d, decoded=%d (encoded: %s)",
				original, decoded, encoded)
		}

		if len(encoded) == 0 {
			t.Errorf("encodeBase62(%d) produced empty string", original)
		}
	})
}

// FuzzHexRoundTrip tests hexadecimal encoding/decoding round-trip.
// Hex uses 4 bits per character and is optimized with bitshifting.
func FuzzHexRoundTrip(f *testing.F) {
	seeds := []int64{
		0,
		1,
		15,                  // Max single digit in hex (F)
		16,                  // Two digits in hex (10)
		255,                 // FF
		256,                 // 100
		1<<41 - 1,           // Max timestamp
		9223372036854775807, // MaxInt64
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, original int64) {
		// Skip negative values for hex encoding (not supported in current implementation)
		if original < 0 {
			return
		}

		encoded := encodeHex(original)
		decoded, err := decodeHex(encoded)
		if err != nil {
			t.Errorf("decodeHex() failed for %d (encoded: %s): %v", original, encoded, err)
			return
		}

		if decoded != original {
			t.Errorf("Hex round-trip failed: original=%d, decoded=%d (encoded: %s)",
				original, decoded, encoded)
		}

		if len(encoded) == 0 {
			t.Errorf("encodeHex(%d) produced empty string", original)
		}
	})
}

// FuzzIDEncodingRoundTrip tests ID type encoding/decoding round-trips for all formats.
// This validates the ID type's encoding methods work correctly with fuzz-generated values.
func FuzzIDEncodingRoundTrip(f *testing.F) {
	// Generate some realistic Snowflake IDs as seeds
	gen, _ := New(42)

	// Add corpus seeds
	seeds := []int64{
		1,
		1 << 41,                       // Large timestamp value
		(1 << 41) | (42 << 12) | 100,  // Full snowflake structure
		9223372036854775807,           // MaxInt64
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	// Try to generate a real ID as a seed
	if id, err := gen.GenerateID(); err == nil {
		f.Add(int64(id))
	}

	f.Fuzz(func(t *testing.T, original int64) {
		id := ID(original)

		// Test Base32
		base32 := id.Base32()
		decoded32, err := ParseBase32(base32)
		if err != nil {
			t.Errorf("ParseBase32() failed for ID %d: %v", original, err)
		} else if decoded32 != id {
			t.Errorf("Base32: original=%d, decoded=%d", id, decoded32)
		}

		// Test Base58
		base58 := id.Base58()
		decoded58, err := ParseBase58(base58)
		if err != nil {
			t.Errorf("ParseBase58() failed for ID %d: %v", original, err)
		} else if decoded58 != id {
			t.Errorf("Base58: original=%d, decoded=%d", id, decoded58)
		}

		// Test Base62
		base62 := id.Base62()
		decoded62, err := ParseBase62(base62)
		if err != nil {
			t.Errorf("ParseBase62() failed for ID %d: %v", original, err)
		} else if decoded62 != id {
			t.Errorf("Base62: original=%d, decoded=%d", id, decoded62)
		}

		// Test Hex (only for non-negative values)
		if original >= 0 {
			hex := id.Hex()
			decodedHex, err := ParseHex(hex)
			if err != nil {
				t.Errorf("ParseHex() failed for ID %d: %v", original, err)
			} else if decodedHex != id {
				t.Errorf("Hex: original=%d, decoded=%d", id, decodedHex)
			}
		}

		// Test Base64
		base64 := id.Base64()
		decoded64, err := ParseBase64(base64)
		if err != nil {
			t.Errorf("ParseBase64() failed for ID %d: %v", original, err)
		} else if decoded64 != id {
			t.Errorf("Base64: original=%d, decoded=%d", id, decoded64)
		}

		// Test Base64URL
		base64url := id.Base64URL()
		decoded64url, err := ParseBase64URL(base64url)
		if err != nil {
			t.Errorf("ParseBase64URL() failed for ID %d: %v", original, err)
		} else if decoded64url != id {
			t.Errorf("Base64URL: original=%d, decoded=%d", id, decoded64url)
		}

		// Test Binary
		binData, err := id.MarshalBinary()
		if err != nil {
			t.Errorf("MarshalBinary() failed for ID %d: %v", original, err)
			return
		}

		var decodedBin ID
		err = decodedBin.UnmarshalBinary(binData)
		if err != nil {
			t.Errorf("UnmarshalBinary() failed for ID %d: %v", original, err)
		} else if decodedBin != id {
			t.Errorf("Binary: original=%d, decoded=%d", id, decodedBin)
		}
	})
}

// FuzzInvalidEncodings tests that decoders properly reject invalid input.
// This ensures robust error handling for malformed encoded strings.
func FuzzInvalidEncodings(f *testing.F) {
	// Add corpus seeds with potentially problematic strings
	seeds := []string{
		"",
		"!@#$%",
		"0OIl",     // Confusing characters
		"ZZZZZZ",
		"\x00\x01", // Binary characters
		"123456789012345678901234567890", // Very long
		"---",
		"   ",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Try each decoder - should either succeed or return error (not panic)
		_, _ = decodeBase32(input)
		_, _ = decodeBase58(input)
		_, _ = decodeBase62(input)
		_, _ = decodeHex(input)

		// Also test ID parsing functions
		_, _ = ParseBase32(input)
		_, _ = ParseBase58(input)
		_, _ = ParseBase62(input)
		_, _ = ParseHex(input)
		_, _ = ParseBase64(input)
		_, _ = ParseBase64URL(input)
		_, _ = ParseString(input)
		_, _ = ParseBase2(input)
		_, _ = ParseBase36(input)

		// If we get here without panic, the fuzzer found no crashes
	})
}
