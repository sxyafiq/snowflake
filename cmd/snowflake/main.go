// Snowflake CLI - Command-line tool for Snowflake ID generation and utilities
//
// Usage:
//   snowflake generate [flags]       Generate Snowflake IDs
//   snowflake parse <id>             Parse and inspect an ID
//   snowflake encode <id> <format>   Convert ID to different format
//   snowflake validate <id>          Validate an ID
//   snowflake bench                  Run performance benchmarks
//
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sxyafiq/snowflake"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "generate", "gen", "g":
		cmdGenerate(os.Args[2:])
	case "parse", "p":
		cmdParse(os.Args[2:])
	case "encode", "enc", "e":
		cmdEncode(os.Args[2:])
	case "validate", "val", "v":
		cmdValidate(os.Args[2:])
	case "bench", "benchmark", "b":
		cmdBench(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("snowflake CLI version %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Snowflake CLI - High-performance distributed unique ID generator

Usage:
  snowflake <command> [flags]

Commands:
  generate, gen, g      Generate Snowflake IDs
  parse, p              Parse and inspect an ID
  encode, enc, e        Convert ID between formats
  validate, val, v      Validate an ID structure
  bench, b              Run performance benchmarks
  version               Show version information
  help                  Show this help message

Examples:
  # Generate a single ID
  snowflake generate --worker 42

  # Generate 10 IDs in Base62 format
  snowflake generate --count 10 --format base62 --worker 42

  # Parse and inspect an ID
  snowflake parse 1234567890123456789

  # Convert ID to different format
  snowflake encode 1234567890123456789 base62

  # Validate an ID
  snowflake validate 1234567890123456789

  # Run benchmarks
  snowflake bench --duration 5s

For detailed help on a command:
  snowflake <command> --help

`)
}

// ============================================================================
// Generate Command
// ============================================================================

func cmdGenerate(args []string) {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	count := fs.Int("count", 1, "Number of IDs to generate")
	workerID := fs.Int64("worker", 0, "Worker ID (0-1023)")
	format := fs.String("format", "decimal", "Output format: decimal, base32, base58, base62, hex")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	batch := fs.Bool("batch", false, "Use batch generation for better performance")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: snowflake generate [flags]

Generate one or more Snowflake IDs.

Flags:
  --count N          Number of IDs to generate (default: 1)
  --worker N         Worker ID 0-1023 (default: 0)
  --format FORMAT    Output format: decimal, base32, base58, base62, hex (default: decimal)
  --json             Output as JSON with full details
  --batch            Use batch generation (faster for large counts)

Examples:
  snowflake generate --worker 42
  snowflake generate --count 1000 --format base62 --worker 42
  snowflake generate --json --worker 5
`)
	}

	fs.Parse(args)

	// Create generator
	gen, err := snowflake.New(*workerID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating generator: %v\n", err)
		os.Exit(1)
	}

	// Generate IDs
	var ids []snowflake.ID
	var genErr error
	startTime := time.Now()
	ctx := context.Background()

	if *batch && *count > 1 {
		// Use batch generation
		ids, genErr = gen.GenerateBatch(ctx, *count)
		if genErr != nil {
			fmt.Fprintf(os.Stderr, "Error generating batch: %v\n", genErr)
			os.Exit(1)
		}
	} else {
		// Generate individually
		ids = make([]snowflake.ID, *count)
		for i := 0; i < *count; i++ {
			ids[i], genErr = gen.GenerateID()
			if genErr != nil {
				fmt.Fprintf(os.Stderr, "Error generating ID: %v\n", genErr)
				os.Exit(1)
			}
		}
	}

	duration := time.Since(startTime)

	// Output results
	if *jsonOutput {
		outputJSON(ids, duration, *workerID)
	} else {
		for _, id := range ids {
			fmt.Println(formatID(id, *format))
		}

		// Show performance stats for large batches
		if *count > 100 {
			rate := float64(*count) / duration.Seconds()
			fmt.Fprintf(os.Stderr, "\nGenerated %d IDs in %v (%.0f IDs/sec)\n",
				*count, duration, rate)
		}
	}
}

