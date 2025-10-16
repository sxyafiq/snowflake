package snowflake

import (
	"errors"
	"testing"
	"time"
)

// ============================================================================
// BitLayout.Validate() Tests
// ============================================================================

func TestBitLayout_Validate_ValidLayouts(t *testing.T) {
	tests := []struct {
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
		{
			"Custom 38+17+8",
			BitLayout{
				TimestampBits: 38,
				WorkerBits:    17,
				SequenceBits:  8,
				TimeUnit:      time.Millisecond,
			},
		},
		{
			"Custom 42+10+11",
			BitLayout{
				TimestampBits: 42,
				WorkerBits:    10,
				SequenceBits:  11,
				TimeUnit:      time.Millisecond,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.layout.Validate(); err != nil {
				t.Errorf("Validate() should succeed for %s, got error: %v", tt.name, err)
			}
		})
	}
}

func TestBitLayout_Validate_InvalidSum(t *testing.T) {
	tests := []struct {
		name   string
		layout BitLayout
	}{
		{
			"Sum < 63",
			BitLayout{
				TimestampBits: 40,
				WorkerBits:    10,
				SequenceBits:  10, // Total: 60
				TimeUnit:      time.Millisecond,
			},
		},
		{
			"Sum > 63",
			BitLayout{
				TimestampBits: 42,
				WorkerBits:    12,
				SequenceBits:  12, // Total: 66
				TimeUnit:      time.Millisecond,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.layout.Validate()
			if err == nil {
				t.Errorf("Validate() should fail for %s", tt.name)
			}
			if !errors.Is(err, ErrInvalidBitLayout) {
				t.Errorf("Expected ErrInvalidBitLayout, got: %v", err)
			}
		})
	}
}

func TestBitLayout_Validate_NegativeValues(t *testing.T) {
	tests := []struct {
		name   string
		layout BitLayout
	}{
		{
			"Negative TimestampBits",
			BitLayout{
				TimestampBits: -1,
				WorkerBits:    10,
				SequenceBits:  12,
				TimeUnit:      time.Millisecond,
			},
		},
		{
			"Negative WorkerBits",
			BitLayout{
				TimestampBits: 41,
				WorkerBits:    -1,
				SequenceBits:  12,
				TimeUnit:      time.Millisecond,
			},
		},
		{
			"Negative SequenceBits",
			BitLayout{
				TimestampBits: 41,
				WorkerBits:    10,
				SequenceBits:  -1,
				TimeUnit:      time.Millisecond,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.layout.Validate()
			if err == nil {
				t.Errorf("Validate() should fail for %s", tt.name)
			}
			if !errors.Is(err, ErrInvalidBitLayout) {
				t.Errorf("Expected ErrInvalidBitLayout, got: %v", err)
			}
		})
	}
}

func TestBitLayout_Validate_OutOfRange(t *testing.T) {
	tests := []struct {
		name   string
		layout BitLayout
	}{
		{
			"TimestampBits too small",
			BitLayout{
				TimestampBits: 37, // Below min of 38
				WorkerBits:    13,
				SequenceBits:  13,
				TimeUnit:      time.Millisecond,
			},
		},
		{
			"TimestampBits too large",
			BitLayout{
				TimestampBits: 43, // Above max of 42
				WorkerBits:    10,
				SequenceBits:  10,
				TimeUnit:      time.Millisecond,
			},
		},
		{
			"WorkerBits too small",
			BitLayout{
				TimestampBits: 41,
				WorkerBits:    7, // Below min of 8
				SequenceBits:  15,
				TimeUnit:      time.Millisecond,
			},
		},
		{
			"WorkerBits too large",
			BitLayout{
				TimestampBits: 41,
				WorkerBits:    19, // Above max of 18
				SequenceBits:  3,
				TimeUnit:      time.Millisecond,
			},
		},
		{
			"SequenceBits too small",
			BitLayout{
				TimestampBits: 41,
				WorkerBits:    17,
				SequenceBits:  5, // Below min of 6
				TimeUnit:      time.Millisecond,
			},
		},
		{
			"SequenceBits too large",
			BitLayout{
				TimestampBits: 41,
				WorkerBits:    8,
				SequenceBits:  15, // Above max of 14 (sum would be 64)
				TimeUnit:      time.Millisecond,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.layout.Validate()
			if err == nil {
				t.Errorf("Validate() should fail for %s", tt.name)
			}
			if !errors.Is(err, ErrInvalidBitLayout) {
				t.Errorf("Expected ErrInvalidBitLayout, got: %v", err)
			}
		})
	}
}

