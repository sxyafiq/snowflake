// Distributed Worker ID Coordination using Redis
//
// This example demonstrates how to dynamically assign and manage worker IDs
// across multiple instances using Redis for coordination.
//
// Features:
// - Dynamic worker ID leasing from a pool (0-1023)
// - Automatic lease renewal with heartbeat
// - Graceful worker ID release on shutdown
// - Dead worker detection and reclamation
//
// Usage:
//   # Start Redis
//   docker run -d -p 6379:6379 redis:alpine
//
//   # Start multiple workers
//   go run main.go &
//   go run main.go &
//   go run main.go &
//
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sxyafiq/snowflake"
)

const (
	WorkerPoolSize = 1024 // Total available worker IDs (0-1023)
	LeaseTTL       = 30 * time.Second
	RenewInterval  = 10 * time.Second
)

// WorkerCoordinator manages worker ID leasing via Redis
type WorkerCoordinator struct {
	redis    *redis.Client
	workerID int64
	stopCh   chan struct{}
}

// NewWorkerCoordinator creates a new coordinator
func NewWorkerCoordinator(redisAddr string) *WorkerCoordinator {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	return &WorkerCoordinator{
		redis:  client,
		stopCh: make(chan struct{}),
	}
}

// LeaseWorkerID attempts to lease an available worker ID
func (wc *WorkerCoordinator) LeaseWorkerID(ctx context.Context) (int64, error) {
	// Try each worker ID until we find an available one
	for id := int64(0); id < WorkerPoolSize; id++ {
		key := fmt.Sprintf("snowflake:worker:%d", id)

		// Try to acquire the worker ID with TTL
		acquired, err := wc.redis.SetNX(ctx, key, "claimed", LeaseTTL).Result()
		if err != nil {
			continue
		}

		if acquired {
			wc.workerID = id
			log.Printf("Successfully leased worker ID: %d", id)

			// Start background lease renewal
			go wc.renewLease(ctx, key)

			return id, nil
		}
	}

	return -1, fmt.Errorf("no available worker IDs in pool")
}

// renewLease periodically renews the worker ID lease
func (wc *WorkerCoordinator) renewLease(ctx context.Context, key string) {
	ticker := time.NewTicker(RenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Renew the lease
			err := wc.redis.Expire(ctx, key, LeaseTTL).Err()
			if err != nil {
				log.Printf("Failed to renew lease for %s: %v", key, err)
				return
			}
			log.Printf("Renewed lease for worker ID %d", wc.workerID)

		case <-wc.stopCh:
			// Gracefully release the worker ID
			log.Printf("Releasing worker ID %d", wc.workerID)
			wc.redis.Del(ctx, key)
			return

		case <-ctx.Done():
			return
		}
	}
}

// ReleaseWorkerID explicitly releases the worker ID
func (wc *WorkerCoordinator) ReleaseWorkerID(ctx context.Context) error {
	close(wc.stopCh)

	key := fmt.Sprintf("snowflake:worker:%d", wc.workerID)
	return wc.redis.Del(ctx, key).Err()
}

// GetActiveWorkers returns list of currently active worker IDs
func (wc *WorkerCoordinator) GetActiveWorkers(ctx context.Context) ([]int64, error) {
	var workers []int64

	// Scan for all worker keys
	iter := wc.redis.Scan(ctx, 0, "snowflake:worker:*", 0).Iterator()
	for iter.Next(ctx) {
		var workerID int64
		fmt.Sscanf(iter.Val(), "snowflake:worker:%d", &workerID)
		workers = append(workers, workerID)
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return workers, nil
}

// Close closes the Redis connection
func (wc *WorkerCoordinator) Close() error {
	return wc.redis.Close()
}

func main() {
	ctx := context.Background()

	fmt.Println("=== Distributed Worker ID Coordination Example ===\n")

	// Connect to Redis
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	coordinator := NewWorkerCoordinator(redisAddr)
	defer coordinator.Close()

	// Lease a worker ID
	workerID, err := coordinator.LeaseWorkerID(ctx)
	if err != nil {
		log.Fatalf("Failed to lease worker ID: %v", err)
	}

	fmt.Printf("Leased worker ID: %d\n", workerID)

	// Create Snowflake generator with leased worker ID
	gen, err := snowflake.New(workerID)
	if err != nil {
		log.Fatalf("Failed to create generator: %v", err)
	}

	fmt.Printf("Generator created with worker ID: %d\n\n", gen.WorkerID())

	// Show active workers
	activeWorkers, err := coordinator.GetActiveWorkers(ctx)
	if err != nil {
		log.Printf("Failed to get active workers: %v", err)
	} else {
		fmt.Printf("Active workers: %v\n\n", activeWorkers)
	}

	// Generate some IDs
	fmt.Println("Generating IDs...")
	for i := 0; i < 10; i++ {
		id, err := gen.GenerateID()
		if err != nil {
			log.Printf("Error generating ID: %v", err)
			continue
		}

		fmt.Printf("  ID %d: %s (Base62: %s, Worker: %d)\n",
			i+1, id, id.Base62(), id.Worker())

		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\nPress Ctrl+C to gracefully shutdown and release worker ID...")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")

	// Release worker ID
	if err := coordinator.ReleaseWorkerID(ctx); err != nil {
		log.Printf("Error releasing worker ID: %v", err)
	} else {
		fmt.Printf("Successfully released worker ID %d\n", workerID)
	}

	fmt.Println("Shutdown complete")
}
