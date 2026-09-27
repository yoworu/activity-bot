-- +goose Up
ALTER TABLE role_categories ADD COLUMN emoji text;

-- +goose Down
ALTER TABLE role_categories DROP COLUMN emoji;