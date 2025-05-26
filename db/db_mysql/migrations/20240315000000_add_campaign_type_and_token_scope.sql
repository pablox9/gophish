-- +migrate Up
ALTER TABLE campaigns ADD COLUMN campaign_type TEXT;
ALTER TABLE campaigns ADD COLUMN token_scope TEXT;
UPDATE campaigns SET campaign_type = 'Standard' WHERE campaign_type IS NULL;

-- +migrate Down
ALTER TABLE campaigns DROP COLUMN campaign_type;
ALTER TABLE campaigns DROP COLUMN token_scope;
