-- ERD 2026-11-15; apply after 009 in one transaction. Existing masters survive.
ALTER TABLE iam.users ADD COLUMN version integer NOT NULL DEFAULT 1 CHECK(version>0), ADD COLUMN deleted_at timestamptz;
ALTER TABLE asset.birds ADD COLUMN version integer NOT NULL DEFAULT 1 CHECK(version>0), ADD COLUMN deleted_at timestamptz;

CREATE TABLE iam.user_master_history (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES iam.users(id),
 version integer NOT NULL CHECK(version>0), snapshot jsonb NOT NULL,
 change_type varchar(20) NOT NULL CHECK(change_type IN ('BASELINE','CREATE','UPDATE','DELETE')),
 actor_id uuid REFERENCES iam.users(id), arena_id uuid REFERENCES asset.arenas(id),
 recorded_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE(user_id,version), CHECK(change_type='BASELINE' OR actor_id IS NOT NULL),
 CHECK(jsonb_typeof(snapshot)='object' AND snapshot ?& ARRAY['name','phone','email','is_active','is_eo','is_deleted']
 AND snapshot-ARRAY['name','phone','email','is_active','is_eo','is_deleted']='{}'::jsonb)
);
CREATE TABLE asset.bird_master_history (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), bird_id uuid NOT NULL REFERENCES asset.birds(id),
 version integer NOT NULL CHECK(version>0), snapshot jsonb NOT NULL,
 change_type varchar(20) NOT NULL CHECK(change_type IN ('BASELINE','CREATE','UPDATE','DELETE')),
 actor_id uuid REFERENCES iam.users(id), arena_id uuid REFERENCES asset.arenas(id),
 recorded_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE(bird_id,version), CHECK(change_type='BASELINE' OR actor_id IS NOT NULL),
 CHECK(jsonb_typeof(snapshot)='object' AND snapshot ?& ARRAY['bird_code','name','owner_id','bird_species_id','is_deleted']
 AND snapshot-ARRAY['bird_code','name','owner_id','bird_species_id','is_deleted']='{}'::jsonb)
);
CREATE INDEX user_history_actor_idx ON iam.user_master_history(actor_id);
CREATE INDEX user_history_arena_idx ON iam.user_master_history(arena_id);
CREATE INDEX bird_history_actor_idx ON asset.bird_master_history(actor_id);
CREATE INDEX bird_history_arena_idx ON asset.bird_master_history(arena_id);

CREATE FUNCTION iam.user_master_snapshot(u iam.users) RETURNS jsonb LANGUAGE sql IMMUTABLE
SET search_path=pg_catalog AS $$
 SELECT jsonb_build_object('name',u.name,'phone',u.phone,'email',u.email,'is_active',u.is_active,'is_eo',u.is_eo,'is_deleted',u.deleted_at IS NOT NULL);
$$;
CREATE FUNCTION asset.bird_master_snapshot(b asset.birds) RETURNS jsonb LANGUAGE sql IMMUTABLE
SET search_path=pg_catalog AS $$
 SELECT jsonb_build_object('bird_code',b.bird_code,'name',b.name,'owner_id',b.owner_id,'bird_species_id',b.bird_species_id,'is_deleted',b.deleted_at IS NOT NULL);
$$;
REVOKE ALL ON FUNCTION iam.user_master_snapshot(iam.users),asset.bird_master_snapshot(asset.birds) FROM PUBLIC;
INSERT INTO iam.user_master_history(user_id,version,snapshot,change_type)
 SELECT id,1,iam.user_master_snapshot(u),'BASELINE' FROM iam.users u;
INSERT INTO asset.bird_master_history(bird_id,version,snapshot,change_type)
 SELECT id,1,asset.bird_master_snapshot(b),'BASELINE' FROM asset.birds b;

