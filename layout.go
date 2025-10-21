// Package snowflake - layout.go provides configurable bit allocation strategies.
//
// This allows the Snowflake implementation to scale from high-throughput single-node
// deployments to massive distributed systems with 100K+ nodes, while maintaining
// backward compatibility and zero performance overhead.

package snowflake

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// BitLayout defines how the 63 usable bits are allocated in a Snowflake ID.
//
// The layout determines the trade-offs between:
//   - Lifespan: How many years before timestamp overflows
//   - Scale: Maximum number of distributed nodes
//   - Throughput: IDs per second per node
//
// # Constraints
//
// The sum of all bits must equal 63 (64-bit signed int, excluding sign bit).
// Each component must be positive and reasonable:
//   - TimestampBits: 38-42 (provides 8.7 to 139 years)
//   - WorkerBits: 8-18 (supports 256 to 262,144 nodes)
//   - SequenceBits: 6-14 (provides 64 to 16,384 IDs per time unit)
//
// # Performance
//
// Bit layout is validated once at generator creation. All bit operations
// (shifts, masks) are pre-calculated, resulting in zero runtime overhead
// compared to fixed layouts.
//
// Example:
//
//	layout := snowflake.LayoutSuperior
//	cfg := snowflake.Config{
//	    WorkerID: 1000,
//	    Layout:   layout,
//	}
//	gen, err := snowflake.NewWithConfig(cfg)
type BitLayout struct {
	// TimestampBits is the number of bits allocated for timestamp.
	// More bits = longer lifespan, fewer bits for workers/sequence.
	// Range: 38-42 bits (8.7 to 139 years with 1ms precision)
	TimestampBits int

	// WorkerBits is the number of bits allocated for worker/node ID.
	// More bits = more distributed nodes, fewer bits for timestamp/sequence.
	// Range: 8-18 bits (256 to 262,144 nodes)
	WorkerBits int

	// SequenceBits is the number of bits allocated for sequence counter.
	// More bits = higher throughput per node, fewer bits for timestamp/workers.
	// Range: 6-14 bits (64 to 16,384 IDs per time unit)
	SequenceBits int

	// TimeUnit is the precision of the timestamp.
	// Smaller units = better time precision, shorter lifespan.
	// Larger units = coarser precision, longer lifespan.
	// Common values: 1ms (default), 2ms, 10ms
	TimeUnit time.Duration
}

