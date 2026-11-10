-- Named statements; placeholders are bound by pgx. No PREPARE execution required.

-- name: EntityArenasGet
SELECT to_jsonb(v) FROM cloud_read.v_arenas v WHERE id=$1::uuid;

-- name: EntityArenasList
SELECT to_jsonb(v) FROM cloud_read.v_arenas v WHERE id::text>$1 AND ($2::text='' OR true) ORDER BY id LIMIT $3;

-- name: EntityArenasCreate
INSERT INTO asset.arenas(name,address,timezone,is_active) VALUES(($1::jsonb->>'name')::text,($1::jsonb->>'address')::text,coalesce(($1::jsonb->>'timezone')::text,'Asia/Jakarta'),coalesce(($1::jsonb->>'is_active')::boolean,true)) RETURNING jsonb_build_object('id',id);

-- name: EntityArenasUpdate
UPDATE asset.arenas SET name=CASE WHEN $2::jsonb?'name' THEN ($2->>'name')::text ELSE name END,address=CASE WHEN $2::jsonb?'address' THEN ($2->>'address')::text ELSE address END,timezone=CASE WHEN $2::jsonb?'timezone' THEN ($2->>'timezone')::text ELSE timezone END,is_active=CASE WHEN $2::jsonb?'is_active' THEN ($2->>'is_active')::boolean ELSE is_active END WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityArenasDelete
DELETE FROM asset.arenas WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityOwnersGet
SELECT to_jsonb(v) FROM cloud_read.v_owners v WHERE id=$1::uuid;

-- name: EntityOwnersList
SELECT to_jsonb(v) FROM cloud_read.v_owners v WHERE id::text>$1 AND ($2::text='' OR true) ORDER BY id LIMIT $3;

-- name: EntityOwnersCreate
INSERT INTO asset.owners(user_id,name,phone,created_by_user_id,created_in_arena_id) VALUES(($1::jsonb->>'user_id')::uuid,($1::jsonb->>'name')::text,($1::jsonb->>'phone')::text,($1::jsonb->>'created_by_user_id')::uuid,($1::jsonb->>'created_in_arena_id')::uuid) RETURNING jsonb_build_object('id',id);

-- name: EntityOwnersUpdate
UPDATE asset.owners SET user_id=CASE WHEN $2::jsonb?'user_id' THEN ($2->>'user_id')::uuid ELSE user_id END,name=CASE WHEN $2::jsonb?'name' THEN ($2->>'name')::text ELSE name END,phone=CASE WHEN $2::jsonb?'phone' THEN ($2->>'phone')::text ELSE phone END,created_by_user_id=CASE WHEN $2::jsonb?'created_by_user_id' THEN ($2->>'created_by_user_id')::uuid ELSE created_by_user_id END,created_in_arena_id=CASE WHEN $2::jsonb?'created_in_arena_id' THEN ($2->>'created_in_arena_id')::uuid ELSE created_in_arena_id END WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityOwnersDelete
DELETE FROM asset.owners WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityBirdsGet
SELECT to_jsonb(v) FROM cloud_read.v_birds v WHERE id=$1::uuid;

-- name: EntityBirdsList
SELECT to_jsonb(v) FROM cloud_read.v_birds v WHERE id::text>$1 AND ($2::text='' OR true) ORDER BY id LIMIT $3;

-- name: EntityBirdsCreate
INSERT INTO asset.birds(owner_id,ring_number,bird_type,name) VALUES(($1::jsonb->>'owner_id')::uuid,($1::jsonb->>'ring_number')::text,($1::jsonb->>'bird_type')::text,($1::jsonb->>'name')::text) RETURNING jsonb_build_object('id',id);

-- name: EntityBirdsUpdate
UPDATE asset.birds SET owner_id=CASE WHEN $2::jsonb?'owner_id' THEN ($2->>'owner_id')::uuid ELSE owner_id END,ring_number=CASE WHEN $2::jsonb?'ring_number' THEN ($2->>'ring_number')::text ELSE ring_number END,bird_type=CASE WHEN $2::jsonb?'bird_type' THEN ($2->>'bird_type')::text ELSE bird_type END,name=CASE WHEN $2::jsonb?'name' THEN ($2->>'name')::text ELSE name END WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityBirdsDelete
DELETE FROM asset.birds WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityGantanganGet
SELECT to_jsonb(v) FROM cloud_read.v_gantangan v WHERE id=$1::uuid;

-- name: EntityGantanganList
SELECT to_jsonb(v) FROM cloud_read.v_gantangan v WHERE id::text>$1 AND ($2::text='' OR arena_id::text=$2) ORDER BY id LIMIT $3;

-- name: EntityGantanganCreate
INSERT INTO asset.gantangan(arena_id,number,description) VALUES(($1::jsonb->>'arena_id')::uuid,($1::jsonb->>'number')::integer,($1::jsonb->>'description')::text) RETURNING jsonb_build_object('id',id);

