package adapter

import (
	"context"
	"errors"

	itemsapp "xianyu-go/internal/application/items"
	"xianyu-go/internal/db"
)

// itemMigrationRepository 实现商品迁移用例所需的持久化能力；数据存取全部委托数据库仓储。
type itemMigrationRepository struct {
	// store 是迁移事务与账号归属查询共享的数据库入口。
	store *db.Store
}

// CookieOwnedBy 判断账号是否属于指定用户。
func (r *itemMigrationRepository) CookieOwnedBy(ctx context.Context, userID int64, cookieID string) (bool, error) {
	if r == nil || r.store == nil || r.store.Cookies == nil {
		return false, errors.New("商品迁移仓储未初始化")
	}
	return r.store.Cookies.CookieOwnedBy(ctx, userID, cookieID)
}

// MigrateItems 在单一事务内执行商品迁移并返回统计。
func (r *itemMigrationRepository) MigrateItems(ctx context.Context, fromCookieID, toCookieID string, itemIDs []string) (itemsapp.ItemMigrationResult, error) {
	if r == nil || r.store == nil || r.store.Items == nil {
		return itemsapp.ItemMigrationResult{}, errors.New("商品迁移仓储未初始化")
	}
	// result 是数据库层返回的迁移统计；两层模型字段一一对应。
	result, err := r.store.Items.MigrateItems(ctx, fromCookieID, toCookieID, itemIDs)
	if err != nil {
		return itemsapp.ItemMigrationResult{}, err
	}
	return itemsapp.ItemMigrationResult{
		Migrated: result.Migrated, Skipped: result.Skipped, RulesMoved: result.RulesMoved,
		KeywordsMoved: result.KeywordsMoved, RepliesMoved: result.RepliesMoved,
	}, nil
}
