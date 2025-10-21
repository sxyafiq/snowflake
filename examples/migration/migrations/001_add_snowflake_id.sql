-- Migration: Add Snowflake ID column to users table
-- Phase: 1
-- Safe to rollback: Yes (just drop the column)

-- Add snowflake_id column (nullable initially)
ALTER TABLE users ADD COLUMN snowflake_id BIGINT;

-- Create index for efficient lookups
CREATE INDEX idx_users_snowflake_id ON users(snowflake_id);

-- Add constraint for future (after backfill)
-- ALTER TABLE users ADD CONSTRAINT snowflake_id_not_null CHECK (snowflake_id IS NOT NULL);

-- Rollback:
-- ALTER TABLE users DROP COLUMN snowflake_id;