func formatID(id snowflake.ID, format string) string {
	switch strings.ToLower(format) {
	case "base32", "b32":
		return id.Base32()
	case "base58", "b58":
		return id.Base58()
	case "base62", "b62":
		return id.Base62()
	case "hex", "x":
		return id.Hex()
	case "binary", "bin":
		return id.Base2()
	default:
		return id.String()
	}
}

func outputJSON(ids []snowflake.ID, duration time.Duration, workerID int64) {
	type IDInfo struct {
		ID        string    `json:"id"`
		Base62    string    `json:"base62"`
		Hex       string    `json:"hex"`
		Timestamp time.Time `json:"timestamp"`
		Worker    int64     `json:"worker"`
		Sequence  int64     `json:"sequence"`
	}

	type Output struct {
		Count      int        `json:"count"`
		WorkerID   int64      `json:"worker_id"`
		Duration   string     `json:"duration"`
		RatePerSec float64    `json:"rate_per_sec"`
		IDs        []IDInfo   `json:"ids"`
	}

	infos := make([]IDInfo, len(ids))
	for i, id := range ids {
		ts, worker, seq := id.Components()
		infos[i] = IDInfo{
			ID:        id.String(),
			Base62:    id.Base62(),
			Hex:       id.Hex(),
			Timestamp: time.UnixMilli(ts),
			Worker:    worker,
			Sequence:  seq,
		}
	}

	rate := float64(len(ids)) / duration.Seconds()
	output := Output{
		Count:      len(ids),
		WorkerID:   workerID,
		Duration:   duration.String(),
		RatePerSec: rate,
		IDs:        infos,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(output)
}

// ============================================================================
// Parse Command
// ============================================================================

func cmdParse(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: snowflake parse <id>\n")
		fmt.Fprintf(os.Stderr, "\nParse and inspect a Snowflake ID.\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  snowflake parse 1234567890123456789\n")
		fmt.Fprintf(os.Stderr, "  snowflake parse 7n42dgm5tflk  # Base62 format\n")
		os.Exit(1)
	}

	idStr := args[0]

	// Try to parse in different formats
	var id snowflake.ID
	var err error

	// Try decimal first
	id, err = snowflake.ParseString(idStr)
	if err != nil {
		// Try Base62
		id, err = snowflake.ParseBase62(idStr)
		if err != nil {
			// Try Base58
			id, err = snowflake.ParseBase58(idStr)
			if err != nil {
				// Try Hex
				id, err = snowflake.ParseHex(idStr)
				if err != nil {
					// Try Base32
					id, err = snowflake.ParseBase32(idStr)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: Unable to parse ID '%s'\n", idStr)
						os.Exit(1)
					}
				}
			}
		}
	}

	// Extract components
	ts, worker, seq := id.Components()
	timestamp := time.UnixMilli(ts)
	age := id.Age()

	// Print detailed information
	fmt.Printf("Snowflake ID: %s\n", id)
	fmt.Printf("\n")
	fmt.Printf("Components:\n")
	fmt.Printf("  Timestamp:  %s (%d ms since epoch)\n", timestamp.Format(time.RFC3339), ts)
	fmt.Printf("  Worker ID:  %d\n", worker)
	fmt.Printf("  Sequence:   %d\n", seq)
	fmt.Printf("\n")
	fmt.Printf("Encodings:\n")
	fmt.Printf("  Decimal:    %s\n", id.String())
	fmt.Printf("  Base62:     %s\n", id.Base62())
	fmt.Printf("  Base58:     %s\n", id.Base58())
	fmt.Printf("  Base32:     %s\n", id.Base32())
	fmt.Printf("  Hex:        %s\n", id.Hex())
	fmt.Printf("\n")
	fmt.Printf("Age:          %v\n", age.Round(time.Millisecond))
	fmt.Printf("Valid:        %v\n", id.IsValid())
}

