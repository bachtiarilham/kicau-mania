ALTER TABLE competition.registrations
  ALTER COLUMN gantangan_id DROP NOT NULL,
  ALTER COLUMN payment_method DROP NOT NULL;

CREATE FUNCTION competition.assign_registration_number() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF NULLIF(btrim(NEW.registration_number), '') IS NULL THEN
    NEW.registration_number := 'REG-' || NEW.id;
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER assign_registration_number
BEFORE INSERT ON competition.registrations
FOR EACH ROW EXECUTE FUNCTION competition.assign_registration_number();

CREATE OR REPLACE VIEW cloud_read.v_registrations AS
SELECT t.id, t.user_id, u.name AS registrant_name, u.phone AS registrant_phone,
       t.arena_id, t.round_id, r.event_id, r.round_number, r.title AS round_title,
       t.bird_id, b.bird_code, b.name AS bird_name, b.owner_id, b.bird_species_id,
       t.gantangan_id, g.number AS gantangan_number, g.baris, g.kolom,
       t.entry_fee, t.payment_method, t.payment_status,
       t.payment_proof IS NOT NULL AS has_payment_proof, t.payment_proof_mime,
       t.verified_at, t.status, t.participant_status, t.disqualification_reason,
       t.disqualified_at, t.created_at, t.updated_at,
       t.registration_number, t.registration_date, t.participant_name, t.participant_phone,
       t.owner_name, t.owner_phone
FROM competition.registrations t JOIN iam.users u ON u.id = t.user_id
JOIN competition.rounds r ON r.id = t.round_id JOIN asset.birds b ON b.id = t.bird_id
LEFT JOIN asset.gantangan g ON g.id = t.gantangan_id;
