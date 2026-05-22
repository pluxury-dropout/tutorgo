-- +goose Up
ALTER TABLE lessons ADD COLUMN room_ended_at TIMESTAMPTZ;

-- +goose DOWN
ALTER TABLE lessons DROP COLUMN room_ended_at;