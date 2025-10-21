# UUID to Snowflake ID Migration

This example demonstrates a zero-downtime migration from UUID to Snowflake IDs using a phased rollout strategy.

## Why Migrate?

**Benefits of Snowflake IDs over UUIDs:**

| Feature | UUID | Snowflake ID |
|---------|------|--------------|
| Size | 128 bits (16 bytes) | 64 bits (8 bytes) |
| Sortable | No | Yes (time-ordered) |
| Index efficiency | Poor | Excellent |
| Generation speed | ~100ns | ~300ns |
| Database storage | 36 chars string or 16 bytes | 8 bytes |
| Readability | Complex | Compact (Base62) |

**Real-world impact:**
- **50% smaller IDs** = Less storage, faster queries
- **Time-ordered** = Better index performance, easier debugging
- **Compact encoding** = Shorter URLs, better UX

## Migration Strategy: 5 Phases

### Phase 0: UUID Only (Current State)

```sql
CREATE TABLE users (
    uuid TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL
);
```

**No changes yet. Existing system continues normally.**

### Phase 1: Add Snowflake ID Column

```sql
ALTER TABLE users ADD COLUMN snowflake_id BIGINT;
CREATE INDEX idx_users_snowflake_id ON users(snowflake_id);
```

**Actions:**
- Deploy schema change
- Column is nullable
- No application changes yet

**Rollback:** Drop column if needed

### Phase 2: Dual-Write

```go
func (s *UserService) CreateUser(name, email string) error {
    userUUID := uuid.New()
    snowflakeID, _ := s.generator.GenerateID()

    // Write BOTH IDs
    db.Exec("INSERT INTO users (uuid, snowflake_id, name, email) VALUES (?, ?, ?, ?)",
        userUUID, snowflakeID, name, email)
}
```

**Actions:**
- Update application to write both IDs
- New records have both UUID and Snowflake ID
- Reads still use UUID (safe fallback)

**Duration:** Run for monitoring period (1-7 days)

**Rollback:** Revert to Phase 1 (stop writing Snowflake ID)

### Phase 3: Backfill Existing Data

```go
// Backfill all records missing Snowflake ID
UPDATE users SET snowflake_id = generate_snowflake_id() WHERE snowflake_id IS NULL;
```

**Actions:**
- Run backfill script during low-traffic period
- Process in batches (e.g., 1000 records/batch)
- Monitor database load
- Validate all records have Snowflake ID

**Duration:** Minutes to hours depending on data size

**Validation:**
```sql
SELECT COUNT(*) FROM users WHERE snowflake_id IS NULL;  -- Should be 0
```

**Rollback:** Can still rollback to Phase 2 (data not lost)

### Phase 4: Switch Reads to Snowflake ID

```go
func (s *UserService) GetUser(id int64) (*User, error) {
    // Now read by Snowflake ID instead of UUID
    return s.GetUserBySnowflakeID(snowflake.ID(id))
}
```

**Actions:**
- Update application to read by Snowflake ID
- Continue writing both IDs (safety net)
- Monitor for any lookup failures

**Duration:** Run for confidence period (7-30 days)

**Rollback:** Switch reads back to UUID

### Phase 5: Snowflake ID Only (Optional)

```sql
ALTER TABLE users DROP COLUMN uuid;
ALTER TABLE users ADD PRIMARY KEY (snowflake_id);
```

**Actions:**
- Stop writing UUID
- Eventually drop UUID column
- Update all foreign keys

**Warning:** This is the point of no return. Only proceed after extended confidence period.

## Running the Example

### Quick Demo (All Phases)

```bash
go run main.go

# Output shows all 5 phases:
# Phase 0: Creates users with UUID only
# Phase 1: Schema ready for Snowflake IDs
# Phase 2: Creates users with both IDs
# Phase 3: Backfills existing users
# Phase 4: Reads from Snowflake ID
# Phase 5: Migration complete
```

### Run Specific Phase

```bash
# Test dual-write phase
go run main.go --phase=2

# Test backfill
go run main.go --phase=3
```

## Production Deployment

### Timeline Example (1 Million Users)

| Phase | Duration | Actions |
|-------|----------|---------|
| 0 | Ongoing | Current state |
| 1 | 1 day | Schema change, monitoring |
| 2 | 7 days | Dual-write, monitor metrics |
| 3 | 2 hours | Backfill (off-peak hours) |
| 4 | 30 days | Read from Snowflake, monitor |
| 5 | Future | Drop UUID column |

**Total: ~6 weeks for safe migration**

### Backfill Strategy

For large datasets, use batched backfill:

```go
// Backfill in batches of 1000
for {
    result := db.Exec(`
        UPDATE users
        SET snowflake_id = ?
        WHERE uuid IN (
            SELECT uuid FROM users
            WHERE snowflake_id IS NULL
            LIMIT 1000
        )
    `)

    if result.RowsAffected == 0 {
        break  // Done
    }

    time.Sleep(100 * time.Millisecond)  // Rate limit
}
```

