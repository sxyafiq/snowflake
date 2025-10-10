package main

import (
	"fmt"
	"log"

	"github.com/sxyafiq/snowflake"
)

func main() {
	fmt.Println("=== Snowflake ID Generator - Basic Example ===")

	// Generate ID using default generator (worker ID 0)
	id, err := snowflake.GenerateID()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Generated ID: %d\n", id.Int64())
	fmt.Printf("  Base58:  %s\n", id.Base58())
	fmt.Printf("  Base62:  %s (URL-safe)\n", id.Base62())
	fmt.Printf("  Hex:     %s\n", id.Hex())
	fmt.Printf("  Base32:  %s\n", id.Base32())
	fmt.Println()

	// Extract components
	timestamp, worker, sequence := id.Components()
	fmt.Println("ID Components:")
	fmt.Printf("  Timestamp: %d ms\n", timestamp)
	fmt.Printf("  Worker ID: %d\n", worker)
	fmt.Printf("  Sequence:  %d\n", sequence)
	fmt.Printf("  Generated: %v\n", id.Time())
	fmt.Printf("  Age:       %v\n", id.Age())
	fmt.Println()

	// Validation
	fmt.Printf("ID Valid: %v\n", id.IsValid())
}
