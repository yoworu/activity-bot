-- +goose Up
ALTER TABLE chats ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- +goose Down
DROP TABLE chats;