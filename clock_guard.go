package snowflake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ClockGuard persists a per-(worker, layout, epoch) high-water timestamp so a
// Generator can detect — across process restarts — that the wall clock has
// regressed below IDs it already emitted.
//
// # Why this exists
//
// Within a single process the Generator is anchored to the monotonic clock, so
// the timestamp it derives never moves backward regardless of NTP steps, leap
// seconds, or manual clock changes. That protection does NOT survive a restart:
// the monotonic anchor is re-seeded from the wall clock at construction, and
// the in-memory last-emitted timestamp resets. If the wall clock is now earlier
// than IDs minted before the restart (e.g. a clock that was running fast gets
// corrected backward), a fresh Generator with the same worker ID can mint
// duplicate timestamp/worker/sequence tuples.
//
// A ClockGuard closes that gap by durably recording a high-water mark. On
// startup the Generator compares the current clock against the persisted value
// and either waits for the clock to catch up or refuses to start.
//
// # Scope
//
// A ClockGuard protects against SEQUENTIAL restarts of the same logical worker.
// It does NOT protect against two live processes sharing a worker ID — that is
// a worker-ID-assignment problem requiring an exclusive lease (Redis/etcd/file
// lock), which is intentionally out of scope here.
//
// # Contract
//
// Implementations MUST be safe for concurrent use. Load returning found=false
// means nothing was ever persisted for the key (first-ever startup) and MUST
// NOT be reported as an error.
type ClockGuard interface {
	// Load returns the persisted high-water timestamp (in the layout's time
	// units) for key, or found=false if nothing was ever stored.
	Load(ctx context.Context, key GuardKey) (highWater int64, found bool, err error)

	// Store records highWater as the new persisted value for key. The Generator
	// calls this from the generation path, throttled to roughly once per
	// ClockGuardInterval, so a moderately slow Store is acceptable.
	Store(ctx context.Context, key GuardKey, highWater int64) error
}

// GuardKey scopes a persisted high-water mark to a single generator identity.
//
// A high-water value is only comparable within the same key: different layouts
// use different time-unit granularity and bit positions, so a value persisted
// under one layout is meaningless under another. Worker IDs partition the ID
// space, so each worker guards only its own regression.
type GuardKey struct {
	WorkerID int64
	Epoch    int64

	// Layout fingerprint — distinguishes incompatible bit/time-unit layouts.
	TimestampBits uint8
	WorkerBits    uint8
	SequenceBits  uint8
	TimeUnitMs    int64
}

// String returns a stable, collision-free key suitable for use as a map or
// file key. Every scoping dimension is included so incompatible layouts never
// share a slot.
func (k GuardKey) String() string {
	return fmt.Sprintf("w%d-e%d-t%d-k%d-s%d-u%d",
		k.WorkerID, k.Epoch, k.TimestampBits, k.WorkerBits, k.SequenceBits, k.TimeUnitMs)
}

// FileClockGuard is the default ClockGuard: it stores high-water marks as a
// JSON object on local disk, keyed by GuardKey.String(). It is pure stdlib (no
// external dependencies) and writes atomically (temp file + rename) so a crash
// mid-write never corrupts the state file.
//
// FileClockGuard suits single-host deployments and local development. For a
// fleet sharing storage, implement ClockGuard over a networked store (the
// interface is intentionally minimal). FileClockGuard does not provide
// cross-process locking; point each worker at its own file, or accept that the
// last writer wins when several share one file.
type FileClockGuard struct {
	mu   sync.Mutex
	path string
}

// NewFileClockGuard returns a FileClockGuard backed by the file at path. The
// file and its parent directory are created lazily on first Store; a missing
// file is treated as "no state yet" by Load.
func NewFileClockGuard(path string) *FileClockGuard {
	return &FileClockGuard{path: path}
}

// Load reads the high-water mark for key. A missing file yields found=false
// with no error, so a first-ever startup is not a hard failure.
func (g *FileClockGuard) Load(_ context.Context, key GuardKey) (int64, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	marks, err := g.read()
	if err != nil {
		return 0, false, err
	}
	v, ok := marks[key.String()]
	return v, ok, nil
}

// Store atomically records highWater for key, preserving any other keys already
// present in the file.
func (g *FileClockGuard) Store(_ context.Context, key GuardKey, highWater int64) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	marks, err := g.read()
	if err != nil {
		return err
	}
	marks[key.String()] = highWater
	return g.writeAtomic(marks)
}

// read loads the on-disk map, returning an empty map when the file does not
// exist yet. Caller must hold g.mu.
func (g *FileClockGuard) read() (map[string]int64, error) {
	data, err := os.ReadFile(g.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]int64{}, nil
		}
		return nil, fmt.Errorf("clock guard read %s: %w", g.path, err)
	}
	if len(data) == 0 {
		return map[string]int64{}, nil
	}
	marks := map[string]int64{}
	if err := json.Unmarshal(data, &marks); err != nil {
		return nil, fmt.Errorf("clock guard parse %s: %w", g.path, err)
	}
	return marks, nil
}

