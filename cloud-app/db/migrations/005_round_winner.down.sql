-- Never erase an official result through a schema rollback.
LOCK TABLE competition.rounds IN ACCESS EXCLUSIVE MODE;
DO $winner$
BEGIN
    IF EXISTS (SELECT FROM competition.rounds WHERE phase = 'CLOSED'
        OR winner_registration_item_id IS NOT NULL OR winner_confirmed_by_user_id IS NOT NULL
        OR winner_confirmed_at IS NOT NULL) THEN
        RAISE EXCEPTION 'Cannot remove winner fields while official results exist; restore a pre-migration backup instead';
    END IF;
END;
$winner$;
DROP INDEX competition.rounds_winner_confirmer_idx;
ALTER TABLE competition.rounds
    DROP CONSTRAINT rounds_closed_winner_check,
    DROP CONSTRAINT rounds_winner_same_round_arena_fkey,
    DROP COLUMN winner_registration_item_id,
    DROP COLUMN winner_confirmed_by_user_id,
    DROP COLUMN winner_confirmed_at;
