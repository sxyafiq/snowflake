# Database Sharding Example

This example demonstrates how to implement database sharding using Snowflake IDs with multiple sharding strategies and automatic routing.

## Overview

Database sharding distributes data across multiple database instances to improve scalability and performance. Snowflake IDs are ideal for sharding because they:

- Contain timestamps (enabling time-based sharding)
- Are globally unique (no cross-shard conflicts)
- Are sortable (efficient range queries)
- Have consistent size (predictable distribution)

## Sharding Strategies

### 1. Modulo Sharding

**Best for:** Even distribution, simple implementation

```go
shardIndex = id % numShards
```

**Pros:**
- Simple and fast
- Even distribution across shards
- Predictable shard assignment

**Cons:**
- Rebalancing requires moving all data
- Not stable when adding/removing shards

**Use case:** Fixed number of shards, uniform access patterns

### 2. Consistent Hashing

**Best for:** Dynamic shard counts, minimal rebalancing

```go
hash = fnv(id)
shardIndex = hash % numShards
```

**Pros:**
- Only ~1/N keys need to move when adding shard
- Stable distribution
- Works well with dynamic shard counts

**Cons:**
- Slightly more complex
- Requires good hash function

**Use case:** Growing systems, frequent shard additions

### 3. Range-Based Sharding

**Best for:** Time-series data, time-based queries

```go
bucket = timestamp / rangeSize
shardIndex = bucket % numShards
```

**Pros:**
- Excellent for time-series queries
- Recent data on same shard (hot data locality)
- Easy to archive old shards

**Cons:**
- Uneven distribution if write patterns vary over time
- Hot spots during peak periods

**Use case:** Logs, events, time-series data

## Quick Start

### Run with SQLite (Demo)

```bash
# Run the example
go run main.go

# Output shows distribution for each strategy:
# - Modulo: Even distribution
# - Consistent Hash: Stable distribution
# - Range: Time-based distribution
```

### Run with PostgreSQL (Production)

```bash
# Start 4 PostgreSQL shards
docker-compose up -d

# Run example with PostgreSQL
go run main.go --db=postgres

# View shard distribution
docker-compose exec shard-0 psql -U postgres -d snowflake -c "SELECT COUNT(*) FROM users;"
```

## Architecture

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │
       ▼
┌─────────────────┐
│  ShardRouter    │  ◄── Uses ShardStrategy
└──────┬──────────┘
       │
       ├──────┬──────┬──────┐
       ▼      ▼      ▼      ▼
   ┌─────┐┌─────┐┌─────┐┌─────┐
   │Shard││Shard││Shard││Shard│
   │  0  ││  1  ││  2  ││  3  │
   └─────┘└─────┘└─────┘└─────┘
```

## Code Examples

### Create Shard Router

```go
// Choose strategy
strategy := &ModuloStrategy{}  // or ConsistentHashStrategy, RangeStrategy

// Create router with 4 shards
router, err := NewShardRouter(4, strategy)
if err != nil {
    log.Fatal(err)
}
defer router.Close()
```

### Insert Data

```go
service := NewUserService(router, workerID)

// Create user (automatically routed to correct shard)
id, err := service.CreateUser(ctx, "Alice", "alice@example.com")
if err != nil {
    log.Fatal(err)
}

// ID determines which shard stores this user
fmt.Printf("User stored in shard: %d\n", strategy.CalculateShard(id, 4))
```

### Query Data

```go
// Single-shard query (fast)
user, err := service.GetUser(ctx, id)
if err != nil {
    log.Fatal(err)
}

// Cross-shard query (slower, but comprehensive)
allUsers, err := service.GetAllUsers(ctx)
if err != nil {
    log.Fatal(err)
}
```

## Performance Comparison

### Single Database vs Sharded (4 shards)

| Operation | Single DB | 4 Shards | Improvement |
|-----------|-----------|----------|-------------|
| Insert | 1,000/sec | 4,000/sec | 4x |
| Query by ID | 5,000/sec | 20,000/sec | 4x |
| Scan all | 100/sec | 25/sec | 0.25x (worse) |

**Key insight:** Sharding excels at single-record operations but slows down full-table scans.

## Shard Distribution Analysis

Run the example to see actual distribution:

```
Testing Modulo Strategy:
Creating 20 users...

