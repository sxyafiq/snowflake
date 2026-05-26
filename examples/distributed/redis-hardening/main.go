// Multi-host clock guard and worker-ID lease backed by Redis.
//
// The built-in FileClockGuard and FileWorkerLease are single-host. Across
// hosts you need shared storage, which is what the snowflake.ClockGuard and
// snowflake.WorkerLease interfaces are for. This file implements both over
// Redis and wires them into a Generator.
//
// Key differences from the file-based defaults:
//
//   - RedisClockGuard stores the high-water mark with a monotonic ("only ever
//     increases") Lua script, so a delayed write can never lower the mark.
//   - RedisWorkerLease is a TTL lease with a fencing token and a heartbeat:
//     unlike an OS flock there is no kernel to free it on crash, so the TTL is
//     the liveness signal. Renewal and release are token-checked (compare-and-
//     swap) so a process can never disturb a lease it no longer owns.
//
// Usage:
//
//	docker run -d -p 6379:6379 redis:alpine
//	go run ./examples/distributed/redis-hardening
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sxyafiq/snowflake"
)

// ---------------------------------------------------------------------------
// RedisClockGuard implements snowflake.ClockGuard.
// ---------------------------------------------------------------------------

// RedisClockGuard persists per-identity high-water timestamps in Redis.
type RedisClockGuard struct {
	rdb    *redis.Client
	prefix string
}

// NewRedisClockGuard returns a ClockGuard backed by rdb. Keys are namespaced
// under prefix so several apps can share one Redis.
func NewRedisClockGuard(rdb *redis.Client, prefix string) *RedisClockGuard {
	return &RedisClockGuard{rdb: rdb, prefix: prefix}
}

func (g *RedisClockGuard) key(k snowflake.GuardKey) string {
	return g.prefix + ":guard:" + k.String()
}

// Load returns the persisted high-water mark, or found=false if none exists.
func (g *RedisClockGuard) Load(ctx context.Context, k snowflake.GuardKey) (int64, bool, error) {
	v, err := g.rdb.Get(ctx, g.key(k)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("redis clock guard load: %w", err)
	}
	hw, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("redis clock guard parse %q: %w", v, err)
	}
	return hw, true, nil
}

// monotonicStore sets the key to hw only if hw is greater than the current
// value (or the key is unset). This makes Store safe against out-of-order or
// retried writes — the mark can only ever advance.
var monotonicStore = redis.NewScript(`
local cur = redis.call("GET", KEYS[1])
if cur == false or tonumber(ARGV[1]) > tonumber(cur) then
	redis.call("SET", KEYS[1], ARGV[1])
end
return redis.call("GET", KEYS[1])
`)

// Store advances the high-water mark for k to hw (monotonically).
func (g *RedisClockGuard) Store(ctx context.Context, k snowflake.GuardKey, hw int64) error {
	if err := monotonicStore.Run(ctx, g.rdb, []string{g.key(k)}, hw).Err(); err != nil {
		return fmt.Errorf("redis clock guard store: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// RedisWorkerLease implements snowflake.WorkerLease.
// ---------------------------------------------------------------------------

// RedisWorkerLease is a TTL-based exclusive lease with a fencing token and a
// background heartbeat. The TTL — not an OS primitive — is what frees a dead
// holder's identity, so choose ttl > max expected pause (GC, scheduling) and
// keep the renewal interval comfortably below it.
type RedisWorkerLease struct {
	rdb     *redis.Client
	prefix  string
	ttl     time.Duration
	renewal time.Duration
}

// NewRedisWorkerLease returns a WorkerLease backed by rdb.
func NewRedisWorkerLease(rdb *redis.Client, prefix string, ttl time.Duration) *RedisWorkerLease {
	return &RedisWorkerLease{
		rdb:     rdb,
		prefix:  prefix,
		ttl:     ttl,
		renewal: ttl / 3, // renew well before expiry to tolerate a missed beat
	}
}

func (l *RedisWorkerLease) key(k string) string { return l.prefix + ":lease:" + k }

// Acquire takes the lease for key if no live holder owns it.
func (l *RedisWorkerLease) Acquire(ctx context.Context, key string) (snowflake.LeaseHandle, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}

	ok, err := l.rdb.SetNX(ctx, l.key(key), token, l.ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("redis worker lease acquire: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("%w: key=%q", snowflake.ErrWorkerLeaseHeld, key)
	}

	h := &redisLeaseHandle{
		rdb:   l.rdb,
		key:   l.key(key),
		token: token,
		ttl:   l.ttl,
		stop:  make(chan struct{}),
	}
	go h.heartbeat(l.renewal)
	return h, nil
}

// renewIfOwner extends the TTL only while we still hold the token.
var renewIfOwner = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`)

// releaseIfOwner deletes the key only while we still hold the token, so a
// process never deletes a lease that has expired and been re-acquired.
var releaseIfOwner = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)

// redisLeaseHandle owns one acquired lease and renews it until released.
type redisLeaseHandle struct {
	rdb   *redis.Client
	key   string
	token string
	ttl   time.Duration
	once  sync.Once
	stop  chan struct{}
}

// heartbeat renews the lease on an interval until Release is called.
//
// If a renewal is rejected — the lease expired and another process took it —
// this logs and stops. A production implementation should do more than log:
// surface the loss (callback or channel) so the owner stops generating IDs,
// since it can no longer prove exclusive ownership of the worker identity.
func (h *redisLeaseHandle) heartbeat(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-h.stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), interval)
			res, err := renewIfOwner.Run(ctx, h.rdb, []string{h.key}, h.token, h.ttl.Milliseconds()).Int()
			cancel()
			if err != nil || res == 0 {
				log.Printf("worker lease %s: renewal failed (err=%v owned=%d) — lease lost", h.key, err, res)
				return
			}
		}
	}
}

// Release stops the heartbeat and deletes the lease if we still own it. Safe to
// call more than once.
func (h *redisLeaseHandle) Release() error {
	var err error
	h.once.Do(func() {
		close(h.stop)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = releaseIfOwner.Run(ctx, h.rdb, []string{h.key}, h.token).Err()
	})
	if err != nil {
		return fmt.Errorf("redis worker lease release: %w", err)
	}
	return nil
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("worker lease token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ---------------------------------------------------------------------------
// Wiring
// ---------------------------------------------------------------------------

func main() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer func() { _ = rdb.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("cannot reach Redis at %s: %v (start one with: docker run -d -p 6379:6379 redis:alpine)", addr, err)
	}

	const workerID = 42
	cfg := snowflake.DefaultConfig(workerID)
	cfg.ClockGuard = NewRedisClockGuard(rdb, "snowflake")
	cfg.WorkerLease = NewRedisWorkerLease(rdb, "snowflake", 30*time.Second)

	gen, err := snowflake.NewWithConfig(cfg)
	if errors.Is(err, snowflake.ErrWorkerLeaseHeld) {
		log.Fatalf("worker ID %d already held by another live host — refusing to start", workerID)
	}
	if err != nil {
		log.Fatalf("create generator: %v", err)
	}
	defer func() { _ = gen.Close() }()

	fmt.Printf("Generator for worker %d started (Redis-backed guard + lease).\n\n", gen.WorkerID())
	for i := 0; i < 10; i++ {
		id, err := gen.GenerateID()
		if err != nil {
			log.Fatalf("generate: %v", err)
		}
		fmt.Printf("  %s  (base62=%s)\n", id, id.Base62())
	}
	fmt.Println("\nDone. Lease released on exit; clock-guard mark persists in Redis.")
}