CREATE FUNCTION iam.prepare_user_master() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='UPDATE' THEN
  IF NEW.id<>OLD.id OR NEW.version<>OLD.version THEN RAISE EXCEPTION 'Master ID/version is system managed' USING ERRCODE='23514'; END IF;
  IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN NEW.is_active:=false; END IF;
  IF iam.user_master_snapshot(NEW)=iam.user_master_snapshot(OLD) THEN
   NEW.version:=OLD.version; NEW.updated_at:=OLD.updated_at; NEW.deleted_at:=OLD.deleted_at;
   RETURN NEW;
  END IF;
  NEW.version:=OLD.version+1;
 ELSE
  IF NEW.version<>1 THEN RAISE EXCEPTION 'New master version must be 1' USING ERRCODE='23514'; END IF;
 END IF;
 IF nullif(current_setting('app.actor_id',true),'') IS NULL THEN RAISE EXCEPTION 'Master mutation requires transaction actor context' USING ERRCODE='23514'; END IF;
 NEW.updated_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE FUNCTION iam.record_user_master() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='UPDATE' AND NEW.version=OLD.version THEN RETURN NEW; END IF;
 INSERT INTO iam.user_master_history(user_id,version,snapshot,change_type,actor_id,arena_id)
 VALUES(NEW.id,NEW.version,iam.user_master_snapshot(NEW),
 CASE WHEN TG_OP='INSERT' THEN 'CREATE' WHEN NEW.deleted_at IS NOT NULL THEN 'DELETE' ELSE 'UPDATE' END,
 nullif(current_setting('app.actor_id',true),'')::uuid,nullif(current_setting('app.arena_id',true),'')::uuid);
 IF TG_OP='UPDATE' AND OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
  UPDATE iam.user_sessions SET revoked_at=CURRENT_TIMESTAMP WHERE user_id=NEW.id AND revoked_at IS NULL;
 END IF;
 RETURN NEW;
END $$;
CREATE FUNCTION asset.prepare_bird_master() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='UPDATE' THEN
  IF NEW.id<>OLD.id OR NEW.bird_code<>OLD.bird_code OR NEW.version<>OLD.version THEN RAISE EXCEPTION 'Bird ID/code/version is system managed' USING ERRCODE='23514'; END IF;
  IF asset.bird_master_snapshot(NEW)=asset.bird_master_snapshot(OLD) THEN
   NEW.version:=OLD.version; NEW.updated_at:=OLD.updated_at; NEW.deleted_at:=OLD.deleted_at; RETURN NEW;
  END IF;
  NEW.version:=OLD.version+1;
 ELSE
  IF NEW.version<>1 THEN RAISE EXCEPTION 'New master version must be 1' USING ERRCODE='23514'; END IF;
 END IF;
 IF nullif(current_setting('app.actor_id',true),'') IS NULL THEN RAISE EXCEPTION 'Master mutation requires transaction actor context' USING ERRCODE='23514'; END IF;
 NEW.updated_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE FUNCTION asset.record_bird_master() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='UPDATE' AND NEW.version=OLD.version THEN RETURN NEW; END IF;
 INSERT INTO asset.bird_master_history(bird_id,version,snapshot,change_type,actor_id,arena_id)
 VALUES(NEW.id,NEW.version,asset.bird_master_snapshot(NEW),
 CASE WHEN TG_OP='INSERT' THEN 'CREATE' WHEN NEW.deleted_at IS NOT NULL THEN 'DELETE' ELSE 'UPDATE' END,
 nullif(current_setting('app.actor_id',true),'')::uuid,nullif(current_setting('app.arena_id',true),'')::uuid);
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION iam.prepare_user_master(),iam.record_user_master(),asset.prepare_bird_master(),asset.record_bird_master() FROM PUBLIC;
CREATE TRIGGER users_prepare_master BEFORE INSERT OR UPDATE ON iam.users FOR EACH ROW EXECUTE FUNCTION iam.prepare_user_master();
CREATE TRIGGER users_record_master AFTER INSERT OR UPDATE ON iam.users FOR EACH ROW EXECUTE FUNCTION iam.record_user_master();
CREATE TRIGGER birds_prepare_master BEFORE INSERT OR UPDATE ON asset.birds FOR EACH ROW EXECUTE FUNCTION asset.prepare_bird_master();
CREATE TRIGGER birds_record_master AFTER INSERT OR UPDATE ON asset.birds FOR EACH ROW EXECUTE FUNCTION asset.record_bird_master();
REVOKE ALL ON iam.user_master_history,asset.bird_master_history FROM PUBLIC,kicau_cloud_app;
GRANT SELECT ON iam.user_master_history,asset.bird_master_history TO kicau_cloud_app;
REVOKE DELETE,TRUNCATE ON iam.users,asset.birds FROM kicau_cloud_app;
REVOKE UPDATE,DELETE,TRUNCATE ON asset.bird_ownership_history FROM kicau_cloud_app;

