// Package snowflake - errors.go provides custom error types with rich context.
//
// These error types provide detailed information for debugging and monitoring,
// including timestamps, worker IDs, drift amounts, and configuration details.

package snowflake

import (
	"errors"
	"fmt"
	"time"
)

// Standard errors (declared in snowflake.go, referenced here for documentation)
// These can be used with errors.Is() and errors.As() for error checking.
var (
	// ErrSequenceOverflow is returned when sequence exhaustion occurs.
	// This is typically handled internally but can be exposed for monitoring.
	ErrSequenceOverflow = errors.New("sequence overflow")
)

// ============================================================================
// Custom Error Types
// ============================================================================

// ClockError represents a clock-related error with detailed timing information.
//
// This error type captures the exact timing details when clock drift is detected,
// making it easier to debug NTP issues, system time changes, or VM migrations.
//
// Example usage:
//
//	if err := gen.GenerateID(); err != nil {
//	    var clockErr *ClockError
//	    if errors.As(err, &clockErr) {
//	        log.Error("clock drift detected",
//	            "drift_ms", clockErr.DriftMilliseconds,
//	            "current", clockErr.CurrentTimestamp,
//	            "last", clockErr.LastTimestamp,
//	            "worker", clockErr.WorkerID)
//	    }
//	}
type ClockError struct {
	// CurrentTimestamp is the current monotonic timestamp in milliseconds.
	CurrentTimestamp int64

	// LastTimestamp is the last generated timestamp in milliseconds.
	LastTimestamp int64

	// DriftMilliseconds is the amount of backward drift (always positive).
	DriftMilliseconds int64

	// ToleranceMilliseconds is the maximum acceptable drift.
	ToleranceMilliseconds int64

	// WorkerID is the ID of the generator that encountered the error.
	WorkerID int64

	// Recovered indicates whether the error was recovered (by waiting).
	// If false, this error caused ID generation to fail.
	Recovered bool
}

// Error implements the error interface.
func (e *ClockError) Error() string {
	status := "unrecoverable"
	if e.Recovered {
		status = "recovered"
	}
	return fmt.Sprintf("clock moved backwards: drift=%dms tolerance=%dms current=%d last=%d worker=%d (%s)",
		e.DriftMilliseconds, e.ToleranceMilliseconds,
		e.CurrentTimestamp, e.LastTimestamp, e.WorkerID, status)
}

// Unwrap returns the underlying error for errors.Is() compatibility.
func (e *ClockError) Unwrap() error {
	return ErrClockMovedBack
}

// DriftDuration returns the drift amount as a time.Duration.
func (e *ClockError) DriftDuration() time.Duration {
	return time.Duration(e.DriftMilliseconds) * time.Millisecond
}

// ToleranceDuration returns the tolerance as a time.Duration.
func (e *ClockError) ToleranceDuration() time.Duration {
	return time.Duration(e.ToleranceMilliseconds) * time.Millisecond
}

// ExceedsTolerance returns true if the drift exceeds the tolerance.
func (e *ClockError) ExceedsTolerance() bool {
	return e.DriftMilliseconds > e.ToleranceMilliseconds
}

// ConfigError represents a configuration validation error.
//
// This error type provides details about which configuration field failed
// validation and why, making it easier to diagnose configuration issues.
//
// Example usage:
//
//	if err := snowflake.NewWithConfig(cfg); err != nil {
//	    var configErr *ConfigError
//	    if errors.As(err, &configErr) {
//	        log.Error("invalid configuration",
//	            "field", configErr.Field,
//	            "value", configErr.Value,
//	            "reason", configErr.Reason)
//	    }
//	}
type ConfigError struct {
	// Field is the name of the configuration field that failed validation.
	Field string

	// Value is the invalid value (as string for logging).
	Value string

	// Reason is a human-readable explanation of why the value is invalid.
	Reason string

	// Constraint describes the valid range or constraint.
	// Example: "must be between 0 and 1023"
	Constraint string
}

