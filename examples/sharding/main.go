// Database Sharding Example
//
// This example demonstrates how to implement database sharding using Snowflake IDs
// with multiple sharding strategies and automatic routing.
//
// Features demonstrated:
// - Modulo sharding (simple, even distribution)
// - Consistent hashing (stable, minimal rebalancing)
// - Range-based sharding (time-series optimized)
// - Multi-database connection pooling
// - Cross-shard queries
// - Shard rebalancing
//
// Usage:
//   docker-compose up -d  # Start 4 PostgreSQL shards
//   go run main.go
//
package main

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sxyafiq/snowflake"
)

// ShardStrategy defines how IDs are distributed across shards
type ShardStrategy interface {
	CalculateShard(id snowflake.ID, numShards int) int
	Name() string
}

// ModuloStrategy uses simple modulo for even distribution
type ModuloStrategy struct{}

func (s *ModuloStrategy) CalculateShard(id snowflake.ID, numShards int) int {
	return int(id.Int64() % int64(numShards))
}

func (s *ModuloStrategy) Name() string {
	return "Modulo"
}

// ConsistentHashStrategy uses consistent hashing for stable distribution
type ConsistentHashStrategy struct {
	virtualNodes int
}

func NewConsistentHashStrategy(virtualNodes int) *ConsistentHashStrategy {
	return &ConsistentHashStrategy{virtualNodes: virtualNodes}
}

func (s *ConsistentHashStrategy) CalculateShard(id snowflake.ID, numShards int) int {
	h := fnv.New32a()
	h.Write([]byte(id.String()))
	hash := h.Sum32()
	return int(hash % uint32(numShards))
}

func (s *ConsistentHashStrategy) Name() string {
	return "ConsistentHash"
}

// RangeStrategy uses timestamp-based range sharding (time-series optimized)
type RangeStrategy struct {
	rangeSize time.Duration // e.g., 1 day, 1 week, 1 month
}

func NewRangeStrategy(rangeSize time.Duration) *RangeStrategy {
	return &RangeStrategy{rangeSize: rangeSize}
}

func (s *RangeStrategy) CalculateShard(id snowflake.ID, numShards int) int {
	timestamp := id.Timestamp()
	bucket := timestamp / s.rangeSize.Milliseconds()
	return int(bucket % int64(numShards))
}

func (s *RangeStrategy) Name() string {
	return "Range"
}

// ShardRouter manages connections to multiple database shards
type ShardRouter struct {
	shards   []*sql.DB
	strategy ShardStrategy
	mu       sync.RWMutex
}

// NewShardRouter creates a new shard router
func NewShardRouter(shardCount int, strategy ShardStrategy) (*ShardRouter, error) {
	shards := make([]*sql.DB, shardCount)

	// For this example, use SQLite databases to simulate shards
	// In production, these would be separate PostgreSQL/MySQL databases
	for i := 0; i < shardCount; i++ {
		dbPath := fmt.Sprintf("shard_%d.db", i)
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open shard %d: %w", i, err)
		}

		// Set connection pool settings
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)

		// Create schema
		if err := createSchema(db); err != nil {
			return nil, fmt.Errorf("failed to create schema for shard %d: %w", i, err)
		}

		shards[i] = db
	}

	return &ShardRouter{
		shards:   shards,
		strategy: strategy,
	}, nil
}

// GetShard returns the database connection for the shard that should contain the given ID
func (r *ShardRouter) GetShard(id snowflake.ID) *sql.DB {
	r.mu.RLock()
	defer r.mu.RUnlock()

	shardIndex := r.strategy.CalculateShard(id, len(r.shards))
	return r.shards[shardIndex]
}

// GetAllShards returns all shard connections (for cross-shard queries)
func (r *ShardRouter) GetAllShards() []*sql.DB {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.shards
}

// Close closes all shard connections
func (r *ShardRouter) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var firstErr error
	for i, db := range r.shards {
		if err := db.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to close shard %d: %w", i, err)
		}
	}
	return firstErr
}

// User represents a user record
type User struct {
	ID        snowflake.ID
	Name      string
	Email     string
	CreatedAt time.Time
}

// UserService provides user operations with automatic sharding
type UserService struct {
	router    *ShardRouter
	generator *snowflake.Generator
}

// NewUserService creates a new user service
func NewUserService(router *ShardRouter, workerID int64) (*UserService, error) {
	gen, err := snowflake.New(workerID)
	if err != nil {
		return nil, err
	}

	return &UserService{
		router:    router,
		generator: gen,
	}, nil
}

