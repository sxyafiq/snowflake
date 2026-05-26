// Restart-safe, single-owner ID generation (single host).
//
// This example wires the two built-in safety mechanisms together using their
// file-based defaults — no external services required:
//
//   - FileClockGuard  protects against duplicate IDs when the wall clock
//     regresses across a process restart.
//   - FileWorkerLease protects against two live processes sharing a worker ID
//     (an OS flock, so the identity frees automatically on crash).
//
// Run it twice concurrently to see the lease reject the second process, and
// run it repeatedly to see the clock guard persist its high-water mark:
//
//	go run ./examples/hardening        # terminal 1 (holds the lease, then exits)
//	go run ./examples/hardening        # terminal 2 while 1 is running -> rejected
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sxyafiq/snowflake"
)

const workerID = 7

func main() {
	// A stable directory so guard/lease state survives across runs. In
	// production this would live on a durable volume owned by the instance.
	dataDir := filepath.Join(os.TempDir(), "snowflake-hardening-demo")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}
	fmt.Printf("State directory: %s\n\n", dataDir)

	gen, err := newHardenedGenerator(dataDir)
	if err != nil {
		// A second concurrent run lands here.
		if errors.Is(err, snowflake.ErrWorkerLeaseHeld) {
			log.Fatalf("worker ID %d is already in use by another live process — refusing to start", workerID)
		}
		log.Fatalf("create generator: %v", err)
	}
	// Releasing the lease lets a later process take over the worker ID. The
	// clock-guard state intentionally persists on disk for restart protection.
	defer func() {
		if cerr := gen.Close(); cerr != nil {
			log.Printf("close: %v", cerr)
		}
	}()

	fmt.Printf("Started generator for worker ID %d (lease acquired).\n\n", gen.WorkerID())

	demonstrateDuplicateRejection(dataDir)

	fmt.Println("Generating IDs:")
	for i := 0; i < 5; i++ {
		id, err := gen.GenerateID()
		if err != nil {
			log.Fatalf("generate: %v", err)
		}
		fmt.Printf("  %s  (base62=%s worker=%d)\n", id, id.Base62(), id.Worker())
	}

	fmt.Println("\nExiting cleanly — lease released, clock-guard mark retained for next start.")
}

// newHardenedGenerator builds a Generator with both file-based safeguards
// enabled. Both Config fields default to nil (disabled), so opting in is a
// two-line change with no effect on the hot path.
func newHardenedGenerator(dataDir string) (*snowflake.Generator, error) {
	cfg := snowflake.DefaultConfig(workerID)
	cfg.ClockGuard = snowflake.NewFileClockGuard(filepath.Join(dataDir, "clock.guard"))
	cfg.WorkerLease = snowflake.NewFileWorkerLease(filepath.Join(dataDir, "leases"))
	return snowflake.NewWithConfig(cfg)
}

// demonstrateDuplicateRejection shows, in-process, that a second generator for
// the same worker identity is refused while the first holds the lease.
func demonstrateDuplicateRejection(dataDir string) {
	cfg := snowflake.DefaultConfig(workerID)
	cfg.WorkerLease = snowflake.NewFileWorkerLease(filepath.Join(dataDir, "leases"))

	second, err := snowflake.NewWithConfig(cfg)
	switch {
	case errors.Is(err, snowflake.ErrWorkerLeaseHeld):
		fmt.Printf("Second generator for worker %d correctly rejected: %v\n\n", workerID, err)
	case err != nil:
		fmt.Printf("Second generator failed (unexpected): %v\n\n", err)
	default:
		// Should not happen while the first lease is held; clean up if it does.
		_ = second.Close()
		fmt.Printf("WARNING: second generator unexpectedly started for worker %d\n\n", workerID)
	}
}
