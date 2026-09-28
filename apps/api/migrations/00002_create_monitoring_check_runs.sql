-- +goose Up
CREATE TABLE monitoring.check_runs (
    id uuid PRIMARY KEY,
    monitor_id uuid NOT NULL REFERENCES monitoring.monitors(id),
    issued_at timestamptz NOT NULL,
    deadline_at timestamptz NOT NULL,
    completed_at timestamptz NULL,
    result_kind text NULL,
    http_status integer NULL,
    duration_ms integer NULL,

    CONSTRAINT check_runs_deadline_after_issued
        CHECK (deadline_at > issued_at),

    CONSTRAINT check_runs_completion_not_before_issued
        CHECK (completed_at IS NULL OR completed_at >= issued_at),

    CONSTRAINT check_runs_state_shape
        CHECK (
            (
                completed_at IS NULL
                AND result_kind IS NULL
                AND http_status IS NULL
                AND duration_ms IS NULL
            )
            OR
            (
                completed_at IS NOT NULL
                AND result_kind IS NOT NULL
                AND (
                    (
                        result_kind = 'http_response'
                        AND http_status IS NOT NULL
                        AND http_status BETWEEN 100 AND 599
                        AND duration_ms IS NOT NULL
                        AND duration_ms BETWEEN 0 AND 20000
                    )
                    OR
                    (
                        result_kind IN (
                            'dns_error',
                            'policy_rejected',
                            'timeout',
                            'connect_error',
                            'tls_error',
                            'protocol_error',
                            'internal_error'
                        )
                        AND http_status IS NULL
                        AND duration_ms IS NOT NULL
                        AND duration_ms BETWEEN 0 AND 20000
                    )
                    OR
                    (
                        result_kind = 'worker_timeout'
                        AND http_status IS NULL
                        AND duration_ms IS NULL
                    )
                )
            )
        )
);

CREATE UNIQUE INDEX check_runs_one_pending_per_monitor_idx
    ON monitoring.check_runs (monitor_id)
    WHERE completed_at IS NULL;

CREATE INDEX check_runs_latest_terminal_idx
    ON monitoring.check_runs (monitor_id, completed_at DESC)
    WHERE completed_at IS NOT NULL;

-- +goose Down
DROP TABLE monitoring.check_runs;
