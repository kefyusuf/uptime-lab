-- +goose Up
ALTER TABLE monitoring.monitors ADD COLUMN paused boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE monitoring.monitors DROP COLUMN paused;