func TestBitLayout_Validate_InvalidTimeUnit(t *testing.T) {
	tests := []struct {
		name   string
		layout BitLayout
	}{
		{
			"Zero TimeUnit",
			BitLayout{
				TimestampBits: 41,
				WorkerBits:    10,
				SequenceBits:  12,
				TimeUnit:      0,
			},
		},
		{
			"Negative TimeUnit",
			BitLayout{
				TimestampBits: 41,
				WorkerBits:    10,
				SequenceBits:  12,
				TimeUnit:      -1 * time.Millisecond,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.layout.Validate()
			if err == nil {
				t.Errorf("Validate() should fail for %s", tt.name)
			}
			if !errors.Is(err, ErrInvalidBitLayout) {
				t.Errorf("Expected ErrInvalidBitLayout, got: %v", err)
			}
		})
	}
}

// ============================================================================
// BitLayout.CalculateCapacity() Tests
// ============================================================================

func TestBitLayout_CalculateCapacity_LayoutDefault(t *testing.T) {
	cap := LayoutDefault.CalculateCapacity()

	// 41 bits = 2^41 = 2,199,023,255,552 milliseconds
	// = ~69.7 years
	expectedWorkers := int64(1024)    // 2^10
	expectedSequence := int64(4096)   // 2^12
	expectedThroughput := int64(4096000) // 4096 IDs per millisecond = 4,096,000/sec

	if cap.MaxWorkers != expectedWorkers {
		t.Errorf("MaxWorkers = %d, want %d", cap.MaxWorkers, expectedWorkers)
	}
	if cap.MaxSequence != expectedSequence {
		t.Errorf("MaxSequence = %d, want %d", cap.MaxSequence, expectedSequence)
	}
	if cap.ThroughputPerWorker != expectedThroughput {
		t.Errorf("ThroughputPerWorker = %d, want %d", cap.ThroughputPerWorker, expectedThroughput)
	}
	if cap.TimeUnit != time.Millisecond {
		t.Errorf("TimeUnit = %v, want %v", cap.TimeUnit, time.Millisecond)
	}

	// Lifespan should be ~69 years (allowing some margin)
	years := int(cap.Lifespan.Hours() / 24 / 365)
	if years < 69 || years > 70 {
		t.Errorf("Lifespan years = %d, want ~69", years)
	}
}

func TestBitLayout_CalculateCapacity_LayoutSuperior(t *testing.T) {
	cap := LayoutSuperior.CalculateCapacity()

	// 40+14+9: 16K workers, 512 sequence, ~35 years
	expectedWorkers := int64(16384)  // 2^14
	expectedSequence := int64(512)   // 2^9
	expectedThroughput := int64(512000) // 512 IDs per millisecond

	if cap.MaxWorkers != expectedWorkers {
		t.Errorf("MaxWorkers = %d, want %d", cap.MaxWorkers, expectedWorkers)
	}
	if cap.MaxSequence != expectedSequence {
		t.Errorf("MaxSequence = %d, want %d", cap.MaxSequence, expectedSequence)
	}
	if cap.ThroughputPerWorker != expectedThroughput {
		t.Errorf("ThroughputPerWorker = %d, want %d", cap.ThroughputPerWorker, expectedThroughput)
	}

	// Lifespan should be ~35 years
	years := int(cap.Lifespan.Hours() / 24 / 365)
	if years < 34 || years > 36 {
		t.Errorf("Lifespan years = %d, want ~35", years)
	}
}