// writeAtomic writes marks to a temp file in the same directory and renames it
// over the target, so readers never observe a partially written file. Caller
// must hold g.mu.
func (g *FileClockGuard) writeAtomic(marks map[string]int64) error {
	dir := filepath.Dir(g.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("clock guard mkdir %s: %w", dir, err)
	}

	data, err := json.Marshal(marks)
	if err != nil {
		return fmt.Errorf("clock guard marshal: %w", err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(g.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("clock guard temp file: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we bail out before the rename succeeds.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("clock guard write: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("clock guard fsync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("clock guard close: %w", err)
	}
	if err := os.Rename(tmpName, g.path); err != nil {
		return fmt.Errorf("clock guard rename: %w", err)
	}
	return nil
}

// guardKeyForConfig builds the GuardKey identifying a generator with the given
// configuration. Layout fields are read after Validate has defaulted them.
func guardKeyForConfig(cfg Config) GuardKey {
	return GuardKey{
		WorkerID:      cfg.WorkerID,
		Epoch:         cfg.Epoch,
		TimestampBits: uint8(cfg.Layout.TimestampBits),
		WorkerBits:    uint8(cfg.Layout.WorkerBits),
		SequenceBits:  uint8(cfg.Layout.SequenceBits),
		TimeUnitMs:    cfg.Layout.TimeUnit.Milliseconds(),
	}
}

// leaseUnits converts a persistence interval into a number of layout time units,
// rounding up and adding one unit of margin so the persisted high-water mark is
// always strictly ahead of any timestamp emittable before the next persist.
func leaseUnits(interval, timeUnit time.Duration) int64 {
	if interval <= 0 {
		interval = DefaultClockGuardInterval
	}
	units := int64(interval / timeUnit)
	if interval%timeUnit != 0 {
		units++
	}
	if units < 1 {
		units = 1
	}
	return units + 1
}

// recoverFromGuard runs the startup half of the clock-guard protocol. It loads
// the persisted high-water mark and, if the current clock has regressed below
// it, waits up to ClockGuardMaxWait for the clock to catch up — failing with a
// ClockError if it does not. It then persists a fresh mark ahead of the clock
// so a crash immediately after startup is still covered.
//
// Called once from NewWithConfig, before the generator is returned, so the
// blocking wait never affects the hot path.
func (g *Generator) recoverFromGuard(cfg Config) error {
	g.guardKey = guardKeyForConfig(cfg)
	g.guardLeaseUnits = leaseUnits(cfg.ClockGuardInterval, g.timeUnit)
	g.guardMaxWait = cfg.ClockGuardMaxWait
	ctx := context.Background()

	hw, found, err := g.guard.Load(ctx, g.guardKey)
	if err != nil {
		return fmt.Errorf("clock guard load: %w", err)
	}

	current := g.currentTimestamp()
	if found && current <= hw {
		poll := g.timeUnit
		if poll > time.Millisecond {
			poll = time.Millisecond
		}
		deadline := time.Now().Add(g.guardMaxWait)
		for current <= hw {
			if time.Now().After(deadline) {
				// Drift exceeds tolerance: refuse to start rather than risk
				// minting IDs at timestamps already used before the restart.
				return newClockError(current, hw, g.guardMaxWait.Milliseconds(), g.workerID, false)
			}
			time.Sleep(poll)
			current = g.currentTimestamp()
		}
	}

	// Treat the persisted mark as a floor so the first emitted ID is strictly
	// greater than anything emitted before the restart.
	if found && hw > g.lastTimestamp {
		g.lastTimestamp = hw
	}

	target := current + g.guardLeaseUnits
	if err := g.guard.Store(ctx, g.guardKey, target); err != nil {
		return fmt.Errorf("clock guard store: %w", err)
	}
	g.guardNextPersist = target
	return nil
}

// persistGuard enforces the invariant "persisted high-water >= timestamp"
// before the caller emits an ID at timestamp. It writes the mark ahead by
// guardLeaseUnits and is throttled to roughly once per ClockGuardInterval, so
// the per-ID cost is a single comparison in the common case.
//
// It fails closed: if the mark cannot be persisted, the ID is not emitted,
// because a guard that silently stops persisting offers no protection. Caller
// must hold g.mu.
func (g *Generator) persistGuard(ctx context.Context, timestamp int64) error {
	if g.guard == nil || timestamp < g.guardNextPersist {
		return nil
	}
	target := timestamp + g.guardLeaseUnits
	if err := g.guard.Store(ctx, g.guardKey, target); err != nil {
		return fmt.Errorf("clock guard store: %w", err)
	}
	g.guardNextPersist = target
	return nil
}