-- Merge former admin roles without dropping assignments or creating duplicates.
DO $$ BEGIN
 IF EXISTS(SELECT FROM iam.roles WHERE name NOT IN ('SUPERADMIN','EO_ADMIN','ADMIN','OPERATOR','JURI','PESERTA')) THEN
  RAISE EXCEPTION 'Unexpected business roles: review before applying 010';
 END IF;
END $$;
INSERT INTO iam.roles(name) VALUES('ADMIN') ON CONFLICT(name) DO NOTHING;
INSERT INTO iam.user_roles(user_id,role_id)
 SELECT ur.user_id,a.id FROM iam.user_roles ur JOIN iam.roles old ON old.id=ur.role_id
 CROSS JOIN iam.roles a WHERE old.name IN ('SUPERADMIN','EO_ADMIN') AND a.name='ADMIN'
 ON CONFLICT(user_id,role_id) DO NOTHING;
INSERT INTO iam.arena_user_roles(arena_id,user_id,role_id)
 SELECT ur.arena_id,ur.user_id,a.id FROM iam.arena_user_roles ur JOIN iam.roles old ON old.id=ur.role_id
 CROSS JOIN iam.roles a WHERE old.name IN ('SUPERADMIN','EO_ADMIN') AND a.name='ADMIN'
 ON CONFLICT(arena_id,user_id,role_id) DO NOTHING;
DELETE FROM iam.user_roles WHERE role_id IN (SELECT id FROM iam.roles WHERE name IN ('SUPERADMIN','EO_ADMIN'));
DELETE FROM iam.arena_user_roles WHERE role_id IN (SELECT id FROM iam.roles WHERE name IN ('SUPERADMIN','EO_ADMIN'));
DELETE FROM iam.roles WHERE name IN ('SUPERADMIN','EO_ADMIN');

CREATE OR REPLACE VIEW cloud_read.v_users AS
 SELECT id,name,phone,email,email_verified_at,is_active,is_eo,password_hash IS NOT NULL AS has_password,created_at,updated_at,version,deleted_at FROM iam.users;
CREATE OR REPLACE VIEW cloud_read.v_birds AS
 SELECT b.id,b.bird_code,b.name,b.owner_id,u.name AS owner_name,b.bird_species_id,s.name AS bird_species_name,
 b.created_at,b.updated_at,b.version,b.deleted_at
 FROM asset.birds b JOIN iam.users u ON u.id=b.owner_id JOIN asset.bird_species s ON s.id=b.bird_species_id;
CREATE VIEW cloud_read.v_user_master_history AS SELECT id,user_id,version,snapshot,change_type,actor_id,arena_id,recorded_at FROM iam.user_master_history;
CREATE VIEW cloud_read.v_bird_master_history AS SELECT id,bird_id,version,snapshot,change_type,actor_id,arena_id,recorded_at FROM asset.bird_master_history;
GRANT SELECT ON cloud_read.v_user_master_history,cloud_read.v_bird_master_history TO kicau_cloud_app,kicau_cloud_admin_reader;
COMMENT ON TABLE iam.user_master_history IS 'Append-only allowlisted identity snapshots. app.actor_id and optional app.arena_id are trusted UseCase transaction context, never authorization from browser input.';
COMMENT ON TABLE asset.bird_master_history IS 'Append-only current-master versions, distinct from ownership-period and award history. Unchanged business snapshots do not create versions.';