func TestBitLayout_CalculateCapacity_LayoutExtreme(t *testing.T) {
	cap := LayoutExtreme.CalculateCapacity()

	// 39+17+7: 131K workers, 128 sequence, ~17 years (with 1ms time unit)
	expectedWorkers := int64(131072) // 2^17
	expectedSequence := int64(128)   // 2^7
	expectedThroughput := int64(128000) // 128 IDs per millisecond

	if cap.MaxWorkers != expectedWorkers {
		t.Errorf("MaxWorkers = %d, want %d", cap.MaxWorkers, expectedWorkers)
	}
	if cap.MaxSequence != expectedSequence {
		t.Errorf("MaxSequence = %d, want %d", cap.MaxSequence, expectedSequence)
	}
	if cap.ThroughputPerWorker != expectedThroughput {
		t.Errorf("ThroughputPerWorker = %d, want %d", cap.ThroughputPerWorker, expectedThroughput)
	}

	// Lifespan should be ~17 years (39 bits * 1ms)
	years := int(cap.Lifespan.Hours() / 24 / 365)
	if years < 17 || years > 18 {
		t.Errorf("Lifespan years = %d, want ~17", years)
	}
}

func TestBitLayout_CalculateCapacity_LayoutSonyflake(t *testing.T) {
	cap := LayoutSonyflake.CalculateCapacity()

	// 39+16+8 with 10ms time unit: 65K workers, 256 sequence, ~174 years
	expectedWorkers := int64(65536)  // 2^16
	expectedSequence := int64(256)   // 2^8
	expectedThroughput := int64(25600) // 256 IDs per 10ms = 25,600/sec

	if cap.MaxWorkers != expectedWorkers {
		t.Errorf("MaxWorkers = %d, want %d", cap.MaxWorkers, expectedWorkers)
	}
	if cap.MaxSequence != expectedSequence {
		t.Errorf("MaxSequence = %d, want %d", cap.MaxSequence, expectedSequence)
	}
	if cap.ThroughputPerWorker != expectedThroughput {
		t.Errorf("ThroughputPerWorker = %d, want %d", cap.ThroughputPerWorker, expectedThroughput)
	}
	if cap.TimeUnit != 10*time.Millisecond {
		t.Errorf("TimeUnit = %v, want %v", cap.TimeUnit, 10*time.Millisecond)
	}

	// Lifespan should be ~174 years
	years := int(cap.Lifespan.Hours() / 24 / 365)
	if years < 173 || years > 175 {
		t.Errorf("Lifespan years = %d, want ~174", years)
	}
}

func TestBitLayout_CalculateCapacity_LayoutUltimate(t *testing.T) {
	cap := LayoutUltimate.CalculateCapacity()

	// 40+16+7 with 10ms time unit: 65K workers, 128 sequence, ~292 years
	// Note: Theoretical max is 348 years, but time.Duration maxes at ~292 years
	expectedWorkers := int64(65536)  // 2^16
	expectedSequence := int64(128)   // 2^7
	expectedThroughput := int64(12800) // 128 IDs per 10ms = 12,800/sec

	if cap.MaxWorkers != expectedWorkers {
		t.Errorf("MaxWorkers = %d, want %d", cap.MaxWorkers, expectedWorkers)
	}
	if cap.MaxSequence != expectedSequence {
		t.Errorf("MaxSequence = %d, want %d", cap.MaxSequence, expectedSequence)
	}
	if cap.ThroughputPerWorker != expectedThroughput {
		t.Errorf("ThroughputPerWorker = %d, want %d", cap.ThroughputPerWorker, expectedThroughput)
	}
	if cap.TimeUnit != 10*time.Millisecond {
		t.Errorf("TimeUnit = %v, want %v", cap.TimeUnit, 10*time.Millisecond)
	}

	// Lifespan is capped at ~292 years (time.Duration int64 limit)
	years := int(cap.Lifespan.Hours() / 24 / 365)
	if years < 291 || years > 293 {
		t.Errorf("Lifespan years = %d, want ~292", years)
	}
}

