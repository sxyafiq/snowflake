package snowflake

import (
	"context"
	"errors"
	"sync"
)

// ErrSequenceExhausted is returned by SequenceResolver.Next when no sequence
// numbers remain for the requested time unit.
//
// The Generator handles this internally by waiting for the next time unit and
// re-calling the resolver, so user code rarely sees it.
var ErrSequenceExhausted = errors.New("sequence exhausted for time unit")

// SequenceResolver allocates sequence numbers within a Snowflake ID's time
// unit.
//
// The Generator's default resolver is an in-process atomic counter — fast,
// but every node still needs a unique worker ID to avoid collisions. A custom
// SequenceResolver lets a fleet of Generators share a single sequence space
// (typically via Redis INCR or similar), so worker IDs can be set to a shared
// value or 0.
//
// A custom resolver MUST return values in [0, maxSequence] where maxSequence
// matches the Generator's BitLayout — otherwise the high bits of the sequence
// will collide with the worker-ID field. Use Layout.CalculateShifts() to
// derive maxSequence at setup time.
//
// Implementations MUST be safe for concurrent use.
type SequenceResolver interface {
	// Next returns the next sequence value for the given time unit.
	//
	// Returns ErrSequenceExhausted when no sequences remain for this time
	// unit; the Generator will then wait for the next time unit and call
	// Next again. Any other error is propagated to the caller of GenerateID.
	Next(ctx context.Context, timeUnit int64) (sequence int64, err error)
}

// NewLocalResolver returns the default in-process resolver: an atomic counter
// that resets to zero whenever timeUnit advances.
//
// This is what a Generator uses when Config.SequenceResolver is nil. It is
// exported so callers can compose with it (e.g. a cache-then-Redis fallback
// resolver) without re-implementing the basics.
func NewLocalResolver(maxSequence int64) SequenceResolver {
	return &localResolver{maxSequence: maxSequence}
}

// localResolver is the default in-process SequenceResolver: an atomic counter
// guarded by a mutex, resetting whenever timeUnit advances.
type localResolver struct {
	mu           sync.Mutex
	lastTimeUnit int64
	sequence     int64
	maxSequence  int64
}

func (r *localResolver) Next(_ context.Context, timeUnit int64) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if timeUnit != r.lastTimeUnit {
		r.lastTimeUnit = timeUnit
		r.sequence = 0
		return 0, nil
	}

	next := (r.sequence + 1) & r.maxSequence
	if next == 0 {
		return 0, ErrSequenceExhausted
	}
	r.sequence = next
	return next, nil
}