### Monitoring

**Key Metrics:**

```go
// Track dual-write success rate
dualWriteSuccessRate := successfulDualWrites / totalWrites * 100

// Alert if < 99.9%
if dualWriteSuccessRate < 99.9 {
    alert("Dual-write failures detected")
}

// Track backfill progress
progress := usersWithSnowflake / totalUsers * 100
```

**Dashboard:**
- Dual-write error rate
- Backfill completion percentage
- Read latency (UUID vs Snowflake)
- Storage savings

## Handling Edge Cases

### Concurrent Writes During Backfill

**Problem:** User updated during backfill might lose Snowflake ID.

**Solution:** Use UPDATE with WHERE clause:

```sql
UPDATE users
SET snowflake_id = ?
WHERE uuid = ? AND snowflake_id IS NULL  -- Only if not already set
```

### Rollback Scenario

If issues found in Phase 4:

```go
// Switch reads back to UUID
func (s *UserService) GetUser(uuid string) (*User, error) {
    // Rollback: read by UUID again
    return s.GetUserByUUID(uuid)
}

// Continue dual-write for safety
// No data loss occurred
```

### Foreign Key Constraints

If other tables reference users:

```sql
-- Phase 1: Add new FK column
ALTER TABLE orders ADD COLUMN user_snowflake_id BIGINT;

-- Phase 3: Backfill FKs
UPDATE orders o
SET user_snowflake_id = (
    SELECT snowflake_id FROM users WHERE uuid = o.user_uuid
);

-- Phase 5: Drop old FK
ALTER TABLE orders DROP COLUMN user_uuid;
```

## Performance Impact

### Storage Savings

```
1M users * 16 bytes (UUID) = 16 MB
1M users * 8 bytes (Snowflake) = 8 MB
Savings: 50% (8 MB)

With indexes: ~30% total savings
```

### Query Performance

```sql
-- UUID lookup (random index access)
EXPLAIN SELECT * FROM users WHERE uuid = 'f47ac10b-...';
-- Cost: Index scan, random I/O

-- Snowflake lookup (sequential index access)
EXPLAIN SELECT * FROM users WHERE snowflake_id = 123456789;
-- Cost: Index scan, sequential I/O (faster)
```

**Benchmark results:**
- UUID lookup: ~0.5ms
- Snowflake lookup: ~0.3ms
- **40% faster queries**

## Validation Checklist

Before each phase:

**Phase 2 (Dual-Write):**
- [ ] Schema change deployed
- [ ] Application writes both IDs
- [ ] No errors in logs
- [ ] Metrics show 100% dual-write success

**Phase 3 (Backfill):**
- [ ] Backfill script tested
- [ ] Database backup created
- [ ] Backfill completed successfully
- [ ] Validation query returns 0: `SELECT COUNT(*) WHERE snowflake_id IS NULL`

**Phase 4 (Switch Reads):**
- [ ] All records have Snowflake ID
- [ ] Application reads by Snowflake ID
- [ ] No lookup failures in logs
- [ ] Performance metrics acceptable

**Phase 5 (Cleanup):**
- [ ] Extended monitoring period complete (30+ days)
- [ ] No rollback needed
- [ ] Foreign keys updated
- [ ] Ready to drop UUID column

## Troubleshooting

### Backfill Taking Too Long

**Solution:** Optimize batch size and add indexes

```sql
-- Add index for backfill query
CREATE INDEX idx_users_backfill ON users(uuid) WHERE snowflake_id IS NULL;

-- Increase batch size if database can handle it
LIMIT 5000  -- instead of 1000
```

### Dual-Write Failures

**Symptom:** Some records missing Snowflake ID

**Diagnosis:**
```sql
SELECT COUNT(*) FROM users
WHERE created_at > '2024-01-15'  -- After Phase 2 deployment
AND snowflake_id IS NULL;
```

**Solution:** Run targeted backfill for affected records

### Foreign Key Violations

**Symptom:** Cannot update FK references

**Solution:** Temporarily disable constraints during backfill

```sql
SET FOREIGN_KEY_CHECKS = 0;  -- MySQL
-- Run backfill
SET FOREIGN_KEY_CHECKS = 1;
```

## Next Steps

After successful migration:

1. **Drop UUID column** (Phase 5)
2. **Update API documentation** (return Snowflake IDs)
3. **Migrate related tables** (orders, sessions, etc.)
4. **Update backup procedures** (new schema)

## Resources

- [Migration SQL Scripts](./migrations/)
- [Rollback Procedures](./ROLLBACK.md)
- [Monitoring Dashboard](./monitoring.md)
