-- +goose Up
-- Columns metric_type and label are now included in the initial schema (001).
-- This migration is kept as a no-op for goose version tracking compatibility.
SELECT 1;

-- +goose Down
SELECT 1;
