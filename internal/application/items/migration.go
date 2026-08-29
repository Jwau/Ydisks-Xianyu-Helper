// migration.go: 商品迁移用例 —— 把选定商品及其按商品绑定的配置从一个账号迁到另一个账号。
package items

import (
	"context"
	"errors"
	"fmt"
)

// 商品迁移允许的最大单批商品数量，避免一次性事务过长。
const maxMigrateItemIDs = 500

// 商品迁移用例的稳定错误。
var (
	// ErrInvalidCaller 表示调用方未提供有效的用户身份。
	ErrInvalidCaller = errors.New("调用方身份无效")
	// ErrMigrationSameAccount 表示源账号与目标账号相同。
	ErrMigrationSameAccount = errors.New("源账号与目标账号不能相同")
	// ErrMigrationEmptySelection 表示没有选择任何要迁移的商品。
	ErrMigrationEmptySelection = errors.New("请选择要迁移的商品")
	// ErrMigrationSourceNotFound 表示源账号不存在或不属于当前用户。
	ErrMigrationSourceNotFound = errors.New("源账号不存在或无权访问")
	// ErrMigrationTargetNotFound 表示目标账号不存在或不属于当前用户。
	ErrMigrationTargetNotFound = errors.New("目标账号不存在或无权访问")
)

// ItemMigrationResult 是一次商品迁移的各层影响统计。
type ItemMigrationResult struct {
	// Migrated 是成功迁移的商品数量。
	Migrated int64 `json:"migrated"`
	// Skipped 是因目标账号已存在同 ID 商品而跳过的商品 ID 列表。
	Skipped []string `json:"skipped"`
	// RulesMoved 是随之改绑的自动化规则数量。
	RulesMoved int64 `json:"rules_moved"`
	// KeywordsMoved 是随之改绑的关键词回复数量。
	KeywordsMoved int64 `json:"keywords_moved"`
	// RepliesMoved 是随之改绑的指定商品回复数量。
	RepliesMoved int64 `json:"replies_moved"`
}

// ItemMigrationRepository 定义商品迁移用例所需的最小持久化能力。
type ItemMigrationRepository interface {
	// CookieOwnedBy 判断账号是否属于指定用户。
	CookieOwnedBy(ctx context.Context, userID int64, cookieID string) (bool, error)
	// MigrateItems 在单一事务内执行迁移并返回统计。
	MigrateItems(ctx context.Context, fromCookieID, toCookieID string, itemIDs []string) (ItemMigrationResult, error)
}

// ItemMigrationService 编排商品迁移的授权校验与持久化。
type ItemMigrationService struct {
	// repository 是迁移用例依赖的窄持久化 Port。
	repository ItemMigrationRepository
}

// NewItemMigrationService 创建商品迁移服务；仓储缺失时在调用期报错。
func NewItemMigrationService(repository ItemMigrationRepository) *ItemMigrationService {
	return &ItemMigrationService{repository: repository}
}

// Migrate 把 itemIDs 从源账号迁移到目标账号；两个账号都必须属于当前用户。
func (s *ItemMigrationService) Migrate(ctx context.Context, userID int64, fromCookieID, toCookieID string, itemIDs []string) (ItemMigrationResult, error) {
	// result 是迁移成功后的统计。
	var result ItemMigrationResult
	if s == nil || s.repository == nil {
		return result, errors.New("商品迁移仓储未初始化")
	}
	if userID <= 0 {
		return result, ErrInvalidCaller
	}
	if fromCookieID == toCookieID {
		return result, ErrMigrationSameAccount
	}
	if len(itemIDs) == 0 {
		return result, ErrMigrationEmptySelection
	}
	if len(itemIDs) > maxMigrateItemIDs {
		return result, fmt.Errorf("单次最多迁移 %d 个商品", maxMigrateItemIDs)
	}
	// sourceOwned、err 是源账号归属校验结果和错误。
	sourceOwned, err := s.repository.CookieOwnedBy(ctx, userID, fromCookieID)
	if err != nil {
		return result, err
	}
	if !sourceOwned {
		return result, ErrMigrationSourceNotFound
	}
	// targetOwned、err 是目标账号归属校验结果和错误。
	targetOwned, err := s.repository.CookieOwnedBy(ctx, userID, toCookieID)
	if err != nil {
		return result, err
	}
	if !targetOwned {
		return result, ErrMigrationTargetNotFound
	}
	// seen 记录已出现的商品 ID 用于去重。
	seen := make(map[string]struct{}, len(itemIDs))
	// cleaned 是去重去空后的最终迁移列表。
	cleaned := make([]string, 0, len(itemIDs))
	// raw 表示当前遍历过程中的原始商品 ID；duplicate 表示该 ID 是否已出现过。
	for _, raw := range itemIDs {
		if raw == "" {
			continue
		}
		// duplicate 表示该商品 ID 是否已经出现过。
		if _, duplicate := seen[raw]; duplicate {
			continue
		}
		seen[raw] = struct{}{}
		cleaned = append(cleaned, raw)
	}
	if len(cleaned) == 0 {
		return result, ErrMigrationEmptySelection
	}
	return s.repository.MigrateItems(ctx, fromCookieID, toCookieID, cleaned)
}
