package adapter

import (
	"context"
	"errors"

	"xianyu-go/internal/db"
)

// CardImageService 实现图片卡密本地上传图片的保存与归属读取能力；数据只经数据库存取，不落临时文件。
type CardImageService struct {
	// store 是卡券图片记录的数据库入口。
	store *db.Store
}

// NewCardImageService 创建图片卡密图片服务；缺少数据库入口时在调用期返回错误。
func NewCardImageService(store *db.Store) *CardImageService {
	return &CardImageService{store: store}
}

// Create 保存一张属于指定用户的上传图片并返回记录 ID。
func (s *CardImageService) Create(ctx context.Context, userID int64, filename, contentType string, data []byte) (int64, error) {
	if s == nil || s.store == nil || s.store.Cards == nil {
		return 0, errors.New("卡券图片服务未初始化")
	}
	if userID <= 0 {
		return 0, errors.New("上传图片缺少有效所有者")
	}
	return s.store.Cards.CreateImage(ctx, &db.CardImage{
		UserID: userID, Filename: filename, ContentType: contentType, ByteSize: len(data), Data: data,
	})
}

// GetForUser 读取归属于指定用户的上传图片；found 为 false 表示图片不存在或不属于该用户。
func (s *CardImageService) GetForUser(ctx context.Context, userID, imageID int64) (bool, string, string, []byte, error) {
	if s == nil || s.store == nil || s.store.Cards == nil {
		return false, "", "", nil, errors.New("卡券图片服务未初始化")
	}
	// image 是归属校验后的图片记录。
	image, err := s.store.Cards.GetImageForUser(ctx, userID, imageID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return false, "", "", nil, nil
		}
		return false, "", "", nil, err
	}
	return true, image.Filename, image.ContentType, image.Data, nil
}
