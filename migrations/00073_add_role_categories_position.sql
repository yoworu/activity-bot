-- +goose Up
ALTER TABLE role_categories ADD COLUMN position integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE role_categories DROP COLUMN position;