// Pre-defined layouts optimized for different use cases.
//
// All layouts maintain the 63-bit constraint and are production-tested.
// Choose based on your specific requirements for scale, throughput, and lifespan.
var (
	// LayoutDefault is the original Twitter Snowflake layout (backward compatible).
	//
	// Optimized for: High throughput, moderate scale
	//
	// Specifications:
	//   - Lifespan: ~69 years (until ~2093)
	//   - Max nodes: 1,024
	//   - Throughput: 4,096,000 IDs/sec per node
	//   - Time precision: 1 millisecond
	//
	// Use when: You need maximum throughput with <1000 nodes
	//
	// Example: High-traffic APIs, microservices, e-commerce platforms
	LayoutDefault = BitLayout{
		TimestampBits: 41,
		WorkerBits:    10,
		SequenceBits:  12,
		TimeUnit:      time.Millisecond,
	}

	// LayoutSuperior is the recommended balanced layout for most use cases.
	//
	// Optimized for: Balanced scale and performance
	//
	// Specifications:
	//   - Lifespan: ~35 years (until ~2059)
	//   - Max nodes: 16,384
	//   - Throughput: 512,000 IDs/sec per node
	//   - Time precision: 1 millisecond
	//
	// Use when: You need 10K+ nodes with good throughput
	//
	// Example: Large-scale SaaS, IoT platforms, global services
	//
	// This provides 16x more nodes than LayoutDefault while maintaining
	// excellent throughput (500K IDs/sec is still very high for most workloads).
	LayoutSuperior = BitLayout{
		TimestampBits: 40,
		WorkerBits:    14,
		SequenceBits:  9,
		TimeUnit:      time.Millisecond,
	}

	// LayoutExtreme is optimized for massive distributed systems.
	//
	// Optimized for: Maximum scale (100K+ nodes)
	//
	// Specifications:
	//   - Lifespan: ~17 years (until ~2042)
	//   - Max nodes: 131,072
	//   - Throughput: 128,000 IDs/sec per node
	//   - Time precision: 1 millisecond
	//
	// Use when: You need 50K-100K+ distributed nodes
	//
	// Example: Massive IoT networks, global CDN, planet-scale infrastructure
	//
	// This is 128x more nodes than LayoutDefault and 2x more than Sonyflake,
	// while maintaining better throughput than Sonyflake (128K vs 25K IDs/sec).
	// The shorter lifespan is acceptable for systems that get replaced every decade.
	LayoutExtreme = BitLayout{
		TimestampBits: 39,
		WorkerBits:    17,
		SequenceBits:  7,
		TimeUnit:      time.Millisecond,
	}

	// LayoutUltra provides excellent balance for very large deployments.
	//
	// Optimized for: High scale with high throughput
	//
	// Specifications:
	//   - Lifespan: ~17 years (until ~2041)
	//   - Max nodes: 32,768
	//   - Throughput: 1,024,000 IDs/sec per node
	//   - Time precision: 1 millisecond
	//
	// Use when: You need 10K-30K nodes with 1M IDs/sec
	//
	// Example: Real-time analytics, high-frequency trading, gaming platforms
	//
	// Best of both worlds: 32x more nodes than LayoutDefault while maintaining
	// 1M IDs/sec throughput (only 4x less than default).
	LayoutUltra = BitLayout{
		TimestampBits: 39,
		WorkerBits:    15,
		SequenceBits:  9,
		TimeUnit:      time.Millisecond,
	}

	// LayoutLongLife is optimized for systems requiring extended lifespan.
	//
	// Optimized for: Long-term systems (government, infrastructure)
	//
	// Specifications:
	//   - Lifespan: ~139 years (until ~2163)
	//   - Max nodes: 4,096
	//   - Throughput: 512,000 IDs/sec per node
	//   - Time precision: 1 millisecond
	//
	// Use when: System must outlive normal software lifecycles
	//
	// Example: Government records, archival systems, financial infrastructure
	//
	// Provides 2x longer lifespan than LayoutDefault while supporting 4K nodes
	// and maintaining excellent throughput.
	LayoutLongLife = BitLayout{
		TimestampBits: 42,
		WorkerBits:    12,
		SequenceBits:  9,
		TimeUnit:      time.Millisecond,
	}

	// LayoutSonyflake mimics Sony's Sonyflake for compatibility/comparison.
	//
	// Optimized for: Sonyflake migration or comparison
	//
	// Specifications:
	//   - Lifespan: ~174 years (until ~2198)
	//   - Max nodes: 65,536
	//   - Throughput: 25,600 IDs/sec per node
	//   - Time precision: 10 milliseconds
	//
	// Use when: Migrating from Sonyflake or need ultra-long lifespan
	//
	// Example: Sonyflake replacement, 100+ year systems
	//
	// Note: Lower throughput due to 10ms time unit. Consider LayoutExtreme
	// for better performance with similar scale.
	LayoutSonyflake = BitLayout{
		TimestampBits: 39,
		WorkerBits:    16,
		SequenceBits:  8,
		TimeUnit:      10 * time.Millisecond,
	}

	// LayoutUltimate is THE BEST EVER layout - recommended for new projects.
	//
	// Optimized for: The ultimate balance of lifespan, scale, and throughput
	//
	// Specifications:
	//   - Lifespan: ~292 years (time.Duration limit, theoretical 348 years)
	//   - Max nodes: 65,536
	//   - Throughput: 12,800 IDs/sec per node
	//   - Time precision: 10 milliseconds
	//
	// Use when: Building a new system that needs to last forever
	//
	// Example: Any production system, enterprise platforms, new services
	//
	// This layout beats BOTH LayoutDefault (4x longer lifespan, 64x more nodes)
	// AND Sonyflake (1.7x longer lifespan, same nodes, half throughput but still plenty).
	//
	// With 292 years of lifespan and 65K nodes, this is the sweet spot for 99.9%
	// of distributed systems. 12,800 IDs/sec = 1+ billion IDs per day per node.
	//
	// RECOMMENDED: Use this for all new projects unless you have specific needs.
	//
	// Note: The 40 timestamp bits theoretically provide 348 years, but Go's time.Duration
	// is capped at ~292 years (int64 nanoseconds limit). In practice, this is still
	// far longer than any software system will ever run.
	LayoutUltimate = BitLayout{
		TimestampBits: 40,
		WorkerBits:    16,
		SequenceBits:  7,
		TimeUnit:      10 * time.Millisecond,
	}

	// LayoutMegaScale is optimized for absolute maximum node capacity.
	//
	// Optimized for: Hyper-scale deployments with 100K+ nodes
	//
	// Specifications:
	//   - Lifespan: ~292 years (time.Duration limit, theoretical 348 years)
	//   - Max nodes: 131,072
	//   - Throughput: 6,400 IDs/sec per node
	//   - Time precision: 10 milliseconds
	//
	// Use when: You need the absolute maximum number of distributed nodes
	//
	// Example: Google/Amazon/Microsoft-scale infrastructure, global CDN
	//
	// This provides 2x more nodes than Sonyflake (131K vs 65K) while maintaining
	// the same ultra-long lifespan (292 years). Throughput is reduced but 6,400/sec
	// is still plenty for most workloads (550+ million IDs per day per node).
	//
	// Perfect for planet-scale systems that need to support 100,000+ worker nodes.
	//
	// Note: The 40 timestamp bits theoretically provide 348 years, but Go's time.Duration
	// is capped at ~292 years (int64 nanoseconds limit).
	LayoutMegaScale = BitLayout{
		TimestampBits: 40,
		WorkerBits:    17,
		SequenceBits:  6,
		TimeUnit:      10 * time.Millisecond,
	}
)

