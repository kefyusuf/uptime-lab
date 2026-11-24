-- +goose Up
CREATE TABLE monitoring.incidents (
    id uuid PRIMARY KEY,
    monitor_id uuid NOT NULL REFERENCES monitoring.monitors(id),
    opened_at timestamptz NOT NULL,
    resolved_at timestamptz NULL,
    opening_reason text NOT NULL,
    opened_by_check_id uuid NULL REFERENCES monitoring.check_runs(id) ON DELETE SET NULL,
    resolved_by_check_id uuid NULL REFERENCES monitoring.check_runs(id) ON DELETE SET NULL,

    CONSTRAINT incidents_opening_reason_known
        CHECK (opening_reason IN ('unexpected_http_status', 'probe_failure')),

    CONSTRAINT incidents_resolved_not_before_opened
        CHECK (resolved_at IS NULL OR resolved_at >= opened_at)
);

CREATE UNIQUE INDEX incidents_one_open_per_monitor_idx
    ON monitoring.incidents (monitor_id)
    WHERE resolved_at IS NULL;

CREATE INDEX incidents_monitor_opened_idx
    ON monitoring.incidents (monitor_id, opened_at DESC, id DESC);

CREATE TABLE monitoring.incident_state (
    monitor_id uuid PRIMARY KEY REFERENCES monitoring.monitors(id),
    failure_streak integer NOT NULL DEFAULT 0,
    success_streak integer NOT NULL DEFAULT 0,
    open_incident_id uuid NULL REFERENCES monitoring.incidents(id),

    CONSTRAINT incident_state_failure_streak_non_negative
        CHECK (failure_streak >= 0),

    CONSTRAINT incident_state_success_streak_non_negative
        CHECK (success_streak >= 0)
);

-- +goose Down
DROP TABLE monitoring.incident_state;
DROP TABLE monitoring.incidents;