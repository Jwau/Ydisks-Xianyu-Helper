-- +goose Up
CREATE TABLE card_images (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT '',
    byte_size INT NOT NULL DEFAULT 0,
    data LONGTEXT NOT NULL,
    created_at BIGINT NOT NULL
);
CREATE INDEX idx_card_images_user ON card_images(user_id, id DESC);
ALTER TABLE cards ADD COLUMN image_id BIGINT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE cards DROP COLUMN image_id;
DROP TABLE IF EXISTS card_images;