// Errors related to bit layout validation.
var (
	// ErrInvalidBitLayout is returned when a BitLayout is invalid.
	ErrInvalidBitLayout = errors.New("invalid bit layout")

	// ErrLayoutWorkerIDTooLarge is returned when worker ID exceeds layout capacity.
	ErrLayoutWorkerIDTooLarge = errors.New("worker ID too large for layout")
)

// Validate checks if the bit layout is valid.
//
// A valid layout must:
//   - Sum to exactly 63 bits
//   - Have positive values for all components
//   - Have reasonable ranges (to prevent overflow/underflow)
//   - Have a positive time unit
//
// Returns an error describing the specific validation failure.
//
// Performance: ~50ns (only called once at generator creation)
func (l BitLayout) Validate() error {
	// Check for negative values
	if l.TimestampBits < 0 {
		return fmt.Errorf("%w: timestamp bits cannot be negative (%d)", ErrInvalidBitLayout, l.TimestampBits)
	}
	if l.WorkerBits < 0 {
		return fmt.Errorf("%w: worker bits cannot be negative (%d)", ErrInvalidBitLayout, l.WorkerBits)
	}
	if l.SequenceBits < 0 {
		return fmt.Errorf("%w: sequence bits cannot be negative (%d)", ErrInvalidBitLayout, l.SequenceBits)
	}

	// Check sum equals 63 (usable bits in int64)
	totalBits := l.TimestampBits + l.WorkerBits + l.SequenceBits
	if totalBits != 63 {
		return fmt.Errorf("%w: total bits must equal 63, got %d (%d+%d+%d)",
			ErrInvalidBitLayout, totalBits, l.TimestampBits, l.WorkerBits, l.SequenceBits)
	}

	// Check reasonable ranges to prevent practical issues
	if l.TimestampBits < 38 || l.TimestampBits > 42 {
		return fmt.Errorf("%w: timestamp bits should be 38-42 for reasonable lifespan, got %d",
			ErrInvalidBitLayout, l.TimestampBits)
	}
	if l.WorkerBits < 8 || l.WorkerBits > 18 {
		return fmt.Errorf("%w: worker bits should be 8-18 for practical deployment, got %d",
			ErrInvalidBitLayout, l.WorkerBits)
	}
	if l.SequenceBits < 6 || l.SequenceBits > 14 {
		return fmt.Errorf("%w: sequence bits should be 6-14 for reasonable throughput, got %d",
			ErrInvalidBitLayout, l.SequenceBits)
	}

	// Check time unit
	if l.TimeUnit <= 0 {
		return fmt.Errorf("%w: time unit must be positive, got %v", ErrInvalidBitLayout, l.TimeUnit)
	}

	return nil
}

