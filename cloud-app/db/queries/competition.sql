-- Examples for one session. Repository Go can use the SQL bodies directly.

PREPARE list_events AS
SELECT * FROM cloud_read.v_events
ORDER BY date DESC, id LIMIT $1 OFFSET $2;

PREPARE get_events AS
SELECT * FROM cloud_read.v_events WHERE id = $1;

PREPARE list_rounds AS
SELECT * FROM cloud_read.v_rounds WHERE event_id = $1
ORDER BY round_number, id LIMIT $2 OFFSET $3;

PREPARE get_rounds AS
SELECT * FROM cloud_read.v_rounds WHERE id = $1;

PREPARE list_round_participants AS
SELECT * FROM cloud_read.v_round_participants WHERE round_id = $1
ORDER BY lapak_number, id LIMIT $2 OFFSET $3;

PREPARE get_round_participants AS
SELECT * FROM cloud_read.v_round_participants WHERE id = $1;

PREPARE create_event AS
INSERT INTO competition.events (title, date, location) VALUES ($1, $2, $3)
RETURNING id;

PREPARE update_event_details AS
UPDATE competition.events SET title = $2, date = $3, location = $4
WHERE id = $1 RETURNING id;
