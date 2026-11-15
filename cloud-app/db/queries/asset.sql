-- ERD 2026-11-15. Parameters are bound by pgx, never string interpolation.
-- UseCase validates authorization, semantics and transaction/lock order; see README.md.

-- name: ArenaList
-- params: cursor uuid nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_arenas v WHERE ($1::uuid IS NULL OR id>$1) ORDER BY id LIMIT $2::int;

-- name: ArenaGet
-- params: id uuid
SELECT to_jsonb(v) FROM cloud_read.v_arenas v WHERE id=$1::uuid;

-- name: ArenaCreate
-- params: name text, address text, timezone text, is_active boolean
INSERT INTO asset.arenas(name,address,timezone,is_active) VALUES($1::text,$2::text,$3::text,$4::boolean) RETURNING jsonb_build_object('id',id);

-- name: ArenaUpdate
-- params: id uuid, patch jsonb
UPDATE asset.arenas SET name=CASE WHEN $2::jsonb ? 'name' THEN ($2::jsonb->>'name')::text ELSE name END,
 address=CASE WHEN $2::jsonb ? 'address' THEN ($2::jsonb->>'address')::text ELSE address END,
 timezone=CASE WHEN $2::jsonb ? 'timezone' THEN ($2::jsonb->>'timezone')::text ELSE timezone END,
 is_active=CASE WHEN $2::jsonb ? 'is_active' THEN ($2::jsonb->>'is_active')::boolean ELSE is_active END,
 updated_at=CURRENT_TIMESTAMP WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: ArenaDelete
-- params: id uuid
DELETE FROM asset.arenas WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: GantanganList
-- params: arena_id uuid, cursor uuid nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_gantangan v WHERE arena_id=$1::uuid AND ($2::uuid IS NULL OR id>$2) ORDER BY id LIMIT $3::int;

-- name: GantanganGet
-- params: arena_id uuid, id uuid
SELECT to_jsonb(v) FROM cloud_read.v_gantangan v WHERE arena_id=$1::uuid AND id=$2::uuid;

-- name: GantanganCreate
-- params: arena_id uuid, number integer, description text, baris integer, kolom integer
INSERT INTO asset.gantangan(arena_id,number,description,baris,kolom) VALUES($1::uuid,$2::integer,$3::text,$4::integer,$5::integer) RETURNING jsonb_build_object('id',id);

-- name: GantanganUpdate
-- params: arena_id uuid, id uuid, patch jsonb
UPDATE asset.gantangan SET number=CASE WHEN $3::jsonb ? 'number' THEN ($3::jsonb->>'number')::integer ELSE number END,
 description=CASE WHEN $3::jsonb ? 'description' THEN ($3::jsonb->>'description')::text ELSE description END,
 baris=CASE WHEN $3::jsonb ? 'baris' THEN ($3::jsonb->>'baris')::integer ELSE baris END,
 kolom=CASE WHEN $3::jsonb ? 'kolom' THEN ($3::jsonb->>'kolom')::integer ELSE kolom END WHERE arena_id=$1::uuid AND id=$2::uuid RETURNING jsonb_build_object('id',id);

-- name: GantanganDelete
-- params: arena_id uuid, id uuid
DELETE FROM asset.gantangan WHERE arena_id=$1::uuid AND id=$2::uuid RETURNING jsonb_build_object('id',id);

-- name: BirdSpeciesList
-- params: cursor uuid nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_bird_species v WHERE ($1::uuid IS NULL OR id>$1) ORDER BY id LIMIT $2::int;

-- name: BirdSpeciesGet
-- params: id uuid
SELECT to_jsonb(v) FROM cloud_read.v_bird_species v WHERE id=$1::uuid;

-- name: BirdSpeciesCreate
-- params: name text
INSERT INTO asset.bird_species(name) VALUES($1::text) RETURNING jsonb_build_object('id',id);

-- name: BirdSpeciesUpdate
-- params: id uuid, patch jsonb
UPDATE asset.bird_species SET name=CASE WHEN $2::jsonb ? 'name' THEN ($2::jsonb->>'name')::text ELSE name END,
 updated_at=CURRENT_TIMESTAMP WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: BirdSpeciesDelete
-- params: id uuid
DELETE FROM asset.bird_species WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: BirdList
-- params: owner_id uuid nullable, cursor uuid nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_birds v WHERE deleted_at IS NULL AND ($1::uuid IS NULL OR owner_id=$1) AND ($2::uuid IS NULL OR id>$2) ORDER BY id LIMIT $3::int;

-- name: BirdGet
-- params: bird_id uuid
SELECT to_jsonb(v) FROM cloud_read.v_birds v WHERE id=$1::uuid AND deleted_at IS NULL;

-- name: BirdLock
-- params: bird_id uuid
SELECT jsonb_build_object('id',id,'owner_id',owner_id,'bird_species_id',bird_species_id,'version',version,'deleted_at',deleted_at) FROM asset.birds WHERE id=$1::uuid FOR UPDATE;

