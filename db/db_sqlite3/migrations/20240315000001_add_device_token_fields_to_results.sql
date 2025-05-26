-- +migrate Up
ALTER TABLE results ADD COLUMN device_code TEXT;
ALTER TABLE results ADD COLUMN device_code_expiry DATETIME;
ALTER TABLE results ADD COLUMN user_code TEXT;
ALTER TABLE results ADD COLUMN verification_uri TEXT;
ALTER TABLE results ADD COLUMN device_auth_interval INT;

-- +migrate Down
ALTER TABLE results DROP COLUMN device_code;
ALTER TABLE results DROP COLUMN device_code_expiry;
ALTER TABLE results DROP COLUMN user_code;
ALTER TABLE results DROP COLUMN verification_uri;
ALTER TABLE results DROP COLUMN device_auth_interval;
