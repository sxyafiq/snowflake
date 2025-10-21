// UUID to Snowflake ID Migration Example
//
// This example demonstrates a zero-downtime migration strategy from UUID
// to Snowflake IDs using a phased approach with dual-write and backfill.
//
// Migration Phases:
// 1. Add snowflake_id column (nullable)
// 2. Start dual-write (write both UUID and Snowflake)
// 3. Backfill existing records
// 4. Switch reads to Snowflake ID
// 5. Remove UUID column (optional)
//
// Usage:
//   go run main.go --phase=<1-5>
//
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sxyafiq/snowflake"
)

// MigrationPhase represents the current migration state
type MigrationPhase int

const (
	PhaseUUIDOnly MigrationPhase = iota
	PhaseDualWrite
	PhaseBackfill
	PhaseSnowflakeRead
	PhaseSnowflakeOnly
)

func (p MigrationPhase) String() string {
	names := []string{
		"UUID_ONLY",
		"DUAL_WRITE",
		"BACKFILL",
		"SNOWFLAKE_READ",
		"SNOWFLAKE_ONLY",
	}
	return names[p]
}

// UserService manages user operations with migration support
type UserService struct {
	db        *sql.DB
	generator *snowflake.Generator
	phase     MigrationPhase
}

// NewUserService creates a new user service
func NewUserService(db *sql.DB, workerID int64, phase MigrationPhase) (*UserService, error) {
	gen, err := snowflake.New(workerID)
	if err != nil {
		return nil, err
	}

	return &UserService{
		db:        db,
		generator: gen,
		phase:     phase,
	}, nil
}

// User represents a user record
type User struct {
	UUID        uuid.UUID
	SnowflakeID snowflake.ID
	Name        string
	Email       string
	CreatedAt   time.Time
}

// CreateUser creates a user based on current migration phase
func (s *UserService) CreateUser(ctx context.Context, name, email string) error {
	switch s.phase {
	case PhaseUUIDOnly:
		return s.createUserUUIDOnly(ctx, name, email)
	case PhaseDualWrite, PhaseBackfill, PhaseSnowflakeRead, PhaseSnowflakeOnly:
		return s.createUserDualWrite(ctx, name, email)
	default:
		return fmt.Errorf("unknown phase: %v", s.phase)
	}
}

func (s *UserService) createUserUUIDOnly(ctx context.Context, name, email string) error {
	userUUID := uuid.New()

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO users (uuid, name, email) VALUES (?, ?, ?)",
		userUUID.String(), name, email)

	return err
}

func (s *UserService) createUserDualWrite(ctx context.Context, name, email string) error {
	userUUID := uuid.New()
	snowflakeID, err := s.generator.GenerateID()
	if err != nil {
		return fmt.Errorf("failed to generate Snowflake ID: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO users (uuid, snowflake_id, name, email) VALUES (?, ?, ?, ?)",
		userUUID.String(), snowflakeID.Int64(), name, email)

	return err
}

// GetUserByUUID retrieves a user by UUID
func (s *UserService) GetUserByUUID(ctx context.Context, userUUID uuid.UUID) (*User, error) {
	var user User
	var uuidStr, snowflakeIDNull sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		"SELECT uuid, snowflake_id, name, email, created_at FROM users WHERE uuid = ?",
		userUUID.String()).Scan(&uuidStr, &snowflakeIDNull, &user.Name, &user.Email, &user.CreatedAt)

	if err != nil {
		return nil, err
	}

	user.UUID = uuid.MustParse(uuidStr)
	if snowflakeIDNull.Valid {
		user.SnowflakeID = snowflake.ID(snowflakeIDNull.Int64)
	}

	return &user, nil
}

// GetUserBySnowflakeID retrieves a user by Snowflake ID
func (s *UserService) GetUserBySnowflakeID(ctx context.Context, id snowflake.ID) (*User, error) {
	var user User
	var uuidStr sql.NullString
	var snowflakeIDVal int64

	err := s.db.QueryRowContext(ctx,
		"SELECT uuid, snowflake_id, name, email, created_at FROM users WHERE snowflake_id = ?",
		id.Int64()).Scan(&uuidStr, &snowflakeIDVal, &user.Name, &user.Email, &user.CreatedAt)

	if err != nil {
		return nil, err
	}

	if uuidStr.Valid {
		user.UUID = uuid.MustParse(uuidStr.String)
	}
	user.SnowflakeID = snowflake.ID(snowflakeIDVal)

	return &user, nil
}

// GetUser retrieves a user based on current migration phase
func (s *UserService) GetUser(ctx context.Context, uuidStr string, snowflakeID int64) (*User, error) {
	switch s.phase {
	case PhaseUUIDOnly, PhaseDualWrite, PhaseBackfill:
		// Read from UUID
		return s.GetUserByUUID(ctx, uuid.MustParse(uuidStr))
	case PhaseSnowflakeRead, PhaseSnowflakeOnly:
		// Read from Snowflake ID
		return s.GetUserBySnowflakeID(ctx, snowflake.ID(snowflakeID))
	default:
		return nil, fmt.Errorf("unknown phase: %v", s.phase)
	}
}