-- name: BirdCreate
-- params: name text, owner_id uuid, species_id uuid
WITH b AS (INSERT INTO asset.birds(name,owner_id,bird_species_id) VALUES($1,$2::uuid,$3::uuid) RETURNING id,owner_id,version), h AS (INSERT INTO asset.bird_ownership_history(bird_id,user_id) SELECT id,owner_id FROM b RETURNING id) SELECT jsonb_build_object('id',b.id,'version',b.version) FROM b;

-- name: BirdUpdate
-- params: bird_id uuid, expected_version int, patch jsonb (name/bird_species_id)
UPDATE asset.birds SET name=CASE WHEN $3::jsonb?'name' THEN $3->>'name' ELSE name END,bird_species_id=CASE WHEN $3::jsonb?'bird_species_id' THEN ($3->>'bird_species_id')::uuid ELSE bird_species_id END WHERE id=$1::uuid AND version=$2::int AND deleted_at IS NULL RETURNING jsonb_build_object('id',id,'version',version);

-- name: BirdTransferOwner
-- params: bird_id uuid, expected_version int, new_owner_id uuid, notes text nullable
WITH old AS (SELECT id,owner_id,version FROM asset.birds WHERE id=$1::uuid AND deleted_at IS NULL FOR UPDATE), changed AS (UPDATE asset.birds b SET owner_id=$3::uuid FROM old WHERE b.id=old.id AND old.version=$2::int AND old.owner_id<>$3::uuid AND EXISTS(SELECT FROM iam.users u WHERE u.id=$3::uuid AND u.deleted_at IS NULL AND u.is_active) RETURNING b.id,b.owner_id,b.version), history AS (INSERT INTO asset.bird_ownership_history(bird_id,user_id,notes) SELECT id,owner_id,$4 FROM changed RETURNING id) SELECT jsonb_build_object('id',id,'version',version,'changed',true) FROM changed UNION ALL SELECT jsonb_build_object('id',id,'version',version,'changed',false) FROM old WHERE version=$2::int AND owner_id=$3::uuid AND NOT EXISTS(SELECT FROM changed);

-- name: BirdSoftDelete
-- params: bird_id uuid, expected_version int
UPDATE asset.birds SET deleted_at=coalesce(deleted_at,CURRENT_TIMESTAMP) WHERE id=$1::uuid AND version=$2::int RETURNING jsonb_build_object('id',id,'version',version);

-- name: BirdMasterHistory
-- params: bird_id uuid, after_version int, limit int
SELECT to_jsonb(v) FROM cloud_read.v_bird_master_history v WHERE bird_id=$1::uuid AND version>$2::int ORDER BY version,id LIMIT $3::int;

-- name: BirdOwnershipHistory
-- params: bird_id uuid
SELECT to_jsonb(v) FROM cloud_read.v_bird_ownership_history v WHERE bird_id=$1::uuid ORDER BY ownership_date,id;

-- name: BirdAwardList
-- params: bird_id uuid, cursor uuid nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_bird_award_certificate_history v WHERE bird_id=$1::uuid AND ($2::uuid IS NULL OR id>$2) ORDER BY id LIMIT $3::int;

-- name: BirdAwardGet
-- params: bird_id uuid, award_id uuid
SELECT to_jsonb(v) FROM cloud_read.v_bird_award_certificate_history v WHERE bird_id=$1::uuid AND id=$2::uuid;

-- name: BirdAwardCreate
-- params: bird_id uuid, registration_id uuid nullable, award_name text, event_name text nullable, organizer_name text nullable, rank int nullable, awarded_at date, certificate_url text nullable
INSERT INTO asset.bird_award_certificate_history(bird_id,registration_id,award_name,event_name,organizer_name,rank,awarded_at,certificate_url) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6::int,$7::date,$8) RETURNING jsonb_build_object('id',id);

-- name: BirdAwardUpdate
-- params: bird_id uuid, award_id uuid, award_name text, event_name text nullable, organizer_name text nullable, rank int nullable, awarded_at date, certificate_url text nullable
UPDATE asset.bird_award_certificate_history SET award_name=$3,event_name=$4,organizer_name=$5,rank=$6::int,awarded_at=$7::date,certificate_url=$8,updated_at=CURRENT_TIMESTAMP WHERE bird_id=$1::uuid AND id=$2::uuid RETURNING jsonb_build_object('id',id);

-- name: BirdAwardDelete
-- params: bird_id uuid, award_id uuid
DELETE FROM asset.bird_award_certificate_history WHERE bird_id=$1::uuid AND id=$2::uuid RETURNING jsonb_build_object('id',id);

-- name: BirdAwardCertificateInternal
-- params: award_id uuid
SELECT jsonb_build_object('id',id,'bird_id',bird_id,'certificate_url',certificate_url) FROM asset.bird_award_certificate_history WHERE id=$1::uuid;
