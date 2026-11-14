-- Reset 008 intentionally erases old data and has no automatic inverse.
-- Rebuild a separate empty database from the old baseline and migrations 001..007
-- if the previous application schema is needed. Never claim to recover erased data.
DO $$ BEGIN
    RAISE EXCEPTION '008 is a destructive reset; automatic rollback is unsupported. Rebuild the old schema explicitly from baseline plus 001..007.';
END $$;
