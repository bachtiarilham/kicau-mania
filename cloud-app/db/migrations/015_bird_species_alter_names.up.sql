ALTER TABLE asset.bird_species
  ADD COLUMN alter_name_1 text,
  ADD COLUMN alter_name_2 text,
  ADD COLUMN alter_name_3 text;

CREATE OR REPLACE VIEW cloud_read.v_bird_species AS
SELECT id, name, created_at, updated_at, alter_name_1, alter_name_2, alter_name_3
FROM asset.bird_species;
