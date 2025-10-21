// Time-Series Partitioning Example
//
// This example demonstrates automatic table partitioning using Snowflake ID
// timestamps for efficient time-series data storage and querying.
//
// Features:
// - Automatic partition creation based on Snowflake ID timestamps
// - Multiple time buckets (hourly, daily, monthly)
// - Old partition cleanup (data retention)
// - Optimized queries using partition pruning
//
// Usage:
//   go run main.go
//
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sxyafiq/snowflake"
)

// PartitionInterval defines the partitioning strategy
type PartitionInterval string

const (
	Hourly  PartitionInterval = "hourly"
	Daily   PartitionInterval = "daily"
	Monthly PartitionInterval = "monthly"
)

func (pi PartitionInterval) Duration() time.Duration {
	switch pi {
	case Hourly:
		return 1 * time.Hour
	case Daily:
		return 24 * time.Hour
	case Monthly:
		return 30 * 24 * time.Hour // Approximate
	default:
		return 24 * time.Hour
	}
}

// PartitionManager manages time-based table partitions
type PartitionManager struct {
	db       *sql.DB
	interval PartitionInterval
}

// NewPartitionManager creates a new partition manager
func NewPartitionManager(db *sql.DB, interval PartitionInterval) *PartitionManager {
	return &PartitionManager{
		db:       db,
		interval: interval,
	}
}

// GetPartitionName generates partition name from timestamp
func (pm *PartitionManager) GetPartitionName(t time.Time) string {
	switch pm.interval {
	case Hourly:
		return fmt.Sprintf("events_%s", t.Format("2006_01_02_15"))
	case Daily:
		return fmt.Sprintf("events_%s", t.Format("2006_01_02"))
	case Monthly:
		return fmt.Sprintf("events_%s", t.Format("2006_01"))
	default:
		return fmt.Sprintf("events_%s", t.Format("2006_01_02"))
	}
}

// GetPartitionNameForID gets partition name from Snowflake ID
func (pm *PartitionManager) GetPartitionNameForID(id snowflake.ID) string {
	return pm.GetPartitionName(id.Time())
}

