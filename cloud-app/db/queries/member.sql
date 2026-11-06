-- Examples for one session. Repository Go can use the SQL bodies directly.

PREPARE list_plans AS
SELECT * FROM cloud_read.v_plans
ORDER BY name, id LIMIT $1 OFFSET $2;

PREPARE get_plans AS
SELECT * FROM cloud_read.v_plans WHERE id = $1;

PREPARE list_user_subscriptions AS
SELECT * FROM cloud_read.v_user_subscriptions WHERE user_id = $1
ORDER BY start_date DESC, id LIMIT $2 OFFSET $3;

PREPARE get_user_subscriptions AS
SELECT * FROM cloud_read.v_user_subscriptions WHERE id = $1;

PREPARE create_plan AS
INSERT INTO member.plans (name, quota_birds, max_events, price)
VALUES ($1, $2, $3, $4) RETURNING id;

PREPARE update_plan AS
UPDATE member.plans SET name = $2, quota_birds = $3, max_events = $4, price = $5
WHERE id = $1 RETURNING id;

PREPARE delete_plan AS
DELETE FROM member.plans WHERE id = $1 RETURNING id;
