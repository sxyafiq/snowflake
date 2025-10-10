package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/sxyafiq/snowflake"
)

func main() {
	fmt.Println("=== Snowflake ID Generator - Advanced Example ===")

	// Create custom generator with worker ID 42
	cfg := snowflake.DefaultConfig(42)
	cfg.MaxClockBackward = 10 * time.Millisecond
	cfg.EnableMetrics = true

	gen, err := snowflake.NewWithConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Created generator with worker ID: %d\n\n", gen.WorkerID())

	// Generate IDs with context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Println("Generating 100 IDs...")
	for i := 0; i < 100; i++ {
		_, err := gen.GenerateIDWithContext(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Get metrics
	metrics := gen.GetMetrics()
	fmt.Println("\nGenerator Metrics:")
	fmt.Printf("  Generated:        %d IDs\n", metrics.Generated)
	fmt.Printf("  Clock backward:   %d events\n", metrics.ClockBackward)
	fmt.Printf("  Clock errors:     %d\n", metrics.ClockBackwardErr)
	fmt.Printf("  Sequence overflow: %d\n", metrics.SequenceOverflow)
	fmt.Printf("  Total wait time:  %d µs\n", metrics.WaitTimeUs)
	fmt.Println()

	// Demonstrate sharding
	id, _ := gen.GenerateID()
	numShards := int64(10)

	fmt.Printf("Sharding example (ID: %d):\n", id.Int64())
	fmt.Printf("  Shard (modulo):      %d/%d\n", id.Shard(numShards), numShards)
	fmt.Printf("  Shard (by worker):   %d/%d\n", id.ShardByWorker(numShards), numShards)
	fmt.Printf("  Shard (by hour):     %d\n", id.ShardByTime(1*time.Hour))
	fmt.Println()

	// Demonstrate ID comparison
	id1, _ := gen.GenerateID()
	time.Sleep(1 * time.Millisecond)
	id2, _ := gen.GenerateID()

	fmt.Println("ID Comparison:")
	fmt.Printf("  ID1: %s\n", id1.Base62())
	fmt.Printf("  ID2: %s\n", id2.Base62())
	fmt.Printf("  ID1 < ID2: %v\n", id1.Before(id2))
	fmt.Printf("  Compare result: %d\n", id1.Compare(id2))
	fmt.Println()

	// Demonstrate custom formatting
	fmt.Println("Custom Formatting:")
	fmt.Printf("  Format(\"hex\"):    %s\n", id.Format("hex"))
	fmt.Printf("  Format(\"base62\"): %s\n", id.Format("base62"))
	fmt.Printf("  Format(\"base58\"): %s\n", id.Format("base58"))
}