// Error implements the error interface.
func (e *ConfigError) Error() string {
	return fmt.Sprintf("invalid configuration: %s=%s (%s) - %s",
		e.Field, e.Value, e.Reason, e.Constraint)
}

// Unwrap returns the underlying error for errors.Is() compatibility.
func (e *ConfigError) Unwrap() error {
	return ErrInvalidConfig
}

// OverflowError represents a sequence or timestamp overflow error.
//
// This error type provides context about overflow events, which can indicate
// either extremely high throughput (sequence overflow) or approaching the
// lifespan limit (timestamp overflow).
//
// Example usage:
//
//	if err := gen.GenerateID(); err != nil {
//	    var overflowErr *OverflowError
//	    if errors.As(err, &overflowErr) {
//	        if overflowErr.Type == SequenceOverflowType {
//	            log.Warn("high throughput detected",
//	                "sequence_count", overflowErr.SequenceCount,
//	                "timestamp", overflowErr.Timestamp)
//	        }
//	    }
//	}
type OverflowError struct {
	// Type indicates whether this is a sequence or timestamp overflow.
	Type OverflowType

	// Timestamp is the timestamp when overflow occurred (in milliseconds).
	Timestamp int64

	// SequenceCount is the sequence number that caused overflow.
	// Only relevant for SequenceOverflowType.
	SequenceCount int64

	// WorkerID is the ID of the generator that encountered overflow.
	WorkerID int64

	// MaxSequence is the maximum allowed sequence value for this generator.
	MaxSequence int64

	// WaitDuration is how long the generator waited to resolve the overflow.
	WaitDuration time.Duration
}

// OverflowType indicates the type of overflow error.
type OverflowType int

const (
	// SequenceOverflowType indicates the sequence counter exceeded maximum.
	// This happens when >4096 IDs are generated in the same millisecond.
	SequenceOverflowType OverflowType = iota

	// TimestampOverflowType indicates approaching timestamp limit.
	// This happens when nearing the ~69-year lifespan limit.
	TimestampOverflowType
)

// String returns a human-readable name for the overflow type.
func (t OverflowType) String() string {
	switch t {
	case SequenceOverflowType:
		return "sequence_overflow"
	case TimestampOverflowType:
		return "timestamp_overflow"
	default:
		return "unknown_overflow"
	}
}

// Error implements the error interface.
func (e *OverflowError) Error() string {
	switch e.Type {
	case SequenceOverflowType:
		return fmt.Sprintf("sequence overflow: generated >%d IDs in 1ms (worker=%d, timestamp=%d, waited=%v)",
			e.MaxSequence, e.WorkerID, e.Timestamp, e.WaitDuration)
	case TimestampOverflowType:
		return fmt.Sprintf("timestamp overflow: approaching lifespan limit (worker=%d, timestamp=%d)",
			e.WorkerID, e.Timestamp)
	default:
		return fmt.Sprintf("unknown overflow type: %d", e.Type)
	}
}

// Unwrap returns the underlying error for errors.Is() compatibility.
func (e *OverflowError) Unwrap() error {
	return ErrSequenceOverflow
}

// ============================================================================
// Error Helper Functions
// ============================================================================

// IsClockError checks if an error is or wraps a ClockError.
//
// This is a convenience function for checking clock-related errors.
//
// Example:
//
//	if err := gen.GenerateID(); IsClockError(err) {
//	    // Handle clock drift
//	    metrics.IncrementClockDriftCounter()
//	}
func IsClockError(err error) bool {
	var clockErr *ClockError
	return errors.As(err, &clockErr)
}

// IsConfigError checks if an error is or wraps a ConfigError.
//
// This is a convenience function for checking configuration errors.
//
// Example:
//
//	if err := NewWithConfig(cfg); IsConfigError(err) {
//	    // Log configuration problem
//	    log.Error("invalid generator configuration", "error", err)
//	}
func IsConfigError(err error) bool {
	var configErr *ConfigError
	return errors.As(err, &configErr)
}