func TestBitLayout_CalculateCapacity_LayoutMegaScale(t *testing.T) {
	cap := LayoutMegaScale.CalculateCapacity()

	// 40+17+6 with 10ms time unit: 131K workers, 64 sequence, ~292 years
	// Note: Theoretical max is 348 years, but time.Duration maxes at ~292 years
	expectedWorkers := int64(131072)  // 2^17
	expectedSequence := int64(64)     // 2^6
	expectedThroughput := int64(6400) // 64 IDs per 10ms = 6,400/sec

	if cap.MaxWorkers != expectedWorkers {
		t.Errorf("MaxWorkers = %d, want %d", cap.MaxWorkers, expectedWorkers)
	}
	if cap.MaxSequence != expectedSequence {
		t.Errorf("MaxSequence = %d, want %d", cap.MaxSequence, expectedSequence)
	}
	if cap.ThroughputPerWorker != expectedThroughput {
		t.Errorf("ThroughputPerWorker = %d, want %d", cap.ThroughputPerWorker, expectedThroughput)
	}
	if cap.TimeUnit != 10*time.Millisecond {
		t.Errorf("TimeUnit = %v, want %v", cap.TimeUnit, 10*time.Millisecond)
	}

	// Lifespan is capped at ~292 years (time.Duration int64 limit)
	years := int(cap.Lifespan.Hours() / 24 / 365)
	if years < 291 || years > 293 {
		t.Errorf("Lifespan years = %d, want ~292", years)
	}
}

func TestBitLayout_CalculateCapacity_String(t *testing.T) {
	cap := LayoutDefault.CalculateCapacity()
	str := cap.String()

	// Should contain key metrics
	if str == "" {
		t.Error("String() should not be empty")
	}

	// Should contain worker count
	if !contains(str, "1024") {
		t.Errorf("String() should contain MaxWorkers count, got: %s", str)
	}
}

// ============================================================================
// BitLayout.CalculateShifts() Tests
// ============================================================================

func TestBitLayout_CalculateShifts_LayoutDefault(t *testing.T) {
	timestampShift, workerShift, maxWorker, maxSequence := LayoutDefault.CalculateShifts()

	if timestampShift != 22 { // 10 + 12
		t.Errorf("timestampShift = %d, want 22", timestampShift)
	}
	if workerShift != 12 {
		t.Errorf("workerShift = %d, want 12", workerShift)
	}
	if maxWorker != 1023 { // 2^10 - 1
		t.Errorf("maxWorker = %d, want 1023", maxWorker)
	}
	if maxSequence != 4095 { // 2^12 - 1
		t.Errorf("maxSequence = %d, want 4095", maxSequence)
	}
}

func TestBitLayout_CalculateShifts_LayoutSuperior(t *testing.T) {
	timestampShift, workerShift, maxWorker, maxSequence := LayoutSuperior.CalculateShifts()

	if timestampShift != 23 { // 14 + 9
		t.Errorf("timestampShift = %d, want 23", timestampShift)
	}
	if workerShift != 9 {
		t.Errorf("workerShift = %d, want 9", workerShift)
	}
	if maxWorker != 16383 { // 2^14 - 1
		t.Errorf("maxWorker = %d, want 16383", maxWorker)
	}
	if maxSequence != 511 { // 2^9 - 1
		t.Errorf("maxSequence = %d, want 511", maxSequence)
	}
}

func TestBitLayout_CalculateShifts_LayoutExtreme(t *testing.T) {
	timestampShift, workerShift, maxWorker, maxSequence := LayoutExtreme.CalculateShifts()

	if timestampShift != 24 { // 17 + 7
		t.Errorf("timestampShift = %d, want 24", timestampShift)
	}
	if workerShift != 7 {
		t.Errorf("workerShift = %d, want 7", workerShift)
	}
	if maxWorker != 131071 { // 2^17 - 1
		t.Errorf("maxWorker = %d, want 131071", maxWorker)
	}
	if maxSequence != 127 { // 2^7 - 1
		t.Errorf("maxSequence = %d, want 127", maxSequence)
	}
}