-- name: EntityGantanganUpdate
UPDATE asset.gantangan SET arena_id=CASE WHEN $2::jsonb?'arena_id' THEN ($2->>'arena_id')::uuid ELSE arena_id END,number=CASE WHEN $2::jsonb?'number' THEN ($2->>'number')::integer ELSE number END,description=CASE WHEN $2::jsonb?'description' THEN ($2->>'description')::text ELSE description END WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityGantanganDelete
DELETE FROM asset.gantangan WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityEnginesGet
SELECT to_jsonb(v) FROM cloud_read.v_engines v WHERE id=$1::uuid;

-- name: EntityEnginesList
SELECT to_jsonb(v) FROM cloud_read.v_engines v WHERE id::text>$1 AND ($2::text='' OR arena_id::text=$2) ORDER BY id LIMIT $3;

-- name: EntityEnginesCreate
INSERT INTO asset.engines(arena_id,name,is_active) VALUES(($1::jsonb->>'arena_id')::uuid,($1::jsonb->>'name')::text,coalesce(($1::jsonb->>'is_active')::boolean,true)) RETURNING jsonb_build_object('id',id);

-- name: EntityEnginesUpdate
UPDATE asset.engines SET arena_id=CASE WHEN $2::jsonb?'arena_id' THEN ($2->>'arena_id')::uuid ELSE arena_id END,name=CASE WHEN $2::jsonb?'name' THEN ($2->>'name')::text ELSE name END,is_active=CASE WHEN $2::jsonb?'is_active' THEN ($2->>'is_active')::boolean ELSE is_active END WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityEnginesDelete
DELETE FROM asset.engines WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityDevicesGet
SELECT to_jsonb(v) FROM cloud_read.v_devices v WHERE id=$1::uuid;

-- name: EntityDevicesList
SELECT to_jsonb(v) FROM cloud_read.v_devices v WHERE id::text>$1 AND ($2::text='' OR arena_id::text=$2) ORDER BY id LIMIT $3;

-- name: EntityDevicesCreate
INSERT INTO asset.devices(arena_id,engine_id,gantangan_id,type,ip_address,rtsp_url) VALUES(($1::jsonb->>'arena_id')::uuid,($1::jsonb->>'engine_id')::uuid,($1::jsonb->>'gantangan_id')::uuid,($1::jsonb->>'type')::text,($1::jsonb->>'ip_address')::text,($1::jsonb->>'rtsp_url')::text) RETURNING jsonb_build_object('id',id);

-- name: EntityDevicesUpdate
UPDATE asset.devices SET arena_id=CASE WHEN $2::jsonb?'arena_id' THEN ($2->>'arena_id')::uuid ELSE arena_id END,engine_id=CASE WHEN $2::jsonb?'engine_id' THEN ($2->>'engine_id')::uuid ELSE engine_id END,gantangan_id=CASE WHEN $2::jsonb?'gantangan_id' THEN ($2->>'gantangan_id')::uuid ELSE gantangan_id END,type=CASE WHEN $2::jsonb?'type' THEN ($2->>'type')::text ELSE type END,ip_address=CASE WHEN $2::jsonb?'ip_address' THEN ($2->>'ip_address')::text ELSE ip_address END,rtsp_url=CASE WHEN $2::jsonb?'rtsp_url' THEN ($2->>'rtsp_url')::text ELSE rtsp_url END WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityDevicesDelete
DELETE FROM asset.devices WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: LockBird
SELECT to_jsonb(b) FROM asset.birds b WHERE id=$1::uuid FOR UPDATE;

-- name: TransferBird
INSERT INTO asset.bird_owner_transfers(bird_id,from_owner_id,to_owner_id,performed_by_user_id,reason) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5);

-- name: UpdateBirdOwner
UPDATE asset.birds SET owner_id=$2::uuid WHERE id=$1::uuid;

-- name: BirdLookup
SELECT jsonb_build_object('id',id,'name',name,'ring_number',ring_number,'bird_type',bird_type) FROM cloud_read.v_birds WHERE ring_number=$1;

-- name: OwnerBirds
SELECT to_jsonb(b) FROM cloud_read.v_birds b JOIN cloud_read.v_owners o ON o.id=b.owner_id WHERE b.id::text>$1 AND ($4::boolean OR o.user_id=$2::uuid OR ($5::boolean AND o.user_id IS NULL AND o.created_in_arena_id::text=$3)) ORDER BY b.id LIMIT $6;

-- name: ManageableOwners
SELECT to_jsonb(o) FROM cloud_read.v_owners o WHERE o.id::text>$1 AND ($4::boolean OR o.user_id=$2::uuid OR ($5::boolean AND o.user_id IS NULL AND o.created_in_arena_id::text=$3)) ORDER BY o.id LIMIT $6;

-- name: ArenaCatalog
SELECT to_jsonb(v) FROM cloud_read.v_arenas v WHERE id::text>$1 AND ($3::boolean OR v.is_active OR EXISTS(SELECT FROM iam.arena_user_roles a WHERE a.user_id=$2::uuid AND a.arena_id=v.id)) ORDER BY id LIMIT $4;

-- name: LockOwner
SELECT to_jsonb(o) FROM asset.owners o WHERE id=$1::uuid FOR UPDATE;
-- name: ArenaHasActiveRound
SELECT jsonb_build_object('active',exists(SELECT FROM competition.rounds WHERE arena_id=$1::uuid AND phase IN ('RUNNING','JUDGING','INTERRUPTED')));