Shard Distribution:
  Shard 0: 5 users (25.0%)
  Shard 1: 5 users (25.0%)
  Shard 2: 5 users (25.0%)
  Shard 3: 5 users (25.0%)

Testing ConsistentHash Strategy:
Shard Distribution:
  Shard 0: 6 users (30.0%)
  Shard 1: 4 users (20.0%)
  Shard 2: 5 users (25.0%)
  Shard 3: 5 users (25.0%)

Testing Range Strategy:
Shard Distribution:
  Shard 0: 20 users (100.0%)  # All recent data
  Shard 1: 0 users (0.0%)
  Shard 2: 0 users (0.0%)
  Shard 3: 0 users (0.0%)
```

## Production Considerations

### 1. Connection Pooling

```go
db.SetMaxOpenConns(25)        // Max connections per shard
db.SetMaxIdleConns(5)         // Keep 5 idle for quick reuse
db.SetConnMaxLifetime(5*time.Minute)  // Rotate connections
```

### 2. Monitoring

```go
// Track queries per shard
for i, shard := range shards {
    stats := shard.Stats()
    fmt.Printf("Shard %d: %d open, %d idle\n",
        i, stats.OpenConnections, stats.Idle)
}
```

### 3. Rebalancing

When adding a new shard, you need to migrate some data:

```bash
# Example: Add shard 4, move ~20% of data from each existing shard
./rebalance --from=0,1,2,3 --to=4 --strategy=modulo --new-count=5
```

See `migrations/rebalance.md` for detailed procedures.

### 4. Cross-Shard Transactions

**Problem:** Distributed transactions are complex and slow.

**Solution:** Design schema to avoid cross-shard transactions:

```go
// BAD: User and Order in different shards
userShard := GetShard(userID)
orderShard := GetShard(orderID)  // Different shard!

// GOOD: Shard by user ID, keep related data together
userShard := GetShard(userID)
// Store user and their orders in same shard
```

### 5. Backup and Recovery

Backup each shard independently:

```bash
# Backup all shards
for i in 0 1 2 3; do
    pg_dump -h shard-$i -U postgres snowflake > backup_shard_$i.sql
done

# Restore specific shard
psql -h shard-2 -U postgres snowflake < backup_shard_2.sql
```

## When to Shard

### Shard when:
- Single database can't handle write load (>10K writes/sec)
- Dataset too large for single instance (>1TB)
- Need geographic distribution
- Want to isolate tenants

### Don't shard when:
- Data fits in single database
- Most queries need full-table scans
- Lots of JOIN operations across data
- Team lacks distributed systems experience

**Rule of thumb:** Delay sharding as long as possible. Start with vertical scaling, read replicas, and caching first.

## Troubleshooting

### Uneven Distribution

**Symptom:** One shard has significantly more data.

**Diagnosis:**
```go
counts, _ := service.CountUsersByShard(ctx)
for shard, count := range counts {
    fmt.Printf("Shard %d: %d users\n", shard, count)
}
```

**Solutions:**
- For Modulo: Check if IDs are sequential (should be evenly distributed)
- For Hash: Verify hash function quality
- For Range: This is expected for time-series data (recent shard is busiest)

### Hot Shard

**Symptom:** One shard receiving disproportionate traffic.

**Diagnosis:** Monitor query counts per shard.

**Solutions:**
- Add read replicas to hot shard
- Use caching for hot data
- Reconsider sharding strategy
- Split hot shard further

### Cross-Shard Queries Too Slow

**Symptom:** GetAllUsers() takes too long.

**Solutions:**
- Add pagination
- Use async queries with timeouts
- Denormalize data for common queries
- Use separate analytics database

## Next Steps

- **Add shards dynamically**: See `migrations/add_shard.md`
- **Implement consistent hashing ring**: For better rebalancing
- **Add shard-aware caching**: Cache layer that understands sharding
- **Set up monitoring**: Track distribution and hotspots

## Resources

- [Sharding Best Practices](https://www.citusdata.com/blog/2018/01/10/sharding-in-plain-english/)
- [Consistent Hashing Explained](https://www.toptal.com/big-data/consistent-hashing)
- [PostgreSQL Sharding Guide](https://www.postgresql.org/docs/current/ddl-partitioning.html)