func TestBitLayout_CalculateShifts_LayoutUltimate(t *testing.T) {
	timestampShift, workerShift, maxWorker, maxSequence := LayoutUltimate.CalculateShifts()

	if timestampShift != 23 { // 16 + 7
		t.Errorf("timestampShift = %d, want 23", timestampShift)
	}
	if workerShift != 7 {
		t.Errorf("workerShift = %d, want 7", workerShift)
	}
	if maxWorker != 65535 { // 2^16 - 1
		t.Errorf("maxWorker = %d, want 65535", maxWorker)
	}
	if maxSequence != 127 { // 2^7 - 1
		t.Errorf("maxSequence = %d, want 127", maxSequence)
	}
}

func TestBitLayout_CalculateShifts_LayoutMegaScale(t *testing.T) {
	timestampShift, workerShift, maxWorker, maxSequence := LayoutMegaScale.CalculateShifts()

	if timestampShift != 23 { // 17 + 6
		t.Errorf("timestampShift = %d, want 23", timestampShift)
	}
	if workerShift != 6 {
		t.Errorf("workerShift = %d, want 6", workerShift)
	}
	if maxWorker != 131071 { // 2^17 - 1
		t.Errorf("maxWorker = %d, want 131071", maxWorker)
	}
	if maxSequence != 63 { // 2^6 - 1
		t.Errorf("maxSequence = %d, want 63", maxSequence)
	}
}

// ============================================================================
// BitLayout.ValidateWorkerID() Tests
// ============================================================================

func TestBitLayout_ValidateWorkerID_Valid(t *testing.T) {
	tests := []struct {
		name     string
		layout   BitLayout
		workerID int64
	}{
		{"LayoutDefault min", LayoutDefault, 0},
		{"LayoutDefault mid", LayoutDefault, 512},
		{"LayoutDefault max", LayoutDefault, 1023},
		{"LayoutSuperior max", LayoutSuperior, 16383},
		{"LayoutExtreme max", LayoutExtreme, 131071},
		{"LayoutSonyflake max", LayoutSonyflake, 65535},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.layout.ValidateWorkerID(tt.workerID); err != nil {
				t.Errorf("ValidateWorkerID(%d) should succeed for %s, got: %v",
					tt.workerID, tt.name, err)
			}
		})
	}
}

func TestBitLayout_ValidateWorkerID_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		layout   BitLayout
		workerID int64
	}{
		{"LayoutDefault negative", LayoutDefault, -1},
		{"LayoutDefault too large", LayoutDefault, 1024},
		{"LayoutSuperior too large", LayoutSuperior, 16384},
		{"LayoutExtreme too large", LayoutExtreme, 131072},
		{"LayoutSonyflake too large", LayoutSonyflake, 65536},
		{"LayoutDefault way too large", LayoutDefault, 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.layout.ValidateWorkerID(tt.workerID)
			if err == nil {
				t.Errorf("ValidateWorkerID(%d) should fail for %s", tt.workerID, tt.name)
			}
			if !errors.Is(err, ErrLayoutWorkerIDTooLarge) {
				t.Errorf("Expected ErrLayoutWorkerIDTooLarge, got: %v", err)
			}
		})
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestBitLayout_AllPresetsValid(t *testing.T) {
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
			// Validate
			if err := tt.layout.Validate(); err != nil {
				t.Errorf("%s.Validate() failed: %v", tt.name, err)
			}

			// CalculateCapacity
			cap := tt.layout.CalculateCapacity()
			if cap.MaxWorkers <= 0 {
				t.Errorf("%s capacity invalid: MaxWorkers = %d", tt.name, cap.MaxWorkers)
			}
			if cap.MaxSequence <= 0 {
				t.Errorf("%s capacity invalid: MaxSequence = %d", tt.name, cap.MaxSequence)
			}
			if cap.Lifespan <= 0 {
				t.Errorf("%s capacity invalid: Lifespan = %v", tt.name, cap.Lifespan)
			}

			// CalculateShifts
			timestampShift, workerShift, maxWorker, maxSequence := tt.layout.CalculateShifts()
			if timestampShift <= 0 || timestampShift >= 64 {
				t.Errorf("%s shifts invalid: timestampShift = %d", tt.name, timestampShift)
			}
			if workerShift < 0 || workerShift >= 64 {
				t.Errorf("%s shifts invalid: workerShift = %d", tt.name, workerShift)
			}
			if maxWorker <= 0 {
				t.Errorf("%s shifts invalid: maxWorker = %d", tt.name, maxWorker)
			}
			if maxSequence <= 0 {
				t.Errorf("%s shifts invalid: maxSequence = %d", tt.name, maxSequence)
			}

			// Validate worker ID bounds
			if err := tt.layout.ValidateWorkerID(0); err != nil {
				t.Errorf("%s.ValidateWorkerID(0) should succeed: %v", tt.name, err)
			}
			if err := tt.layout.ValidateWorkerID(maxWorker); err != nil {
				t.Errorf("%s.ValidateWorkerID(%d) should succeed: %v", tt.name, maxWorker, err)
			}
			if err := tt.layout.ValidateWorkerID(maxWorker + 1); err == nil {
				t.Errorf("%s.ValidateWorkerID(%d) should fail", tt.name, maxWorker+1)
			}
		})
	}
}