// ============================================================================
// Encode Command
// ============================================================================

func cmdEncode(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: snowflake encode <id> <format>\n")
		fmt.Fprintf(os.Stderr, "\nConvert a Snowflake ID to a different encoding format.\n")
		fmt.Fprintf(os.Stderr, "\nFormats:\n")
		fmt.Fprintf(os.Stderr, "  decimal, dec       Decimal string\n")
		fmt.Fprintf(os.Stderr, "  base62, b62        URL-safe Base62\n")
		fmt.Fprintf(os.Stderr, "  base58, b58        Bitcoin-style Base58\n")
		fmt.Fprintf(os.Stderr, "  base32, b32        z-base-32\n")
		fmt.Fprintf(os.Stderr, "  hex, x             Hexadecimal\n")
		fmt.Fprintf(os.Stderr, "  binary, bin        Binary string\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  snowflake encode 1234567890123456789 base62\n")
		fmt.Fprintf(os.Stderr, "  snowflake encode 7n42dgm5tflk decimal\n")
		os.Exit(1)
	}

	idStr := args[0]
	format := args[1]

	// Parse ID (try multiple formats)
	id, err := parseIDFlexible(idStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Unable to parse ID '%s': %v\n", idStr, err)
		os.Exit(1)
	}

	// Output in requested format
	fmt.Println(formatID(id, format))
}

func parseIDFlexible(idStr string) (snowflake.ID, error) {
	// Try decimal first
	id, err := snowflake.ParseString(idStr)
	if err == nil {
		return id, nil
	}

	// Try Base62
	id, err = snowflake.ParseBase62(idStr)
	if err == nil {
		return id, nil
	}

	// Try Base58
	id, err = snowflake.ParseBase58(idStr)
	if err == nil {
		return id, nil
	}

	// Try Hex
	id, err = snowflake.ParseHex(idStr)
	if err == nil {
		return id, nil
	}

	// Try Base32
	return snowflake.ParseBase32(idStr)
}

// ============================================================================
// Validate Command
// ============================================================================

func cmdValidate(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: snowflake validate <id>\n")
		fmt.Fprintf(os.Stderr, "\nValidate the structure of a Snowflake ID.\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  snowflake validate 1234567890123456789\n")
		os.Exit(1)
	}

	idStr := args[0]

	// Parse ID
	id, err := parseIDFlexible(idStr)
	if err != nil {
		fmt.Printf("INVALID: Unable to parse ID '%s'\n", idStr)
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Validate structure
	if !id.IsValid() {
		fmt.Printf("INVALID: ID structure is invalid\n")

		// Show why it's invalid
		ts, worker, seq := id.Components()
		fmt.Printf("\nComponents:\n")
		fmt.Printf("  Timestamp:  %d ms since epoch\n", ts)
		fmt.Printf("  Worker ID:  %d (valid range: 0-1023)\n", worker)
		fmt.Printf("  Sequence:   %d (valid range: 0-4095)\n", seq)

		if ts <= snowflake.Epoch {
			fmt.Printf("\n  Error: Timestamp is before or equal to epoch\n")
		}
		if worker < 0 || worker > 1023 {
			fmt.Printf("\n  Error: Worker ID out of range\n")
		}
		if seq < 0 || seq > 4095 {
			fmt.Printf("\n  Error: Sequence out of range\n")
		}

		os.Exit(1)
	}

	fmt.Printf("VALID: ID structure is valid\n")

	// Show components
	ts, worker, seq := id.Components()
	timestamp := time.UnixMilli(ts)
	fmt.Printf("\nComponents:\n")
	fmt.Printf("  Timestamp:  %s\n", timestamp.Format(time.RFC3339))
	fmt.Printf("  Worker ID:  %d\n", worker)
	fmt.Printf("  Sequence:   %d\n", seq)
	fmt.Printf("  Age:        %v\n", id.Age().Round(time.Millisecond))
}

