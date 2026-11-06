-- Examples for one session. Repository Go can use the SQL bodies directly.

PREPARE list_birds AS
SELECT * FROM cloud_read.v_birds
ORDER BY name, id LIMIT $1 OFFSET $2;

PREPARE get_birds AS
SELECT * FROM cloud_read.v_birds WHERE id = $1;

PREPARE list_bird_owners AS
SELECT * FROM cloud_read.v_bird_owners WHERE bird_id = $1
ORDER BY is_primary_contact DESC NULLS LAST, id LIMIT $2 OFFSET $3;

PREPARE get_bird_owners AS
SELECT * FROM cloud_read.v_bird_owners WHERE id = $1;

PREPARE list_lapaks AS
SELECT * FROM cloud_read.v_lapaks
ORDER BY number, id LIMIT $1 OFFSET $2;

PREPARE get_lapaks AS
SELECT * FROM cloud_read.v_lapaks WHERE id = $1;

PREPARE list_devices AS
SELECT * FROM cloud_read.v_devices WHERE lapak_id = $1
ORDER BY created_at DESC NULLS LAST, id LIMIT $2 OFFSET $3;

PREPARE get_devices AS
SELECT * FROM cloud_read.v_devices WHERE id = $1;

-- Mutation examples require a future writer role.
PREPARE create_bird AS
INSERT INTO asset.birds (ring_number, bird_type, name) VALUES ($1, $2, $3)
RETURNING id;

PREPARE update_bird AS
UPDATE asset.birds SET ring_number = $2, bird_type = $3, name = $4
WHERE id = $1 RETURNING id;

-- The foreign key blocks deletion when the bird is registered in a round.
PREPARE delete_bird AS
DELETE FROM asset.birds WHERE id = $1 RETURNING id;
