-- Single official winner for an independent round; execute in one transaction.
ALTER TABLE competition.rounds
    ADD COLUMN winner_registration_item_id uuid,
    ADD COLUMN winner_confirmed_by_user_id uuid REFERENCES iam.users(id) ON DELETE RESTRICT,
    ADD COLUMN winner_confirmed_at timestamptz,
    ADD CONSTRAINT rounds_winner_same_round_arena_fkey
        FOREIGN KEY (winner_registration_item_id, id, arena_id)
        REFERENCES competition.registration_items(id, round_id, arena_id) ON DELETE RESTRICT,
    ADD CONSTRAINT rounds_closed_winner_check CHECK (
        (phase = 'CLOSED' AND winner_registration_item_id IS NOT NULL
            AND winner_confirmed_by_user_id IS NOT NULL AND winner_confirmed_at IS NOT NULL)
        OR (phase <> 'CLOSED' AND winner_registration_item_id IS NULL
            AND winner_confirmed_by_user_id IS NULL AND winner_confirmed_at IS NULL)
    );
CREATE INDEX rounds_winner_confirmer_idx ON competition.rounds (winner_confirmed_by_user_id);
COMMENT ON COLUMN competition.rounds.winner_registration_item_id IS
    'Confirmed participant of this same round/arena. UseCase requires complete scores and the highest exact average; operator selects one tied candidate and closes atomically.';
COMMENT ON COLUMN competition.rounds.winner_confirmed_by_user_id IS
    'UseCase must verify operator assignment in this arena; a user FK alone is not authorization.';
-- Scalar fields enforce at most one winner per round. No automatic next-round entry.
-- UseCase must lock the round when submitting/revising scores and when closing,
-- validate complete criteria/judge/participant coverage and exact averages, then
-- commit phase and winner together. CLOSED scores/results cannot be edited.
