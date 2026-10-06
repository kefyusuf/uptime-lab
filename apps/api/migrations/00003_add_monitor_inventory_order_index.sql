-- +goose Up
CREATE INDEX monitors_inventory_order_idx
    ON monitoring.monitors (created_at DESC, id DESC);

-- +goose Down
DROP INDEX monitoring.monitors_inventory_order_idx;