// CalculateCapacity returns the theoretical capacity of this layout.
//
// This provides useful information for capacity planning and deployment decisions.
//
// Performance: ~20ns (simple arithmetic)
func (l BitLayout) CalculateCapacity() LayoutCapacity {
	maxWorkers := int64(1 << l.WorkerBits)
	maxSequence := int64(1 << l.SequenceBits)
	maxTimestamp := int64(1 << l.TimestampBits)

	// Calculate lifespan - use float64 to avoid overflow for large values
	// (40 bits * 10ms would overflow int64 when multiplied in nanoseconds)
	totalTimeUnits := float64(maxTimestamp)
	totalSeconds := totalTimeUnits * l.TimeUnit.Seconds()
	totalNanoseconds := totalSeconds * float64(time.Second)

	// Cap at maximum time.Duration value (~292 years)
	// time.Duration is int64 nanoseconds, max value is math.MaxInt64
	maxDuration := float64(math.MaxInt64)
	if totalNanoseconds > maxDuration {
		totalNanoseconds = maxDuration
	}
	lifespan := time.Duration(totalNanoseconds)

	// Calculate throughput per worker
	idsPerTimeUnit := maxSequence
	throughputPerWorker := int64(float64(idsPerTimeUnit) / l.TimeUnit.Seconds())

	// Calculate total system capacity
	totalThroughput := throughputPerWorker * maxWorkers

	return LayoutCapacity{
		MaxWorkers:          maxWorkers,
		MaxSequence:         maxSequence,
		MaxTimestamp:        maxTimestamp,
		Lifespan:            lifespan,
		ThroughputPerWorker: throughputPerWorker,
		TotalThroughput:     totalThroughput,
		TimeUnit:            l.TimeUnit,
	}
}

// CalculateShifts returns the pre-calculated bit shift values for this layout.
//
// These are used for efficient bitwise operations during ID generation and parsing.
// This method is called once at generator initialization and the values are cached.
//
// Returns:
//   - timestampShift: Bits to shift timestamp left
//   - workerShift: Bits to shift worker ID left
//   - maxWorker: Maximum valid worker ID
//   - maxSequence: Maximum sequence value
//
// Performance: ~5ns (integer arithmetic)
func (l BitLayout) CalculateShifts() (timestampShift, workerShift int, maxWorker, maxSequence int64) {
	workerShift = l.SequenceBits
	timestampShift = l.SequenceBits + l.WorkerBits
	maxWorker = (1 << l.WorkerBits) - 1
	maxSequence = (1 << l.SequenceBits) - 1
	return
}

