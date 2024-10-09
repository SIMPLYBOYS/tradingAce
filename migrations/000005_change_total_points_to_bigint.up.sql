DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'total_points'
    ) THEN
        ALTER TABLE users ALTER COLUMN total_points TYPE BIGINT;
    ELSE
        ALTER TABLE users ADD COLUMN total_points BIGINT DEFAULT 0;
    END IF;
END $$;