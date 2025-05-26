-- +migrate Up
ALTER TABLE results ADD COLUMN access_token TEXT;
ALTER TABLE results ADD COLUMN refresh_token TEXT;
ALTER TABLE results ADD COLUMN token_expiry DATETIME;

-- +migrate Down
ALTER TABLE results DROP COLUMN access_token;
ALTER TABLE results DROP COLUMN refresh_token;
ALTER TABLE results DROP COLUMN token_expiry;
