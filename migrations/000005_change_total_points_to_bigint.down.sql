-- File: migrations/000005_change_total_points_to_bigint.down.sql

DO $$
BEGIN
    -- Check if the total_points column exists
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'total_points'
    ) THEN
        -- If it exists, alter its type back to INTEGER
        ALTER TABLE users ALTER COLUMN total_points TYPE INTEGER;
    END IF;
    -- If it doesn't exist, we don't need to do anything
END $$;