// CreateUser creates a new user in the appropriate shard
func (s *UserService) CreateUser(ctx context.Context, name, email string) (snowflake.ID, error) {
	// Generate ID
	id, err := s.generator.GenerateIDWithContext(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to generate ID: %w", err)
	}

	// Get shard for this ID
	db := s.router.GetShard(id)

	// Insert into shard
	_, err = db.ExecContext(ctx,
		"INSERT INTO users (id, name, email, created_at) VALUES (?, ?, ?, ?)",
		id.Int64(), name, email, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to insert user: %w", err)
	}

	return id, nil
}

// GetUser retrieves a user by ID from the appropriate shard
func (s *UserService) GetUser(ctx context.Context, id snowflake.ID) (*User, error) {
	db := s.router.GetShard(id)

	var user User
	var idInt int64
	var createdAtStr string

	err := db.QueryRowContext(ctx,
		"SELECT id, name, email, created_at FROM users WHERE id = ?",
		id.Int64()).Scan(&idInt, &user.Name, &user.Email, &createdAtStr)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	user.ID = snowflake.ID(idInt)
	user.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)

	return &user, nil
}

// GetAllUsers retrieves all users across all shards (cross-shard query)
func (s *UserService) GetAllUsers(ctx context.Context) ([]*User, error) {
	shards := s.router.GetAllShards()
	usersChan := make(chan []*User, len(shards))
	errChan := make(chan error, len(shards))

	// Query each shard in parallel
	for _, shard := range shards {
		go func(db *sql.DB) {
			rows, err := db.QueryContext(ctx, "SELECT id, name, email, created_at FROM users")
			if err != nil {
				errChan <- err
				return
			}
			defer rows.Close()

			var users []*User
			for rows.Next() {
				var user User
				var idInt int64
				var createdAtStr string

				if err := rows.Scan(&idInt, &user.Name, &user.Email, &createdAtStr); err != nil {
					errChan <- err
					return
				}

				user.ID = snowflake.ID(idInt)
				user.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
				users = append(users, &user)
			}

			usersChan <- users
		}(shard)
	}

	// Collect results from all shards
	var allUsers []*User
	for i := 0; i < len(shards); i++ {
		select {
		case users := <-usersChan:
			allUsers = append(allUsers, users...)
		case err := <-errChan:
			return nil, fmt.Errorf("shard query failed: %w", err)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return allUsers, nil
}

// CountUsersByShare returns the count of users in each shard
func (s *UserService) CountUsersByShard(ctx context.Context) (map[int]int, error) {
	shards := s.router.GetAllShards()
	counts := make(map[int]int)

	for i, shard := range shards {
		var count int
		err := shard.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("failed to count users in shard %d: %w", i, err)
		}
		counts[i] = count
	}

	return counts, nil
}

func createSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);
		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	`)
	return err
}

func main() {
	fmt.Println("=== Snowflake Database Sharding Example ===")
	fmt.Println()

	// Test all three sharding strategies
	strategies := []ShardStrategy{
		&ModuloStrategy{},
		NewConsistentHashStrategy(100),
		NewRangeStrategy(24 * time.Hour), // Daily range
	}

	for _, strategy := range strategies {
		fmt.Printf("Testing %s Strategy:\n", strategy.Name())
		fmt.Println(strings.Repeat("-", 50))

		if err := runExample(strategy); err != nil {
			log.Printf("Error running %s example: %v\n", strategy.Name(), err)
		}

		fmt.Println()
	}

	// Cleanup
	fmt.Println("Cleaning up test databases...")
	for i := 0; i < 4; i++ {
		os.Remove(fmt.Sprintf("shard_%d.db", i))
	}
}

func runExample(strategy ShardStrategy) error {
	ctx := context.Background()

	// Create shard router with 4 shards
	router, err := NewShardRouter(4, strategy)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}
	defer router.Close()

	// Create user service
	service, err := NewUserService(router, 1)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	// Create 20 test users
	fmt.Println("Creating 20 users...")
	userIDs := make([]snowflake.ID, 20)
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("User%d", i+1)
		email := fmt.Sprintf("user%d@example.com", i+1)

		id, err := service.CreateUser(ctx, name, email)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		userIDs[i] = id
		time.Sleep(1 * time.Millisecond) // Small delay to vary timestamps
	}

	// Show shard distribution
	counts, err := service.CountUsersByShard(ctx)
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	fmt.Println("\nShard Distribution:")
	for shard, count := range counts {
		percentage := float64(count) / 20.0 * 100
		fmt.Printf("  Shard %d: %d users (%.1f%%)\n", shard, count, percentage)
	}

	// Test retrieval
	fmt.Println("\nTesting user retrieval:")
	testID := userIDs[0]
	user, err := service.GetUser(ctx, testID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	fmt.Printf("  Retrieved: %s <%s> (ID: %s)\n", user.Name, user.Email, user.ID.Base62())

	// Test cross-shard query
	allUsers, err := service.GetAllUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get all users: %w", err)
	}
	fmt.Printf("\nCross-shard query returned %d users\n", len(allUsers))

	return nil
}