// ============================================================================
// Bitshift Helper Functions Tests
// ============================================================================

func TestIsPowerOfTwo_TrueCases(t *testing.T) {
	tests := []int64{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192}

	for _, n := range tests {
		t.Run("PowerOfTwo_"+string(rune(n)), func(t *testing.T) {
			if !isPowerOfTwo(n) {
				t.Errorf("isPowerOfTwo(%d) = false, want true", n)
			}
		})
	}
}

func TestIsPowerOfTwo_FalseCases(t *testing.T) {
	tests := []int64{0, -1, -2, 3, 5, 6, 7, 9, 10, 11, 12, 13, 14, 15, 17, 100, 1000}

	for _, n := range tests {
		t.Run("NotPowerOfTwo_"+string(rune(n)), func(t *testing.T) {
			if isPowerOfTwo(n) {
				t.Errorf("isPowerOfTwo(%d) = true, want false", n)
			}
		})
	}
}

func TestCalculateTimeUnitShift_PowerOfTwo(t *testing.T) {
	tests := []struct {
		timeUnit time.Duration
		expected int8
	}{
		{1 * time.Millisecond, 0},  // 2^0 = 1
		{2 * time.Millisecond, 1},  // 2^1 = 2
		{4 * time.Millisecond, 2},  // 2^2 = 4
		{8 * time.Millisecond, 3},  // 2^3 = 8
		{16 * time.Millisecond, 4}, // 2^4 = 16
		{32 * time.Millisecond, 5}, // 2^5 = 32
		{64 * time.Millisecond, 6}, // 2^6 = 64
	}

	for _, tt := range tests {
		t.Run(tt.timeUnit.String(), func(t *testing.T) {
			result := calculateTimeUnitShift(tt.timeUnit)
			if result != tt.expected {
				t.Errorf("calculateTimeUnitShift(%v) = %d, want %d", tt.timeUnit, result, tt.expected)
			}
		})
	}
}

func TestCalculateTimeUnitShift_NonPowerOfTwo(t *testing.T) {
	tests := []time.Duration{
		3 * time.Millisecond,
		5 * time.Millisecond,
		6 * time.Millisecond,
		7 * time.Millisecond,
		9 * time.Millisecond,
		10 * time.Millisecond,
		12 * time.Millisecond,
		15 * time.Millisecond,
		100 * time.Millisecond,
	}

	for _, timeUnit := range tests {
		t.Run(timeUnit.String(), func(t *testing.T) {
			result := calculateTimeUnitShift(timeUnit)
			if result != -1 {
				t.Errorf("calculateTimeUnitShift(%v) = %d, want -1", timeUnit, result)
			}
		})
	}
}

func TestCalculateTimeUnitShift_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		timeUnit time.Duration
		expected int8
	}{
		{"Zero", 0, -1},
		{"Negative", -1 * time.Millisecond, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateTimeUnitShift(tt.timeUnit)
			if result != tt.expected {
				t.Errorf("calculateTimeUnitShift(%v) = %d, want %d", tt.timeUnit, result, tt.expected)
			}
		})
	}
}

