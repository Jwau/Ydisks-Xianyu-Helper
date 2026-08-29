package db

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// CardImage 是本地上传的图片卡密图片文件；data 以 base64 文本持久化，读取时解码为字节。
type CardImage struct {
	// ID 是图片记录主键，卡密表通过 image_id 引用。
	ID int64
	// UserID 是上传者标识，读取时做归属校验。
	UserID int64
	// Filename 是上传时的原始文件名。
	Filename string
	// ContentType 是检测出的图片媒体类型。
	ContentType string
	// ByteSize 是解码后的字节大小。
	ByteSize int
	// CreatedAt 是上传时间（Unix 秒）。
	CreatedAt int64
	// Data 是解码后的图片字节；仅在单条查询时填充，列表查询不读取。
	Data []byte
}

// CreateImage 保存一张上传图片并返回记录 ID；data 以 base64 文本入库。
func (c *Cards) CreateImage(ctx context.Context, img *CardImage) (int64, error) {
	if img == nil || img.UserID <= 0 {
		return 0, errors.New("上传图片缺少有效所有者")
	}
	// encoded 是图片字节的 base64 编码文本。
	encoded := base64.StdEncoding.EncodeToString(img.Data)
	// createdAt 是上传时间（Unix 秒）。
	createdAt := time.Now().Unix()
	// id、err 保存按方言取回的新记录主键和插入错误；Postgres 走 RETURNING id。
	id, err := insertReturningID(ctx, c.DB, c.Dialect,
		`INSERT INTO card_images (user_id, filename, content_type, byte_size, data, created_at)
		 VALUES (?,?,?,?,?,?)`,
		img.UserID, img.Filename, img.ContentType, len(img.Data), encoded, createdAt)
	if err != nil {
		return 0, fmt.Errorf("保存上传图片: %w", err)
	}
	return id, nil
}

// GetImageForUser 读取归属于指定用户的上传图片并解码字节；不存在或越权时区分错误语义。
func (c *Cards) GetImageForUser(ctx context.Context, userID, imageID int64) (*CardImage, error) {
	if userID <= 0 || imageID <= 0 {
		return nil, ErrNotFound
	}
	// img 是查询出的图片记录。
	img := &CardImage{}
	// encoded 是数据库中的 base64 文本。
	var encoded string
	// err 表示图片记录查询错误。
	err := c.DB.QueryRowContext(ctx,
		`SELECT id, user_id, filename, content_type, byte_size, data, created_at
		 FROM card_images WHERE id=? AND user_id=?`, imageID, userID).Scan(
		&img.ID, &img.UserID, &img.Filename, &img.ContentType, &img.ByteSize, &encoded, &img.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	// data、decodeErr 是解码后的图片字节和解码错误。
	data, decodeErr := base64.StdEncoding.DecodeString(encoded)
	if decodeErr != nil {
		return nil, fmt.Errorf("解码上传图片: %w", decodeErr)
	}
	img.Data = data
	return img, nil
}

// DeleteImagesForCardUser 删除指定用户的一批上传图片记录；卡密删除时清理其绑定的上传图片。
func (c *Cards) DeleteImage(ctx context.Context, userID, imageID int64) error {
	// _ 忽略删除结果统计；err 表示删除上传图片记录的错误。
	_, err := c.DB.ExecContext(ctx, `DELETE FROM card_images WHERE id=? AND user_id=?`, imageID, userID)
	return err
}