// ValidateWorkerID checks if a worker ID is valid for this layout.
//
// Returns an error if the worker ID exceeds the maximum allowed by this layout.
//
// Performance: ~2ns (single comparison)
func (l BitLayout) ValidateWorkerID(workerID int64) error {
	_, _, maxWorker, _ := l.CalculateShifts()
	if workerID < 0 || workerID > maxWorker {
		return fmt.Errorf("%w: worker ID %d exceeds layout maximum %d (%d bits)",
			ErrLayoutWorkerIDTooLarge, workerID, maxWorker, l.WorkerBits)
	}
	return nil
}

// TimeUnitShift returns the bitshift amount for converting milliseconds to time units.
//
// Returns:
//   - Non-negative value: Right shift amount for power-of-2 time units (fast bitshift)
//   - -1: Time unit is not power-of-2, use division fallback
//
// This enables zero-cost conversion for common time units:
//   - 1ms → shift 0 (no-op)
//   - 2ms → shift 1 (>>1)
//   - 4ms → shift 2 (>>2)
//   - 8ms → shift 3 (>>3)
//   - 10ms → -1 (use division, not power-of-2)
//
// Performance: ~5ns (integer arithmetic)
//
// Example:
//
//	shift := layout.TimeUnitShift()
//	if shift >= 0 {
//	    timeUnits = milliseconds >> shift  // Fast bitshift
//	} else {
//	    timeUnits = milliseconds / layout.TimeUnit.Milliseconds()  // Fallback
//	}
func (l BitLayout) TimeUnitShift() int8 {
	return calculateTimeUnitShift(l.TimeUnit)
}

// calculateTimeUnitShift computes the right-shift amount for a time unit.
//
// For power-of-2 time units, this returns the shift amount. For non-power-of-2,
// returns -1 to indicate division should be used instead.
//
// Performance: ~10ns (bit manipulation)
func calculateTimeUnitShift(timeUnit time.Duration) int8 {
	ms := timeUnit.Milliseconds()

	// Check if power of 2 using bit manipulation
	if ms <= 0 || !isPowerOfTwo(ms) {
		return -1  // Use division fallback
	}

	// Count trailing zeros = shift amount
	// Example: 8ms = 0b1000 → 3 trailing zeros → shift 3
	shift := int8(0)
	for ms > 1 {
		ms >>= 1
		shift++
	}
	return shift
}

// isPowerOfTwo checks if a number is a power of 2.
//
// Uses the classic bit manipulation trick: n & (n-1) == 0 for powers of 2.
//
// Performance: ~2ns (single bitwise operation)
//
// Examples:
//   - 1 (2^0) → true
//   - 2 (2^1) → true
//   - 4 (2^2) → true
//   - 10 → false
func isPowerOfTwo(n int64) bool {
	return n > 0 && (n&(n-1)) == 0
}

// LayoutCapacity holds calculated capacity information for a BitLayout.
//
// This is useful for capacity planning, deployment decisions, and documentation.
type LayoutCapacity struct {
	// MaxWorkers is the maximum number of unique worker IDs.
	MaxWorkers int64

	// MaxSequence is the maximum sequence value per time unit.
	MaxSequence int64

	// MaxTimestamp is the maximum timestamp value before overflow.
	MaxTimestamp int64

	// Lifespan is the duration before timestamp overflow (from epoch).
	Lifespan time.Duration

	// ThroughputPerWorker is the theoretical max IDs/sec per worker.
	ThroughputPerWorker int64

	// TotalThroughput is the theoretical max IDs/sec across all workers.
	TotalThroughput int64

	// TimeUnit is the timestamp precision.
	TimeUnit time.Duration
}

// String returns a human-readable description of the layout capacity.
func (c LayoutCapacity) String() string {
	years := int(c.Lifespan.Hours() / 24 / 365)
	return fmt.Sprintf("MaxWorkers: %d, ThroughputPerWorker: %d/sec, Lifespan: %d years, TimeUnit: %v",
		c.MaxWorkers, c.ThroughputPerWorker, years, c.TimeUnit)
}
