-- +goose Up
CREATE SCHEMA monitoring;

CREATE TABLE monitoring.monitors (
    id uuid PRIMARY KEY,
    target_url text NOT NULL,
    created_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE monitoring.monitors;
DROP SCHEMA monitoring;