// CreatePartition creates a partition table for the given time
func (pm *PartitionManager) CreatePartition(ctx context.Context, t time.Time) error {
	partitionName := pm.GetPartitionName(t)

	// For SQLite, create a new table (simulating partition)
	// In PostgreSQL, you would use: CREATE TABLE ... PARTITION OF ...
	_, err := pm.db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY,
			data TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_%s_created_at ON %s(created_at);
	`, partitionName, partitionName, partitionName))

	if err != nil {
		return fmt.Errorf("failed to create partition %s: %w", partitionName, err)
	}

	log.Printf("Created partition: %s", partitionName)
	return nil
}

// EnsurePartitionExists creates partition if it doesn't exist
func (pm *PartitionManager) EnsurePartitionExists(ctx context.Context, id snowflake.ID) error {
	return pm.CreatePartition(ctx, id.Time())
}

// InsertEvent inserts an event into the appropriate partition
func (pm *PartitionManager) InsertEvent(ctx context.Context, id snowflake.ID, data string) error {
	// Ensure partition exists
	if err := pm.EnsurePartitionExists(ctx, id); err != nil {
		return err
	}

	// Insert into the correct partition
	partitionName := pm.GetPartitionNameForID(id)
	query := fmt.Sprintf("INSERT INTO %s (id, data, created_at) VALUES (?, ?, ?)",
		partitionName)

	_, err := pm.db.ExecContext(ctx, query,
		id.Int64(), data, id.Time())

	return err
}

// QueryEventsByTimeRange queries events in a time range (uses partition pruning)
func (pm *PartitionManager) QueryEventsByTimeRange(ctx context.Context, start, end time.Time) ([]Event, error) {
	// Generate list of partitions to query
	partitions := pm.getPartitionsBetween(start, end)

	var events []Event

	// Query each relevant partition
	for _, partitionName := range partitions {
		query := fmt.Sprintf(`
			SELECT id, data, created_at FROM %s
			WHERE created_at BETWEEN ? AND ?
		`, partitionName)

		rows, err := pm.db.QueryContext(ctx, query, start, end)
		if err != nil {
			// Partition might not exist, skip it
			continue
		}
		defer rows.Close()

		for rows.Next() {
			var event Event
			var idInt int64
			if err := rows.Scan(&idInt, &event.Data, &event.CreatedAt); err != nil {
				log.Printf("Error scanning row: %v", err)
				continue
			}
			event.ID = snowflake.ID(idInt)
			events = append(events, event)
		}
	}

	return events, nil
}

// getPartitionsBetween returns list of partition names in date range
func (pm *PartitionManager) getPartitionsBetween(start, end time.Time) []string {
	var partitions []string

	current := start
	interval := pm.interval.Duration()

	for current.Before(end) || current.Equal(end) {
		partitions = append(partitions, pm.GetPartitionName(current))
		current = current.Add(interval)
	}

	return partitions
}

// CleanupOldPartitions drops partitions older than retention period
func (pm *PartitionManager) CleanupOldPartitions(ctx context.Context, retentionDays int) error {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	// Get all partition names
	rows, err := pm.db.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'events_%'")
	if err != nil {
		return err
	}
	defer rows.Close()

	droppedCount := 0
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}

		// Parse partition time from name
		partitionTime, err := pm.parsePartitionTime(tableName)
		if err != nil {
			continue
		}

		// Drop if older than retention period
		if partitionTime.Before(cutoffTime) {
			_, err := pm.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", tableName))
			if err != nil {
				log.Printf("Failed to drop partition %s: %v", tableName, err)
				continue
			}
			log.Printf("Dropped old partition: %s", tableName)
			droppedCount++
		}
	}

	log.Printf("Cleaned up %d old partitions", droppedCount)
	return nil
}

// parsePartitionTime extracts time from partition name
func (pm *PartitionManager) parsePartitionTime(partitionName string) (time.Time, error) {
	var format string
	var timeStr string

	switch pm.interval {
	case Hourly:
		format = "2006_01_02_15"
		timeStr = partitionName[7:] // Skip "events_"
	case Daily:
		format = "2006_01_02"
		timeStr = partitionName[7:]
	case Monthly:
		format = "2006_01"
		timeStr = partitionName[7:]
	default:
		return time.Time{}, fmt.Errorf("unknown interval")
	}

	return time.Parse(format, timeStr)
}

// Event represents a time-series event
type Event struct {
	ID        snowflake.ID
	Data      string
	CreatedAt time.Time
}

func main() {
	fmt.Println("=== Time-Series Partitioning Example ===\n")

	ctx := context.Background()

	// Setup database
	db, err := sql.Open("sqlite3", "timeseries.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create partition manager (daily partitions)
	pm := NewPartitionManager(db, Daily)

	// Create Snowflake generator
	gen, err := snowflake.New(1)
	if err != nil {
		log.Fatalf("Failed to create generator: %v", err)
	}

	// Insert events over multiple days
	fmt.Println("Inserting events...")
	for day := 0; day < 3; day++ {
		for i := 0; i < 5; i++ {
			id, err := gen.GenerateID()
			if err != nil {
				log.Printf("Error generating ID: %v", err)
				continue
			}

			data := fmt.Sprintf("Event on day %d, #%d", day, i+1)
			if err := pm.InsertEvent(ctx, id, data); err != nil {
				log.Printf("Error inserting event: %v", err)
				continue
			}

			fmt.Printf("  Inserted: %s (Partition: %s)\n",
				data, pm.GetPartitionNameForID(id))

			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(100 * time.Millisecond) // Simulate next day
	}

	// Query events in time range
	fmt.Println("\nQuerying events from last 2 days...")
	start := time.Now().Add(-48 * time.Hour)
	end := time.Now()

	events, err := pm.QueryEventsByTimeRange(ctx, start, end)
	if err != nil {
		log.Printf("Error querying events: %v", err)
	} else {
		fmt.Printf("Found %d events\n", len(events))
		for _, event := range events {
			fmt.Printf("  - %s (ID: %s, Time: %s)\n",
				event.Data,
				event.ID.Base62(),
				event.CreatedAt.Format("2006-01-02 15:04:05"))
		}
	}

	// Demonstrate cleanup
	fmt.Println("\nCleaning up partitions older than 30 days...")
	if err := pm.CleanupOldPartitions(ctx, 30); err != nil {
		log.Printf("Error during cleanup: %v", err)
	}

	fmt.Println("\n=== Example Complete ===")
	fmt.Println("\nKey Benefits:")
	fmt.Println("  - Automatic partition creation based on Snowflake ID timestamps")
	fmt.Println("  - Efficient time-range queries (partition pruning)")
	fmt.Println("  - Easy data retention with partition cleanup")
	fmt.Println("  - Scalable for high-volume time-series data")
}