// IsOverflowError checks if an error is or wraps an OverflowError.
//
// This is a convenience function for checking overflow errors.
//
// Example:
//
//	if err := gen.GenerateID(); IsOverflowError(err) {
//	    // Handle overflow (usually recovered automatically)
//	    log.Warn("overflow detected", "error", err)
//	}
func IsOverflowError(err error) bool {
	var overflowErr *OverflowError
	return errors.As(err, &overflowErr)
}

// GetClockError extracts the ClockError from an error chain.
//
// Returns the ClockError and true if found, nil and false otherwise.
//
// Example:
//
//	if clockErr, ok := GetClockError(err); ok {
//	    fmt.Printf("Drift: %dms\n", clockErr.DriftMilliseconds)
//	}
func GetClockError(err error) (*ClockError, bool) {
	var clockErr *ClockError
	if errors.As(err, &clockErr) {
		return clockErr, true
	}
	return nil, false
}

// GetConfigError extracts the ConfigError from an error chain.
//
// Returns the ConfigError and true if found, nil and false otherwise.
//
// Example:
//
//	if configErr, ok := GetConfigError(err); ok {
//	    fmt.Printf("Invalid field: %s\n", configErr.Field)
//	}
func GetConfigError(err error) (*ConfigError, bool) {
	var configErr *ConfigError
	if errors.As(err, &configErr) {
		return configErr, true
	}
	return nil, false
}

// GetOverflowError extracts the OverflowError from an error chain.
//
// Returns the OverflowError and true if found, nil and false otherwise.
//
// Example:
//
//	if overflowErr, ok := GetOverflowError(err); ok {
//	    fmt.Printf("Overflow type: %s\n", overflowErr.Type)
//	}
func GetOverflowError(err error) (*OverflowError, bool) {
	var overflowErr *OverflowError
	if errors.As(err, &overflowErr) {
		return overflowErr, true
	}
	return nil, false
}

// ============================================================================
// Error Constructor Helpers
// ============================================================================

// newClockError creates a new ClockError with the given parameters.
//
// This is an internal helper for creating clock errors with consistent formatting.
func newClockError(currentTs, lastTs, toleranceMs, workerID int64, recovered bool) *ClockError {
	return &ClockError{
		CurrentTimestamp:      currentTs,
		LastTimestamp:         lastTs,
		DriftMilliseconds:     lastTs - currentTs, // Always positive (last > current)
		ToleranceMilliseconds: toleranceMs,
		WorkerID:              workerID,
		Recovered:             recovered,
	}
}

// newConfigError creates a new ConfigError with the given parameters.
//
// This is an internal helper for creating config errors with consistent formatting.
func newConfigError(field, value, reason, constraint string) *ConfigError {
	return &ConfigError{
		Field:      field,
		Value:      value,
		Reason:     reason,
		Constraint: constraint,
	}
}

// newSequenceOverflowError creates a new sequence OverflowError.
//
// This is an internal helper for creating sequence overflow errors.
func newSequenceOverflowError(timestamp, sequenceCount, workerID, maxSequence int64, waitDuration time.Duration) *OverflowError {
	return &OverflowError{
		Type:          SequenceOverflowType,
		Timestamp:     timestamp,
		SequenceCount: sequenceCount,
		WorkerID:      workerID,
		MaxSequence:   maxSequence,
		WaitDuration:  waitDuration,
	}
}

// newTimestampOverflowError creates a new timestamp OverflowError.
//
// This is an internal helper for creating timestamp overflow warnings.
func newTimestampOverflowError(timestamp, workerID int64) *OverflowError {
	return &OverflowError{
		Type:      TimestampOverflowType,
		Timestamp: timestamp,
		WorkerID:  workerID,
	}
}