// ============================================================================
// Benchmark Command
// ============================================================================

func cmdBench(args []string) {
	fs := flag.NewFlagSet("bench", flag.ExitOnError)
	duration := fs.Duration("duration", 3*time.Second, "Benchmark duration")
	workerID := fs.Int64("worker", 0, "Worker ID (0-1023)")
	batchSize := fs.Int("batch", 100, "Batch size for batch generation test")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: snowflake bench [flags]

Run performance benchmarks for ID generation.

Flags:
  --duration D      Benchmark duration (default: 3s)
  --worker N        Worker ID 0-1023 (default: 0)
  --batch N         Batch size for batch test (default: 100)

Examples:
  snowflake bench --duration 5s
  snowflake bench --worker 42 --duration 10s
`)
	}

	fs.Parse(args)

	// Create generator
	gen, err := snowflake.New(*workerID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating generator: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Running benchmarks (duration: %v, worker: %d)\n\n", *duration, *workerID)
	ctx := context.Background()

	// Benchmark 1: Single ID generation
	fmt.Printf("1. Single ID Generation:\n")
	count := 0
	start := time.Now()
	deadline := start.Add(*duration)
	for time.Now().Before(deadline) {
		_, err := gen.GenerateID()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating ID: %v\n", err)
			break
		}
		count++
	}
	elapsed := time.Since(start)
	rate := float64(count) / elapsed.Seconds()
	nsPerOp := float64(elapsed.Nanoseconds()) / float64(count)

	fmt.Printf("   Generated:      %d IDs\n", count)
	fmt.Printf("   Duration:       %v\n", elapsed)
	fmt.Printf("   Rate:           %.0f IDs/sec (%.0f ns/op)\n", rate, nsPerOp)
	fmt.Printf("\n")

	// Benchmark 2: Batch generation
	fmt.Printf("2. Batch Generation (batch size: %d):\n", *batchSize)
	count = 0
	batchCount := 0
	start = time.Now()
	deadline = start.Add(*duration)
	for time.Now().Before(deadline) {
		ids, err := gen.GenerateBatch(ctx, *batchSize)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating batch: %v\n", err)
			break
		}
		count += len(ids)
		batchCount++
	}
	elapsed = time.Since(start)
	rate = float64(count) / elapsed.Seconds()
	nsPerOp = float64(elapsed.Nanoseconds()) / float64(count)

	fmt.Printf("   Generated:      %d IDs in %d batches\n", count, batchCount)
	fmt.Printf("   Duration:       %v\n", elapsed)
	fmt.Printf("   Rate:           %.0f IDs/sec (%.0f ns/op)\n", rate, nsPerOp)
	fmt.Printf("   Avg batch time: %.2f ms\n", float64(elapsed.Milliseconds())/float64(batchCount))
	fmt.Printf("\n")

	// Benchmark 3: Encoding performance
	fmt.Printf("3. Encoding Performance (1000 operations):\n")
	testID, err := gen.GenerateID()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating test ID: %v\n", err)
		os.Exit(1)
	}

	encodingTests := []struct {
		name string
		fn   func() string
	}{
		{"Decimal", func() string { return testID.String() }},
		{"Base62", func() string { return testID.Base62() }},
		{"Base58", func() string { return testID.Base58() }},
		{"Base32", func() string { return testID.Base32() }},
		{"Hex", func() string { return testID.Hex() }},
	}

	for _, test := range encodingTests {
		start := time.Now()
		for i := 0; i < 1000; i++ {
			_ = test.fn()
		}
		elapsed := time.Since(start)
		nsPerOp := float64(elapsed.Nanoseconds()) / 1000
		fmt.Printf("   %-8s %6.0f ns/op\n", test.name+":", nsPerOp)
	}

	fmt.Printf("\nBenchmark complete!\n")
}
