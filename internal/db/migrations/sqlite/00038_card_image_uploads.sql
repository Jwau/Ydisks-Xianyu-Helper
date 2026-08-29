-- +goose Up
CREATE TABLE card_images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT '',
    byte_size INTEGER NOT NULL DEFAULT 0,
    data TEXT NOT NULL,
    created_at BIGINT NOT NULL
);
CREATE INDEX idx_card_images_user ON card_images(user_id, id DESC);
ALTER TABLE cards ADD COLUMN image_id INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE cards DROP COLUMN image_id;
DROP TABLE IF EXISTS card_images;