func TestBitLayout_TimeUnitShift_AllLayouts(t *testing.T) {
	tests := []struct {
		name     string
		layout   BitLayout
		expected int8
	}{
		{"LayoutDefault (1ms)", LayoutDefault, 0},
		{"LayoutSuperior (1ms)", LayoutSuperior, 0},
		{"LayoutExtreme (1ms)", LayoutExtreme, 0},
		{"LayoutUltra (1ms)", LayoutUltra, 0},
		{"LayoutLongLife (1ms)", LayoutLongLife, 0},
		{"LayoutSonyflake (10ms)", LayoutSonyflake, -1}, // Non-power-of-2
		{"LayoutUltimate (10ms)", LayoutUltimate, -1},   // Non-power-of-2
		{"LayoutMegaScale (10ms)", LayoutMegaScale, -1}, // Non-power-of-2
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.layout.TimeUnitShift()
			if result != tt.expected {
				t.Errorf("%s.TimeUnitShift() = %d, want %d", tt.name, result, tt.expected)
			}
		})
	}
}

func TestBitshiftVsDivision_Equivalence(t *testing.T) {
	// Verify that bitshift and division produce identical results for all time units
	testCases := []struct {
		name        string
		timeUnit    time.Duration
		currentMs   int64
		description string
	}{
		{"1ms_1000ms", 1 * time.Millisecond, 1000, "1 second"},
		{"1ms_60000ms", 1 * time.Millisecond, 60000, "1 minute"},
		{"2ms_1000ms", 2 * time.Millisecond, 1000, "1 second"},
		{"2ms_60000ms", 2 * time.Millisecond, 60000, "1 minute"},
		{"4ms_1000ms", 4 * time.Millisecond, 1000, "1 second"},
		{"4ms_60000ms", 4 * time.Millisecond, 60000, "1 minute"},
		{"8ms_1000ms", 8 * time.Millisecond, 1000, "1 second"},
		{"10ms_1000ms", 10 * time.Millisecond, 1000, "1 second"},
		{"10ms_60000ms", 10 * time.Millisecond, 60000, "1 minute"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Division method (always works)
			divisionResult := tc.currentMs / tc.timeUnit.Milliseconds()

			// Bitshift method (for power-of-2 only)
			shift := calculateTimeUnitShift(tc.timeUnit)
			var bitshiftResult int64
			if shift >= 0 {
				bitshiftResult = tc.currentMs >> shift
			} else {
				bitshiftResult = tc.currentMs / tc.timeUnit.Milliseconds()
			}

			if divisionResult != bitshiftResult {
				t.Errorf("Results differ for %s: division=%d, bitshift=%d",
					tc.description, divisionResult, bitshiftResult)
			}
		})
	}
}

func TestBitshiftPerformance_Optimization(t *testing.T) {
	// Verify that layouts with power-of-2 time units use bitshift (shift >= 0)
	// and non-power-of-2 use division (shift == -1)

	powerOfTwoLayouts := []struct {
		name   string
		layout BitLayout
	}{
		{"LayoutDefault", LayoutDefault},
		{"LayoutSuperior", LayoutSuperior},
		{"LayoutExtreme", LayoutExtreme},
		{"LayoutUltra", LayoutUltra},
		{"LayoutLongLife", LayoutLongLife},
	}

	for _, tt := range powerOfTwoLayouts {
		t.Run(tt.name+"_ShouldUseBitshift", func(t *testing.T) {
			shift := tt.layout.TimeUnitShift()
			if shift < 0 {
				t.Errorf("%s should use bitshift optimization (shift >= 0), got shift = %d",
					tt.name, shift)
			}
		})
	}

	nonPowerOfTwoLayouts := []struct {
		name   string
		layout BitLayout
	}{
		{"LayoutSonyflake", LayoutSonyflake},
		{"LayoutUltimate", LayoutUltimate},
		{"LayoutMegaScale", LayoutMegaScale},
	}

	for _, tt := range nonPowerOfTwoLayouts {
		t.Run(tt.name+"_ShouldUseDivision", func(t *testing.T) {
			shift := tt.layout.TimeUnitShift()
			if shift != -1 {
				t.Errorf("%s should use division fallback (shift == -1), got shift = %d",
					tt.name, shift)
			}
		})
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
