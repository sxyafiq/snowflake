# Distributed Worker ID Coordination

This example demonstrates how to dynamically assign and manage worker IDs across multiple instances using Redis for distributed coordination.

## Problem

In distributed systems, each Snowflake generator instance needs a unique worker ID (0-1023). Managing this manually is error-prone:

- Manual assignment doesn't scale
- Pod restarts can cause ID conflicts
- No automatic reclamation of dead workers

## Solution

Use Redis to coordinate worker ID assignment:

1. **Worker pool**: Maintain available worker IDs (0-1023)
2. **Lease-based**: Each instance leases a worker ID with TTL
3. **Heartbeat**: Automatic lease renewal while running
4. **Graceful release**: Worker ID returned to pool on shutdown
5. **Auto-reclaim**: Dead workers' IDs automatically expire

## Quick Start

### Start Redis

```bash
docker run -d -p 6379:6379 redis:alpine
```

### Run Multiple Workers

```bash
# Terminal 1
go run redis/main.go
# Output: Leased worker ID: 0

# Terminal 2
go run redis/main.go
# Output: Leased worker ID: 1

# Terminal 3
go run redis/main.go
# Output: Leased worker ID: 2
```

Each instance automatically gets a unique worker ID!

## How It Works

### 1. Lease Worker ID

```go
coordinator := NewWorkerCoordinator("localhost:6379")

// Try to claim an available worker ID
workerID, err := coordinator.LeaseWorkerID(ctx)
if err != nil {
    log.Fatal("No available worker IDs")
}

// Create generator with leased ID
gen, _ := snowflake.New(workerID)
```

### 2. Automatic Renewal

```go
// Background goroutine renews lease every 10 seconds
go coordinator.renewLease(ctx, key)

// Lease has 30-second TTL
// If process dies, lease expires automatically
```

### 3. Graceful Shutdown

```go
// On SIGTERM/SIGINT
coordinator.ReleaseWorkerID(ctx)
// Worker ID immediately available for reuse
```

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Worker 1   │     │   Worker 2   │     │   Worker 3   │
│  (ID: 0)     │     │  (ID: 1)     │     │  (ID: 2)     │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │   Lease with TTL   │                    │
       │ ┌──────────────────┼────────────────────┘
       └─┤                  │
         ▼                  ▼
    ┌────────────────────────────┐
    │         Redis             │
    │  worker:0 = "claimed"     │
    │  worker:1 = "claimed"     │
    │  worker:2 = "claimed"     │
    └────────────────────────────┘
```

## Production Deployment

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: snowflake-worker
spec:
  replicas: 10  # Each gets unique worker ID automatically
  template:
    spec:
      containers:
      - name: worker
        image: snowflake-worker:latest
        env:
        - name: REDIS_ADDR
          value: "redis:6379"
```

### Configuration

```go
const (
    WorkerPoolSize = 1024             // Total available IDs
    LeaseTTL       = 30 * time.Second // Lease expiry
    RenewInterval  = 10 * time.Second // Renewal frequency
)
```

**Rule of thumb:** RenewInterval should be < LeaseTTL/2

## Monitoring

### Check Active Workers

```bash
# Redis CLI
redis-cli
> KEYS snowflake:worker:*
1) "snowflake:worker:0"
2) "snowflake:worker:1"
3) "snowflake:worker:2"

> TTL snowflake:worker:0
(integer) 25  # Seconds until expiry
```

### Metrics

```go
// Track in application
activeWorkers, _ := coordinator.GetActiveWorkers(ctx)
prometheus.WorkerCount.Set(float64(len(activeWorkers)))
```

## Failure Scenarios

### Worker Crash

**What happens:**
- Lease expires after 30 seconds
- Worker ID automatically reclaimed
- New worker can claim the ID

**Timeline:**
```
t=0s   Worker crashes
t=30s  Lease expires
t=30s  Worker ID available again
```

### Network Partition

**What happens:**
- Worker can't renew lease
- After TTL, Redis expires the key
- Worker ID becomes available

**Protection:**
```go
// Application should handle renewal failures
if renewFailed {
    log.Error("Lost connection to Redis - worker ID may be reclaimed")
    // Options: retry, alert, graceful shutdown
}
```

### Redis Failure

**What happens:**
- All workers lose coordination
- Generator continues working with current ID
- **Risk:** New workers might claim same IDs

**Mitigation:**
```go
// Use Redis Sentinel or Cluster for HA
client := redis.NewFailoverClient(&redis.FailoverOptions{
    MasterName:    "mymaster",
    SentinelAddrs: []string{":26379", ":26380", ":26381"},
})
```

## Alternatives: Etcd

For higher reliability, use Etcd:

```go
// Etcd provides stronger consistency guarantees
client, _ := clientv3.New(clientv3.Config{
    Endpoints: []string{"localhost:2379"},
})

// Use leases for automatic expiry
lease, _ := client.Grant(ctx, 30)
client.Put(ctx, "snowflake:worker:0", "claimed", clientv3.WithLease(lease.ID))
```

**Redis vs Etcd:**

| Feature | Redis | Etcd |
|---------|-------|------|
| Simplicity | Easier | More complex |
| Consistency | Eventual | Strong |
| Performance | Faster | Slightly slower |
| HA | Sentinel/Cluster | Built-in Raft |
| Use case | Most systems | Critical systems |

## Troubleshooting

### All Worker IDs Taken

**Symptom:** `no available worker IDs in pool`

**Diagnosis:**
```bash
redis-cli KEYS snowflake:worker:* | wc -l
# If output is 1024, pool is full
```

**Solutions:**
- Scale down unnecessary workers
- Increase pool size (use different layout)
- Check for dead workers (manual cleanup)

### Lease Not Renewing

**Symptom:** Worker loses ID after 30 seconds

**Diagnosis:** Check renewal errors in logs

**Solutions:**
- Verify Redis connectivity
- Check network latency
- Increase LeaseTTL if network is slow

### Worker ID Reuse Too Fast

**Symptom:** Worker ID reclaimed while previous worker still running

**Solution:** Increase LeaseTTL to give more grace period

## Best Practices

1. **Set appropriate TTL**: Balance between quick reclamation and safety margin
2. **Monitor renewal**: Alert if renewals start failing
3. **Use HA Redis**: Sentinel or Cluster for production
4. **Graceful shutdown**: Always release worker ID on exit
5. **Handle edge cases**: Plan for network partitions, Redis failures

## Next Steps

- Implement Etcd version (see `etcd/main.go`)
- Add Prometheus metrics for monitoring
- Implement leader election for coordination
- Add worker health checks

## Resources

- [Redis TTL Documentation](https://redis.io/commands/ttl)
- [Distributed Locks with Redis](https://redis.io/docs/manual/patterns/distributed-locks/)
- [Etcd Lease Documentation](https://etcd.io/docs/v3.5/learning/api/#lease-api)
