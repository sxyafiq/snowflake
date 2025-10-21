# Snowflake

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/sxyafiq/snowflake)](https://goreportcard.com/report/github.com/sxyafiq/snowflake)

A high-performance, production-ready Snowflake ID generator for distributed systems. Generates 64-bit unique, time-ordered IDs at 2.2M/sec with sub-microsecond latency.

**Key Features:** Monotonic clock · Zero dependencies · 11 encoding formats · Built-in observability · Production-tested

---

## Table of Contents

- [Why Snowflake?](#why-snowflake)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Features](#features)
- [Usage Examples](#usage-examples)
- [API Reference](#api-reference)
- [Performance](#performance)
- [Production Deployment](#production-deployment)
- [Architecture](#architecture)
- [FAQ](#faq)
- [Contributing](#contributing)
- [License](#license)

---

## Why Snowflake?

Distributed systems need unique IDs that are:
- **Sortable by time** - Essential for time-series data and efficient indexing
- **Globally unique** - No coordination needed between nodes
- **Fast to generate** - Low latency for high-throughput systems
- **Compact** - Efficient storage and network transfer

This implementation provides:

| Requirement | Solution |
|-------------|----------|
| **Reliability** | Monotonic clock resistant to NTP drift |
| **Performance** | 2.2M IDs/sec, <500ns latency, zero allocations |
| **Observability** | Built-in metrics for production monitoring |
| **Flexibility** | 11 encoding formats for different use cases |
| **Production-Ready** | Context support, error handling, comprehensive tests |

**vs. UUID v4:** Time-ordered, sequential, 50% more compact when encoded
**vs. Auto-increment:** Distributed, no single point of failure, globally unique

---

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/sxyafiq/snowflake"
)

func main() {
    // Generate an ID (uses default generator, worker ID 0)
    id, err := snowflake.GenerateID()
    if err != nil {
        panic(err)
    }

    fmt.Printf("ID: %d\n", id.Int64())           // 1234567890123456789
    fmt.Printf("URL-safe: %s\n", id.Base62())    // 7n42dgm5tflk
    fmt.Printf("Time: %v\n", id.Time())          // 2024-01-15 10:30:45
}
```

**For distributed systems** (multiple nodes):

```go
// Each node must have a unique worker ID (0-1023)
gen, _ := snowflake.New(workerID)
id, _ := gen.GenerateID()
```

**For new projects** (use LayoutUltimate):

```go
// 292 years lifespan, 65K nodes, 12.8K IDs/sec per node
cfg := snowflake.DefaultConfig(workerID)
cfg.Layout = snowflake.LayoutUltimate
gen, _ := snowflake.NewWithConfig(cfg)
id, _ := gen.GenerateID()
```

---

## Installation

```bash
go get github.com/sxyafiq/snowflake
```

**Requirements:**
- Go 1.21 or higher
- No external dependencies

### Command-Line Tool

A powerful CLI tool is included for quick ID generation, parsing, and utilities:

```bash
# Install the CLI
go install ./cmd/snowflake

# Generate IDs
snowflake generate --worker 42
snowflake generate --count 10 --format base62 --worker 42

# Parse and inspect IDs
snowflake parse 1234567890123456789

# Convert between formats
snowflake encode 1234567890123456789 base62

# Run benchmarks
snowflake bench --duration 5s --worker 42
```

See [cmd/snowflake/README.md](./cmd/snowflake/README.md) for full CLI documentation.

---

## Features

### Core Capabilities

**ID Generation**
- **Monotonic clock** - Resistant to NTP adjustments and leap seconds
- **High performance** - 2.2M IDs/sec per worker, ~450ns latency
- **Thread-safe** - Safe for concurrent use with minimal lock contention
- **Zero allocations** - No heap allocations in hot path

**Reliability**
- **Clock drift tolerance** - Configurable tolerance (default 5ms)
- **Context support** - Graceful cancellation for timeouts
- **Error handling** - Clear error messages for debugging
- **Metrics** - Built-in counters for monitoring

**Encoding Formats** (11 total)
- **Int64** - Native storage format
- **Base62** - URL-safe, compact (recommended for APIs)
- **Base58** - Bitcoin-style, no ambiguous characters
- **Base32/36/64** - Standard encodings
- **Hex** - Debugging and low-level protocols
- **Binary** - Network protocols (8 bytes)

**Integration**
- **SQL databases** - `sql.Scanner` and `driver.Valuer`
- **JSON** - Safe marshaling (string format for JavaScript)
- **XML/YAML/TOML** - Text marshaling support

**Advanced**
- **Component extraction** - Extract timestamp, worker ID, sequence
- **Validation** - Verify ID structure and integrity
- **Comparison** - Before/After/Equal operations
- **Sharding** - Calculate shard/partition for distribution

### Configurable Bit Layouts

Choose the optimal trade-off between **lifespan**, **scale**, and **throughput**:

| Layout | Lifespan | Max Nodes | Throughput/Node | Use Case |
|--------|----------|-----------|-----------------|----------|
| **LayoutDefault** | 69 years | 1,024 | 4.1M IDs/sec | High throughput, <1K nodes |
| **LayoutSuperior** | 35 years | 16,384 | 512K IDs/sec | Balanced (recommended) |
| **LayoutExtreme** | 17 years | 131,072 | 128K IDs/sec | Massive scale (100K+ nodes) |
| **LayoutUltra** | 17 years | 32,768 | 1M IDs/sec | High throughput + scale |
| **LayoutLongLife** | 139 years | 4,096 | 512K IDs/sec | Long-term systems |
| **LayoutSonyflake** | 174 years | 65,536 | 25.6K IDs/sec | Sonyflake compatibility |
| **LayoutUltimate** ⭐ | 292 years | 65,536 | 12.8K IDs/sec | **Best for new projects** |
| **LayoutMegaScale** | 292 years | 131,072 | 6.4K IDs/sec | Maximum node capacity |

**Recommendation:** Use `LayoutUltimate` for new projects - it provides the longest lifespan with excellent scale.

**Example:**
```go
cfg := snowflake.DefaultConfig(workerID)
cfg.Layout = snowflake.LayoutUltimate  // 292 years, 65K nodes
gen, _ := snowflake.NewWithConfig(cfg)
```

---

## Usage Examples

### Basic Usage

```go
// Default generator (worker ID 0)
id, err := snowflake.GenerateID()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Generated: %d\n", id.Int64())
fmt.Printf("Timestamp: %v\n", id.Time())
fmt.Printf("Worker: %d\n", id.Worker())
fmt.Printf("Sequence: %d\n", id.Sequence())
```

### Custom Configuration

```go
cfg := snowflake.Config{
    WorkerID:         42,                        // Unique per node
    Epoch:            snowflake.Epoch,           // 2024-01-01
    MaxClockBackward: 10 * time.Millisecond,    // Clock drift tolerance
    EnableMetrics:    true,                      // Enable observability
}

gen, err := snowflake.NewWithConfig(cfg)
if err != nil {
    log.Fatal(err)
}

id, _ := gen.GenerateID()
```

### With Context (Timeout Support)

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

id, err := gen.GenerateIDWithContext(ctx)
if err == snowflake.ErrContextCanceled {
    log.Println("ID generation timed out")
} else if err == snowflake.ErrClockMovedBack {
    log.Println("Clock drift exceeded tolerance")
}
```

### Encoding & Parsing

```go
id, _ := snowflake.GenerateID()

// Encode to different formats
base62 := id.Base62()     // "7n42dgm5tflk" (URL-safe)
base58 := id.Base58()     // "BukQL2gPvMW" (no ambiguous chars)
hex := id.Hex()           // "112210f47de98115"

// Parse from any format
id1, _ := snowflake.ParseString("1234567890123456789")
id2, _ := snowflake.ParseBase62("7n42dgm5tflk")
id3, _ := snowflake.ParseHex("112210f47de98115")
```

### Database Integration

```go
type User struct {
    ID    snowflake.ID `db:"id" json:"id"`
    Name  string       `db:"name" json:"name"`
}

// Insert (ID automatically converts to int64)
db.Exec("INSERT INTO users (id, name) VALUES ($1, $2)", user.ID, user.Name)

// Query (ID automatically scans from int64)
db.QueryRow("SELECT id, name FROM users WHERE id = $1", userID).
    Scan(&user.ID, &user.Name)

// JSON (marshals as string for JavaScript safety)
// {"id": "1234567890123456789", "name": "Alice"}
```

### Sharding & Partitioning

```go
id, _ := snowflake.GenerateID()

// Simple modulo sharding (0-9)
shard := id.Shard(10)

// Shard by worker (consistent routing)
shard := id.ShardByWorker(10)

// Time-based partitioning (for time-series data)
hourBucket := id.ShardByTime(1 * time.Hour)
tableName := fmt.Sprintf("events_%d", hourBucket)
```

### Observability

```go
gen, _ := snowflake.New(1)

// Generate IDs
for i := 0; i < 10000; i++ {
    gen.GenerateID()
}

// Check metrics
metrics := gen.GetMetrics()
fmt.Printf("Generated: %d\n", metrics.Generated)
fmt.Printf("Clock backward events: %d\n", metrics.ClockBackward)
fmt.Printf("Sequence overflows: %d\n", metrics.SequenceOverflow)
fmt.Printf("Total wait time: %d µs\n", metrics.WaitTimeUs)

// Alert on issues
if metrics.ClockBackwardErr > 0 {
    log.Warn("Clock issues detected - check NTP configuration")
}
```

---

## API Reference

### Generator

```go
// Creation
gen, err := New(workerID int64) (*Generator, error)
gen, err := NewWithConfig(cfg Config) (*Generator, error)

// ID Generation
id, err := gen.GenerateID() (ID, error)
id, err := gen.GenerateIDWithContext(ctx context.Context) (ID, error)
id := gen.MustGenerateID() ID  // Panics on error

// Information
workerID := gen.WorkerID() int64
metrics := gen.GetMetrics() Metrics
gen.ResetMetrics()  // For testing
```

### ID Type

```go
// Conversions
id.Int64() int64
id.String() string
id.Uint64() uint64

// Encoding (11 formats)
id.Base2() string      // Binary
id.Base32() string     // z-base-32
id.Base36() string     // 0-9, a-z
id.Base58() string     // Bitcoin-style
id.Base62() string     // URL-safe (recommended)
id.Base64() string     // Standard
id.Base64URL() string  // URL-safe variant
id.Hex() string        // Hexadecimal

// Component Extraction
id.Time() time.Time
id.Timestamp() int64
id.Worker() int64
id.Sequence() int64
id.Components() (timestamp, workerID, sequence int64)
id.Age() time.Duration

// Validation & Comparison
id.IsValid() bool
id.Before(other ID) bool
id.After(other ID) bool
id.Equal(other ID) bool
id.Compare(other ID) int  // -1, 0, 1

// Sharding
id.Shard(numShards int64) int64
id.ShardByWorker(numShards int64) int64
id.ShardByTime(duration time.Duration) int64

// Custom Formatting
id.Format(format string) string  // "hex", "base62", etc.
```

### Parsing

```go
ParseString(s string) (ID, error)
ParseInt64(i int64) ID
ParseBase2(s string) (ID, error)
ParseBase32(s string) (ID, error)
ParseBase36(s string) (ID, error)
ParseBase58(s string) (ID, error)
ParseBase62(s string) (ID, error)
ParseBase64(s string) (ID, error)
ParseBase64URL(s string) (ID, error)
ParseHex(s string) (ID, error)
ParseBytes(b []byte) (ID, error)
ParseIntBytes(b [8]byte) ID
```

### Errors

```go
ErrInvalidWorkerID    // Worker ID not in range [0, 1023]
ErrClockMovedBack     // Clock drift exceeded tolerance
ErrContextCanceled    // Context canceled during generation
ErrInvalidConfig      // Configuration validation failed
```

---

## Performance

### Benchmarks

Measured on Apple M1 Pro, Go 1.23:

```
Operation                    Speed          Allocations
─────────────────────────────────────────────────────────
ID Generation                ~450 ns/op     0 allocs
ID Generation (concurrent)   ~380 ns/op     0 allocs
Base58 Encoding              ~850 ns/op     1 alloc
Base62 Encoding              ~820 ns/op     1 alloc
Base58 Parsing               ~950 ns/op     1 alloc
Hex Encoding                 ~450 ns/op     1 alloc
Component Extraction         ~10 ns/op      0 allocs
```

### Throughput

- **Single worker (Default layout):** ~2.2 million IDs/sec (4,096 IDs/ms)
- **Concurrent (4 workers):** ~8.8 million IDs/sec
- **Maximum (1024 workers):** Theoretical ~4.2 billion IDs/sec

### Layout-Specific Performance

Different layouts have varying throughput limits based on sequence bits and time unit:

| Layout | Throughput/Worker | Concurrent (4 workers) | Notes |
|--------|-------------------|------------------------|-------|
| LayoutDefault | 4.1M IDs/sec | 16.4M IDs/sec | Highest throughput |
| LayoutSuperior | 512K IDs/sec | 2M IDs/sec | Balanced |
| LayoutUltimate | 12.8K IDs/sec | 51.2K IDs/sec | Recommended, long lifespan |
| LayoutMegaScale | 6.4K IDs/sec | 25.6K IDs/sec | Maximum scale (131K nodes) |

**Bitshift Optimization:** Power-of-2 time units (1ms, 2ms, 4ms, 8ms) use bitshift operations instead of division, providing 5-10% performance improvement in the hot path with zero allocations.

### Comparison

| Implementation | Generation | Concurrent | Features | Dependencies |
|---------------|------------|------------|----------|--------------|
| **This package** | 450 ns | 380 ns | Context, Metrics, 11 encodings | 0 |
| bwmarrin/snowflake | 520 ns | 480 ns | Basic encodings | 0 |
| sony/sonyflake | 680 ns | 620 ns | Different bit layout | 0 |
| UUID v4 | 120 ns | 100 ns | Random, not sortable | crypto/rand |

---

## Production Deployment

### Worker ID Assignment

**Critical:** Each node must have a unique worker ID (0-1023) to prevent ID collisions.

#### Static Assignment (Simple)

```go
// Assign manually per deployment
// Pod 1: workerID = 1
// Pod 2: workerID = 2
gen, _ := snowflake.New(1)
```

#### Environment Variable (Kubernetes)

```go
workerID, _ := strconv.ParseInt(os.Getenv("WORKER_ID"), 10, 64)
gen, _ := snowflake.New(workerID)
```

#### Hash-Based (Pod Name)

```go
import "hash/fnv"

func getWorkerIDFromPodName() int64 {
    podName := os.Getenv("POD_NAME")
    hash := fnv.New32a()
    hash.Write([]byte(podName))
    return int64(hash.Sum32() % 1024)
}

gen, _ := snowflake.New(getWorkerIDFromPodName())
```

#### Distributed Coordination (Redis/Etcd)

```go
// Lease worker IDs dynamically
workerID, err := leaseWorkerID(ctx, redisClient)
if err != nil {
    log.Fatal(err)
}
defer releaseWorkerID(ctx, redisClient, workerID)

gen, _ := snowflake.New(workerID)
```

### Production Best Practices

**1. Choose the Right Layout**

```go
// For new projects (recommended)
cfg.Layout = snowflake.LayoutUltimate  // 292 years, 65K nodes, 12.8K IDs/sec

// For massive scale (100K+ nodes)
cfg.Layout = snowflake.LayoutMegaScale  // 292 years, 131K nodes, 6.4K IDs/sec

// For Sonyflake compatibility
cfg.Layout = snowflake.LayoutSonyflake  // 174 years, 65K nodes, 10ms precision

// For maximum throughput (<1K nodes)
cfg.Layout = snowflake.LayoutDefault    // 69 years, 1K nodes, 4.1M IDs/sec
```

**Layout Selection Guide:**
- **Most cases:** LayoutUltimate (best balance of lifespan, scale, throughput)
- **100K+ nodes:** LayoutMegaScale (maximum node capacity)
- **Legacy/high throughput:** LayoutDefault (69 years, highest throughput)
- **Long-term archival:** LayoutLongLife (139 years)

**2. Choose the Right Encoding**

| Use Case | Format | Why |
|----------|--------|-----|
| Database storage | Int64 | Most efficient (8 bytes) |
| REST API | Base62 | URL-safe, compact |
| Display to users | Base58 | No ambiguous characters |
| Binary protocols | IntBytes() | Fixed 8-byte format |

**3. Handle Clock Issues**

```go
cfg := snowflake.DefaultConfig(workerID)
cfg.MaxClockBackward = 10 * time.Millisecond  // Tune for your environment

gen, _ := snowflake.NewWithConfig(cfg)

// Monitor clock issues
if metrics := gen.GetMetrics(); metrics.ClockBackwardErr > 0 {
    // Alert ops team - check NTP configuration
    log.Error("Clock drift detected", "errors", metrics.ClockBackwardErr)
}
```

**3. Monitor Metrics**

```go
// Expose metrics for alerting
http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
    m := gen.GetMetrics()
    fmt.Fprintf(w, "snowflake_ids_generated %d\n", m.Generated)
    fmt.Fprintf(w, "snowflake_clock_backward %d\n", m.ClockBackward)
    fmt.Fprintf(w, "snowflake_sequence_overflow %d\n", m.SequenceOverflow)
})
```

**4. Database Schema**

```sql
-- PostgreSQL
CREATE TABLE users (
    id BIGINT PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ...
);
CREATE INDEX idx_users_created ON users(created_at);

-- MySQL
CREATE TABLE users (
    id BIGINT PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ...
) ENGINE=InnoDB;

-- SQLite
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ...
);
```

### Common Pitfalls

❌ **Don't:** Reuse worker IDs across nodes
✅ **Do:** Ensure unique worker ID per instance

❌ **Don't:** Use default generator in distributed systems
✅ **Do:** Create generator with explicit worker ID

❌ **Don't:** Ignore clock backward errors
✅ **Do:** Monitor and alert on clock issues

❌ **Don't:** Generate >4096 IDs in a millisecond per worker
✅ **Do:** Use multiple workers for higher throughput

---

## Architecture

### ID Structure

**Default Layout (LayoutDefault):**
```
64-bit Snowflake ID Layout:

┌─────────────────────────────────────────────┬──────────────┬──────────────┐
│           41 bits: Timestamp                │  10 bits:    │  12 bits:    │
│     (milliseconds since epoch)              │  Worker ID   │  Sequence    │
│                                             │  (0-1023)    │  (0-4095)    │
└─────────────────────────────────────────────┴──────────────┴──────────────┘
                                              ^              ^
                                              |              |
                                      1024 workers    4096 IDs/ms/worker
```

**LayoutUltimate (Recommended):**
```
64-bit Snowflake ID Layout:

┌─────────────────────────────────────────────┬──────────────┬──────────────┐
│       40 bits: Timestamp (10ms units)       │  16 bits:    │   7 bits:    │
│     2^40 × 10ms = 292 years                 │  Worker ID   │  Sequence    │
│                                             │  (65,536)    │  (128)       │
└─────────────────────────────────────────────┴──────────────┴──────────────┘
                                              ^              ^
                                              |              |
                                       65K workers     12.8K IDs/sec
```

**Components:**

- **Timestamp:** Milliseconds (or units) since epoch (2024-01-01)
  - Default: 41 bits = ~69 years
  - Ultimate: 40 bits × 10ms = 292 years
  - Provides time-ordering

- **Worker ID:** Instance identifier
  - Default: 10 bits = 1,024 nodes
  - Ultimate: 16 bits = 65,536 nodes
  - Must be unique per node

- **Sequence:** Per-time-unit counter
  - Default: 12 bits = 4,096 IDs/ms
  - Ultimate: 7 bits = 128 IDs/10ms = 12,800 IDs/sec
  - Prevents collisions within same time unit

### Bit Layout Configurations

The package supports 8 pre-configured layouts optimized for different scenarios:

**LayoutUltimate (Recommended):**
- **Lifespan:** 292 years (until ~2317)
- **Scale:** 65,536 workers
- **Throughput:** 12,800 IDs/sec per worker
- **Best for:** New projects needing long lifespan + high scale

**LayoutMegaScale:**
- **Lifespan:** 292 years
- **Scale:** 131,072 workers (maximum)
- **Throughput:** 6,400 IDs/sec per worker
- **Best for:** Hyper-scale deployments (100K+ nodes)

**Performance Optimization:**
Power-of-2 time units (1ms, 2ms, 4ms, 8ms) use bitshift instead of division for ~5-10% performance gain. Non-power-of-2 time units (10ms) use division fallback with negligible impact.

### Design Decisions

**Why Monotonic Clock?**
- Resistant to NTP time corrections
- Prevents duplicate IDs during clock adjustments
- Uses `time.Since()` for monotonic guarantees

**Why Bitwise Operations?**
- Compile-time constant evaluation
- Zero allocations in hot path
- 2-3x faster than modulo/division

**Why Atomic Metrics?**
- Lock-free metric updates
- Minimal performance impact (~15ns)
- Production observability without overhead

**Why Context Support?**
- Graceful shutdown during clock drift
- Timeout handling for distributed systems
- Better error propagation

---

## FAQ

**Q: How do I ensure uniqueness across multiple servers?**
A: Each server must have a unique worker ID (0-1023). See [Worker ID Assignment](#worker-id-assignment).

**Q: What happens if the clock moves backward?**
A: The generator waits if drift is within tolerance (default 5ms), otherwise returns `ErrClockMovedBack`.

**Q: Can I generate more than 4096 IDs per millisecond?**
A: Use multiple workers. Each worker can generate 4096 IDs/ms.

**Q: Is it safe to use in concurrent goroutines?**
A: Yes, all methods are thread-safe with minimal lock contention.

**Q: How do I migrate from UUID?**
A: Store IDs as `BIGINT` instead of `CHAR(36)`. Update application code to use Snowflake IDs.

**Q: What's the performance overhead?**
A: ~450ns per ID with zero heap allocations. Metrics add ~15ns via atomic operations.

**Q: Can I customize the epoch?**
A: Yes, via `Config.Epoch`. Earlier epochs extend the ~69-year lifespan.

**Q: What encoding should I use for APIs?**
A: Base62 - it's URL-safe, compact, and widely compatible.

**Q: How do I handle worker ID collisions?**
A: Use distributed coordination (Redis/Etcd) or ensure static assignment in your deployment.

**Q: Is this compatible with Twitter's Snowflake?**
A: Yes, `LayoutDefault` uses the same bit layout (41+10+12). IDs are interoperable (note: different epoch).

**Q: Which layout should I use for a new project?**
A: `LayoutUltimate` - it provides 292 years lifespan with 65,536 nodes and 12,800 IDs/sec per worker, perfect for 99.9% of use cases.

**Q: How do I migrate to a different layout?**
A: Layouts are not backward compatible. Generate new IDs with the new layout, but keep existing IDs in their original format. Use layout-aware parsing: `ParseIDComponentsWithLayout(id, layout)` to parse IDs from different layouts.

**Q: What's the difference between LayoutUltimate and LayoutSonyflake?**
A: LayoutUltimate has 1.7x longer lifespan (292 vs 174 years), same node capacity (65K), but half the throughput (12.8K vs 25.6K IDs/sec). For most systems, 12.8K/sec per worker is more than sufficient.

**Q: Can I use bitshift optimization with 10ms time units?**
A: No, 10ms is not a power-of-2, so it uses division fallback. However, the performance impact is negligible (~5-10%). LayoutUltimate and LayoutMegaScale use 10ms for longer lifespan.

---

## Contributing

Contributions are welcome! Please follow these guidelines:

1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/my-feature`)
3. **Write** tests for new functionality
4. **Ensure** all tests pass (`go test ./...`)
5. **Format** code (`go fmt ./...`)
6. **Commit** with clear messages
7. **Push** to your fork
8. **Open** a Pull Request

**Before submitting:**
- Add tests for new features
- Update documentation
- Run `go test -race ./...`
- Check `go vet ./...`

---

## License

MIT License - see [LICENSE](LICENSE) for details.

Copyright (c) 2025 Syafiq Ismail

---

## Acknowledgments

Inspired by Twitter's Snowflake ID generation algorithm.

**Related Projects:**
- [bwmarrin/snowflake](https://github.com/bwmarrin/snowflake) - Popular Go implementation
- [sony/sonyflake](https://github.com/sony/sonyflake) - Sony's variant with different bit layout

---

**Project Links:**
- [GitHub](https://github.com/sxyafiq/snowflake)
- [Documentation](https://pkg.go.dev/github.com/sxyafiq/snowflake)
- [Examples](./examples)
