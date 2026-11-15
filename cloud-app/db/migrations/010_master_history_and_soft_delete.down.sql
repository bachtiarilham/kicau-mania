-- Dropping history or splitting merged ADMIN permissions would lose information.
DO $$ BEGIN
 RAISE EXCEPTION '010 has no automatic rollback: master history and merged ADMIN assignments must not be silently discarded. Restore an explicitly selected earlier database instead.';
END $$;
