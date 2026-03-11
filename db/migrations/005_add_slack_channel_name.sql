-- +goose Up
ALTER TABLE user_tag_alerts ADD COLUMN slack_channel_name TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE user_tag_alerts DROP COLUMN slack_channel_name;
