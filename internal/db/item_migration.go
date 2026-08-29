package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ItemMigrationResult 保存一次商品迁移的各层影响统计。
type ItemMigrationResult struct {
	// Migrated 是成功迁移到目标账号的商品数量。
	Migrated int64
	// Skipped 是因目标账号已存在同 ID 商品而跳过的商品 ID 列表。
	Skipped []string
	// RulesMoved 是随之改绑到目标账号的自动化规则数量。
	RulesMoved int64
	// KeywordsMoved 是随之改绑的关键词回复数量。
	KeywordsMoved int64
	// RepliesMoved 是随之改绑的指定商品回复数量。
	RepliesMoved int64
}

// ErrItemMigrateNothing 表示选定列表里没有任何可迁移商品。
var ErrItemMigrateNothing = errors.New("选定商品均不可迁移")

// MigrateItems 在单一事务内把选定商品从源账号改绑到目标账号，
// 并同步迁移按商品绑定的自动化规则、关键词回复和指定商品回复。
// 目标账号已存在同 ID 商品（含软删除）时跳过该商品及其绑定，保证幂等重试安全。
func (i *Items) MigrateItems(ctx context.Context, fromCookieID, toCookieID string, itemIDs []string) (ItemMigrationResult, error) {
	// result 是返回给应用层的迁移统计。
	var result ItemMigrationResult
	if fromCookieID == "" || toCookieID == "" || fromCookieID == toCookieID || len(itemIDs) == 0 {
		return result, fmt.Errorf("迁移参数无效")
	}
	// tx、err 是覆盖整批迁移的单一事务和打开错误。
	tx, err := i.DB.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	// itemID 表示当前遍历过程中的商品 ID。
	for _, itemID := range itemIDs {
		if itemID == "" {
			continue
		}
		// targetExists 表示目标账号是否已有同 ID 商品（含软删除记录）。
		var targetExists bool
		// existsErr 表示目标存在性查询错误。
		if existsErr := tx.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM item_info WHERE cookie_id=? AND item_id=?)`,
			toCookieID, itemID).Scan(&targetExists); existsErr != nil {
			return result, existsErr
		}
		if targetExists {
			result.Skipped = append(result.Skipped, itemID)
			continue
		}
		// res、updateErr 是商品行改绑结果和错误；仅未删除行参与迁移。
		res, updateErr := tx.ExecContext(ctx,
			`UPDATE item_info SET cookie_id=?, updated_at=CURRENT_TIMESTAMP
			  WHERE cookie_id=? AND item_id=? AND deleted_at IS NULL`,
			toCookieID, fromCookieID, itemID)
		if updateErr != nil {
			return result, fmt.Errorf("迁移商品 %s: %w", itemID, updateErr)
		}
		// moved 表示实际改绑的商品行数；零行说明源账号没有可迁移记录。
		moved, _ := res.RowsAffected()
		if moved == 0 {
			result.Skipped = append(result.Skipped, itemID)
			continue
		}
		result.Migrated += moved
		// rulesRes、rulesErr 是自动化规则改绑结果和错误。
		rulesRes, rulesErr := tx.ExecContext(ctx,
			`UPDATE automation_rules SET cookie_id=?, updated_at=CURRENT_TIMESTAMP
			  WHERE cookie_id=? AND item_id=? AND deleted_at IS NULL`,
			toCookieID, fromCookieID, itemID)
		if rulesErr != nil {
			return result, fmt.Errorf("迁移商品 %s 的自动化规则: %w", itemID, rulesErr)
		}
		// rulesMoved 保存实际改绑的规则行数。
		if rulesMoved, _ := rulesRes.RowsAffected(); rulesMoved > 0 {
			result.RulesMoved += rulesMoved
		}
		// keywordsRes 是关键词回复改绑结果。
		keywordsRes, keywordsErr := tx.ExecContext(ctx,
			`UPDATE keywords SET cookie_id=? WHERE cookie_id=? AND item_id=?`,
			toCookieID, fromCookieID, itemID)
		if keywordsErr != nil {
			return result, fmt.Errorf("迁移商品 %s 的关键词回复: %w", itemID, keywordsErr)
		}
		// keywordsMoved 保存实际改绑的关键词回复行数。
		if keywordsMoved, _ := keywordsRes.RowsAffected(); keywordsMoved > 0 {
			result.KeywordsMoved += keywordsMoved
		}
		// repliesRes 是指定商品回复改绑结果。
		repliesRes, repliesErr := tx.ExecContext(ctx,
			`UPDATE item_replay SET cookie_id=? WHERE cookie_id=? AND item_id=?`,
			toCookieID, fromCookieID, itemID)
		if repliesErr != nil {
			return result, fmt.Errorf("迁移商品 %s 的指定商品回复: %w", itemID, repliesErr)
		}
		// repliesMoved 保存实际改绑的指定商品回复行数。
		if repliesMoved, _ := repliesRes.RowsAffected(); repliesMoved > 0 {
			result.RepliesMoved += repliesMoved
		}
	}
	if result.Migrated == 0 && len(result.Skipped) == 0 {
		return result, ErrItemMigrateNothing
	}
	// err 表示迁移事务提交错误。
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

// CookieOwnedBy 判断账号是否属于指定用户，用于迁移前的双方归属校验。
func (c *Cookies) CookieOwnedBy(ctx context.Context, userID int64, cookieID string) (bool, error) {
	if userID <= 0 || cookieID == "" {
		return false, nil
	}
	// exists 保存归属判断结果。
	var exists bool
	// err 表示归属查询错误。
	err := c.DB.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM cookies WHERE id=? AND user_id=?)`, cookieID, userID).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return exists, nil
}