// BackfillSnowflakeIDs backfills Snowflake IDs for existing UUID records
func (s *UserService) BackfillSnowflakeIDs(ctx context.Context) error {
	// Find all users without Snowflake ID
	rows, err := s.db.QueryContext(ctx,
		"SELECT uuid FROM users WHERE snowflake_id IS NULL")
	if err != nil {
		return err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var uuidStr string
		if err := rows.Scan(&uuidStr); err != nil {
			return err
		}

		// Generate Snowflake ID
		snowflakeID, err := s.generator.GenerateID()
		if err != nil {
			log.Printf("Failed to generate ID for UUID %s: %v", uuidStr, err)
			continue
		}

		// Update record
		_, err = s.db.ExecContext(ctx,
			"UPDATE users SET snowflake_id = ? WHERE uuid = ?",
			snowflakeID.Int64(), uuidStr)
		if err != nil {
			log.Printf("Failed to update UUID %s: %v", uuidStr, err)
			continue
		}

		count++
		if count%1000 == 0 {
			log.Printf("Backfilled %d records...", count)
		}
	}

	log.Printf("Backfill complete: %d records updated", count)
	return nil
}

// ValidateMigration validates that all records have both IDs
func (s *UserService) ValidateMigration(ctx context.Context) error {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE snowflake_id IS NULL").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return fmt.Errorf("validation failed: %d users still missing Snowflake ID", count)
	}

	log.Println("Validation passed: All users have Snowflake IDs")
	return nil
}

// MigrationStats returns statistics about the migration
type MigrationStats struct {
	TotalUsers          int
	UsersWithSnowflake  int
	UsersWithoutSnowflake int
	CompletionPercent   float64
}

func (s *UserService) GetMigrationStats(ctx context.Context) (*MigrationStats, error) {
	var stats MigrationStats

	// Total users
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&stats.TotalUsers)
	if err != nil {
		return nil, err
	}

	// Users with Snowflake ID
	err = s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE snowflake_id IS NOT NULL").Scan(&stats.UsersWithSnowflake)
	if err != nil {
		return nil, err
	}

	stats.UsersWithoutSnowflake = stats.TotalUsers - stats.UsersWithSnowflake
	if stats.TotalUsers > 0 {
		stats.CompletionPercent = float64(stats.UsersWithSnowflake) / float64(stats.TotalUsers) * 100
	}

	return &stats, nil
}

func setupDatabase() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "migration.db")
	if err != nil {
		return nil, err
	}

	// Create initial schema (UUID only)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			uuid TEXT PRIMARY KEY,
			snowflake_id INTEGER,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_users_snowflake_id ON users(snowflake_id);
	`)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func demonstratePhase(service *UserService, phase MigrationPhase, ctx context.Context) {
	fmt.Printf("\n=== Phase %d: %s ===\n", phase, phase)
	service.phase = phase

	switch phase {
	case PhaseUUIDOnly:
		fmt.Println("Creating users with UUID only...")
		for i := 1; i <= 5; i++ {
			name := fmt.Sprintf("User%d", i)
			email := fmt.Sprintf("user%d@example.com", i)
			if err := service.CreateUser(ctx, name, email); err != nil {
				log.Printf("Error creating user: %v", err)
			}
		}

	case PhaseDualWrite:
		fmt.Println("Creating users with both UUID and Snowflake ID...")
		for i := 6; i <= 10; i++ {
			name := fmt.Sprintf("User%d", i)
			email := fmt.Sprintf("user%d@example.com", i)
			if err := service.CreateUser(ctx, name, email); err != nil {
				log.Printf("Error creating user: %v", err)
			}
		}

	case PhaseBackfill:
		fmt.Println("Backfilling Snowflake IDs for existing users...")
		if err := service.BackfillSnowflakeIDs(ctx); err != nil {
			log.Printf("Error during backfill: %v", err)
		}
		if err := service.ValidateMigration(ctx); err != nil {
			log.Printf("Validation error: %v", err)
		}

	case PhaseSnowflakeRead:
		fmt.Println("Switching reads to Snowflake ID...")
		fmt.Println("(Application now reads by Snowflake ID, still writes both)")

	case PhaseSnowflakeOnly:
		fmt.Println("Migration complete - using Snowflake IDs exclusively")
		fmt.Println("(UUID column can now be dropped in a future deployment)")
	}

	// Show migration progress
	stats, err := service.GetMigrationStats(ctx)
	if err != nil {
		log.Printf("Error getting stats: %v", err)
		return
	}

	fmt.Printf("\nMigration Progress:\n")
	fmt.Printf("  Total users: %d\n", stats.TotalUsers)
	fmt.Printf("  With Snowflake ID: %d\n", stats.UsersWithSnowflake)
	fmt.Printf("  Without Snowflake ID: %d\n", stats.UsersWithoutSnowflake)
	fmt.Printf("  Completion: %.1f%%\n", stats.CompletionPercent)
}

func main() {
	phaseFlag := flag.Int("phase", 0, "Migration phase (0-4)")
	flag.Parse()

	ctx := context.Background()

	// Setup database
	db, err := setupDatabase()
	if err != nil {
		log.Fatalf("Failed to setup database: %v", err)
	}
	defer db.Close()

	// Create service
	service, err := NewUserService(db, 1, MigrationPhase(*phaseFlag))
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	if *phaseFlag >= 0 && *phaseFlag <= 4 {
		// Run specific phase
		demonstratePhase(service, MigrationPhase(*phaseFlag), ctx)
	} else {
		// Run all phases sequentially
		fmt.Println("=== UUID to Snowflake ID Migration Demo ===")
		fmt.Println("Running all phases sequentially...\n")

		for phase := PhaseUUIDOnly; phase <= PhaseSnowflakeOnly; phase++ {
			demonstratePhase(service, phase, ctx)
			time.Sleep(500 * time.Millisecond)
		}

		fmt.Println("\n=== Migration Complete ===")
		fmt.Println("All users now have Snowflake IDs!")
		fmt.Println("Next steps:")
		fmt.Println("  1. Monitor for any issues")
		fmt.Println("  2. After confidence period, drop UUID column")
		fmt.Println("  3. Update application to remove UUID references")
	}
}
