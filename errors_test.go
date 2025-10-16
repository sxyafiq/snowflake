package snowflake

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// ClockError Tests
// ============================================================================

func TestClockError_Error(t *testing.T) {
	err := newClockError(1000, 1500, 5, 42, false)

	msg := err.Error()

	// Should contain key information
	if !strings.Contains(msg, "clock moved backwards") {
		t.Error("Error message should contain 'clock moved backwards'")
	}
	if !strings.Contains(msg, "drift=500ms") {
		t.Errorf("Error message should contain drift amount, got: %s", msg)
	}
	if !strings.Contains(msg, "worker=42") {
		t.Errorf("Error message should contain worker ID, got: %s", msg)
	}
	if !strings.Contains(msg, "unrecoverable") {
		t.Errorf("Error message should indicate recovery status, got: %s", msg)
	}
}

func TestClockError_Unwrap(t *testing.T) {
	err := newClockError(1000, 1500, 5, 42, false)

	if !errors.Is(err, ErrClockMovedBack) {
		t.Error("ClockError should unwrap to ErrClockMovedBack")
	}
}

func TestClockError_DriftDuration(t *testing.T) {
	err := newClockError(1000, 1500, 5, 42, false)

	duration := err.DriftDuration()
	expected := 500 * time.Millisecond

	if duration != expected {
		t.Errorf("DriftDuration() = %v, want %v", duration, expected)
	}
}

func TestClockError_ToleranceDuration(t *testing.T) {
	err := newClockError(1000, 1500, 5, 42, false)

	duration := err.ToleranceDuration()
	expected := 5 * time.Millisecond

	if duration != expected {
		t.Errorf("ToleranceDuration() = %v, want %v", duration, expected)
	}
}

func TestClockError_ExceedsTolerance(t *testing.T) {
	tests := []struct {
		name      string
		drift     int64
		tolerance int64
		want      bool
	}{
		{"Within tolerance", 3, 5, false},
		{"Equal to tolerance", 5, 5, false},
		{"Exceeds tolerance", 10, 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newClockError(1000, 1000+tt.drift, tt.tolerance, 42, false)
			if got := err.ExceedsTolerance(); got != tt.want {
				t.Errorf("ExceedsTolerance() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClockError_RecoveredStatus(t *testing.T) {
	recovered := newClockError(1000, 1500, 5, 42, true)
	unrecovered := newClockError(1000, 1500, 5, 42, false)

	recoveredMsg := recovered.Error()
	unrecoveredMsg := unrecovered.Error()

	if !strings.Contains(recoveredMsg, "recovered") {
		t.Error("Recovered error should contain 'recovered' status")
	}
	if !strings.Contains(unrecoveredMsg, "unrecoverable") {
		t.Error("Unrecovered error should contain 'unrecoverable' status")
	}
}

// ============================================================================
// ConfigError Tests
// ============================================================================

func TestConfigError_Error(t *testing.T) {
	err := newConfigError("WorkerID", "1024", "out of range", "must be 0-1023")

	msg := err.Error()

	// Should contain all fields
	if !strings.Contains(msg, "WorkerID") {
		t.Errorf("Error message should contain field name, got: %s", msg)
	}
	if !strings.Contains(msg, "1024") {
		t.Errorf("Error message should contain value, got: %s", msg)
	}
	if !strings.Contains(msg, "out of range") {
		t.Errorf("Error message should contain reason, got: %s", msg)
	}
	if !strings.Contains(msg, "must be 0-1023") {
		t.Errorf("Error message should contain constraint, got: %s", msg)
	}
}

func TestConfigError_Unwrap(t *testing.T) {
	err := newConfigError("WorkerID", "1024", "out of range", "must be 0-1023")

	if !errors.Is(err, ErrInvalidConfig) {
		t.Error("ConfigError should unwrap to ErrInvalidConfig")
	}
}

// ============================================================================
// OverflowError Tests
// ============================================================================

func TestOverflowError_SequenceType(t *testing.T) {
	err := newSequenceOverflowError(12345, 4096, 42, 4095, 500*time.Microsecond)

	msg := err.Error()

	if !strings.Contains(msg, "sequence overflow") {
		t.Errorf("Error message should contain 'sequence overflow', got: %s", msg)
	}
	if !strings.Contains(msg, "worker=42") {
		t.Errorf("Error message should contain worker ID, got: %s", msg)
	}
	if !strings.Contains(msg, "waited=500µs") {
		t.Errorf("Error message should contain wait duration, got: %s", msg)
	}
}

func TestOverflowError_TimestampType(t *testing.T) {
	err := newTimestampOverflowError(12345, 42)

	msg := err.Error()

	if !strings.Contains(msg, "timestamp overflow") {
		t.Errorf("Error message should contain 'timestamp overflow', got: %s", msg)
	}
	if !strings.Contains(msg, "worker=42") {
		t.Errorf("Error message should contain worker ID, got: %s", msg)
	}
}

func TestOverflowError_Unwrap(t *testing.T) {
	err := newSequenceOverflowError(12345, 4096, 42, 4095, 0)

	if !errors.Is(err, ErrSequenceOverflow) {
		t.Error("OverflowError should unwrap to ErrSequenceOverflow")
	}
}

func TestOverflowType_String(t *testing.T) {
	tests := []struct {
		typ  OverflowType
		want string
	}{
		{SequenceOverflowType, "sequence_overflow"},
		{TimestampOverflowType, "timestamp_overflow"},
		{OverflowType(999), "unknown_overflow"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.typ.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// Error Helper Functions Tests
// ============================================================================

func TestIsClockError(t *testing.T) {
	clockErr := newClockError(1000, 1500, 5, 42, false)
	configErr := newConfigError("WorkerID", "1024", "invalid", "must be 0-1023")
	stdErr := errors.New("standard error")

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"ClockError", clockErr, true},
		{"ConfigError", configErr, false},
		{"Standard error", stdErr, false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsClockError(tt.err); got != tt.want {
				t.Errorf("IsClockError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsConfigError(t *testing.T) {
	configErr := newConfigError("WorkerID", "1024", "invalid", "must be 0-1023")
	clockErr := newClockError(1000, 1500, 5, 42, false)
	stdErr := errors.New("standard error")

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"ConfigError", configErr, true},
		{"ClockError", clockErr, false},
		{"Standard error", stdErr, false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConfigError(tt.err); got != tt.want {
				t.Errorf("IsConfigError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsOverflowError(t *testing.T) {
	overflowErr := newSequenceOverflowError(12345, 4096, 42, 4095, 0)
	clockErr := newClockError(1000, 1500, 5, 42, false)
	stdErr := errors.New("standard error")

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"OverflowError", overflowErr, true},
		{"ClockError", clockErr, false},
		{"Standard error", stdErr, false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOverflowError(tt.err); got != tt.want {
				t.Errorf("IsOverflowError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetClockError(t *testing.T) {
	clockErr := newClockError(1000, 1500, 5, 42, false)
	configErr := newConfigError("WorkerID", "1024", "invalid", "must be 0-1023")

	// Should extract ClockError
	if extracted, ok := GetClockError(clockErr); !ok {
		t.Error("GetClockError() should extract ClockError")
	} else if extracted.DriftMilliseconds != 500 {
		t.Errorf("Extracted drift = %d, want 500", extracted.DriftMilliseconds)
	}

	// Should not extract from other error types
	if _, ok := GetClockError(configErr); ok {
		t.Error("GetClockError() should not extract from ConfigError")
	}

	// Should not extract from nil
	if _, ok := GetClockError(nil); ok {
		t.Error("GetClockError() should not extract from nil")
	}
}

func TestGetConfigError(t *testing.T) {
	configErr := newConfigError("WorkerID", "1024", "invalid", "must be 0-1023")
	clockErr := newClockError(1000, 1500, 5, 42, false)

	// Should extract ConfigError
	if extracted, ok := GetConfigError(configErr); !ok {
		t.Error("GetConfigError() should extract ConfigError")
	} else if extracted.Field != "WorkerID" {
		t.Errorf("Extracted field = %s, want WorkerID", extracted.Field)
	}

	// Should not extract from other error types
	if _, ok := GetConfigError(clockErr); ok {
		t.Error("GetConfigError() should not extract from ClockError")
	}

	// Should not extract from nil
	if _, ok := GetConfigError(nil); ok {
		t.Error("GetConfigError() should not extract from nil")
	}
}

func TestGetOverflowError(t *testing.T) {
	overflowErr := newSequenceOverflowError(12345, 4096, 42, 4095, 100*time.Microsecond)
	clockErr := newClockError(1000, 1500, 5, 42, false)

	// Should extract OverflowError
	if extracted, ok := GetOverflowError(overflowErr); !ok {
		t.Error("GetOverflowError() should extract OverflowError")
	} else if extracted.Type != SequenceOverflowType {
		t.Errorf("Extracted type = %v, want SequenceOverflowType", extracted.Type)
	}

	// Should not extract from other error types
	if _, ok := GetOverflowError(clockErr); ok {
		t.Error("GetOverflowError() should not extract from ClockError")
	}

	// Should not extract from nil
	if _, ok := GetOverflowError(nil); ok {
		t.Error("GetOverflowError() should not extract from nil")
	}
}

// ============================================================================
// Integration Tests with Generator
// ============================================================================

func TestConfigValidation_WithNewErrorTypes(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		field  string
	}{
		{
			"Invalid WorkerID negative",
			Config{WorkerID: -1, Epoch: Epoch, MaxClockBackward: 5 * time.Millisecond},
			"WorkerID",
		},
		{
			"Invalid WorkerID too large",
			Config{WorkerID: 2000, Epoch: Epoch, MaxClockBackward: 5 * time.Millisecond},
			"WorkerID",
		},
		{
			"Invalid Epoch",
			Config{WorkerID: 1, Epoch: -100, MaxClockBackward: 5 * time.Millisecond},
			"Epoch",
		},
		{
			"Invalid MaxClockBackward",
			Config{WorkerID: 1, Epoch: Epoch, MaxClockBackward: -1 * time.Millisecond},
			"MaxClockBackward",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWithConfig(tt.config)
			if err == nil {
				t.Fatal("NewWithConfig() should return error for invalid config")
			}

			// Should be a ConfigError
			if !IsConfigError(err) {
				t.Errorf("Error should be ConfigError, got: %T", err)
			}

			// Should contain field name
			configErr, ok := GetConfigError(err)
			if !ok {
				t.Fatal("Should be able to extract ConfigError")
			}

			if configErr.Field != tt.field {
				t.Errorf("ConfigError.Field = %s, want %s", configErr.Field, tt.field)
			}
		})
	}
}

func TestConfigValidation_ValidConfig(t *testing.T) {
	cfg := Config{
		WorkerID:         42,
		Epoch:            Epoch,
		MaxClockBackward: 10 * time.Millisecond,
		EnableMetrics:    true,
	}

	gen, err := NewWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewWithConfig() with valid config should succeed, got: %v", err)
	}

	if gen == nil {
		t.Error("Generator should not be nil")
	}

	if gen.WorkerID() != 42 {
		t.Errorf("WorkerID = %d, want 42", gen.WorkerID())
	}
}

// ============================================================================
// Backward Compatibility Tests
// ============================================================================

func TestBackwardCompatibility_ErrorsIs(t *testing.T) {
	// ClockError should still work with errors.Is for ErrClockMovedBack
	clockErr := newClockError(1000, 1500, 5, 42, false)
	if !errors.Is(clockErr, ErrClockMovedBack) {
		t.Error("errors.Is should work with ClockError and ErrClockMovedBack")
	}

	// ConfigError should still work with errors.Is for ErrInvalidConfig
	configErr := newConfigError("WorkerID", "1024", "invalid", "must be 0-1023")
	if !errors.Is(configErr, ErrInvalidConfig) {
		t.Error("errors.Is should work with ConfigError and ErrInvalidConfig")
	}
}

func TestBackwardCompatibility_OldErrorVariables(t *testing.T) {
	// Ensure old error variables still exist and work
	if ErrInvalidWorkerID == nil {
		t.Error("ErrInvalidWorkerID should exist")
	}
	if ErrClockMovedBack == nil {
		t.Error("ErrClockMovedBack should exist")
	}
	if ErrContextCanceled == nil {
		t.Error("ErrContextCanceled should exist")
	}
	if ErrInvalidConfig == nil {
		t.Error("ErrInvalidConfig should exist")
	}
}
