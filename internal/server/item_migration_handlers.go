package server

// item_migration_handlers.go: 商品跨账号迁移端点。

import (
	"errors"
	"net/http"

	itemapp "xianyu-go/internal/application/items"
)

// itemMigrationRequest 是商品迁移请求的具名 DTO。
type itemMigrationRequest struct {
	// FromCookieID 是源账号标识。
	FromCookieID string `json:"from_cookie_id"`
	// ToCookieID 是目标账号标识。
	ToCookieID string `json:"to_cookie_id"`
	// ItemIDs 是待迁移的商品 ID 列表。
	ItemIDs []string `json:"item_ids"`
}

// itemMigrationResponse 是商品迁移结果的具名响应。
type itemMigrationResponse struct {
	// Success 表示迁移是否完成。
	Success bool `json:"success"`
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

// migrateItems 把选定商品从源账号迁移到目标账号；两个账号都必须属于当前用户。
func (s *Server) migrateItems(w http.ResponseWriter, r *http.Request) {
	// sess 是当前认证会话。
	sess := authSess(r)
	if sess == nil {
		writeErr(w, http.StatusUnauthorized, "未授权访问")
		return
	}
	// req 是迁移请求 DTO。
	var req itemMigrationRequest
	// decodeErr 表示请求 JSON 解码错误。
	if decodeErr := decodeJSON(r, &req); decodeErr != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	// result、err 是迁移用例返回的统计和业务错误。
	result, err := s.itemMigrationApplication().Migrate(r.Context(), sess.UserID, req.FromCookieID, req.ToCookieID, req.ItemIDs)
	if err != nil {
		switch {
		case errors.Is(err, itemapp.ErrMigrationSameAccount), errors.Is(err, itemapp.ErrMigrationEmptySelection):
			writeErr(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, itemapp.ErrMigrationSourceNotFound), errors.Is(err, itemapp.ErrMigrationTargetNotFound):
			writeErr(w, http.StatusNotFound, err.Error())
		case errors.Is(err, itemapp.ErrInvalidCaller):
			writeErr(w, http.StatusUnauthorized, err.Error())
		default:
			if s.Logger != nil {
				s.Logger.Warn("商品迁移失败", "user_id", sess.UserID, "err", err.Error())
			}
			writeErr(w, http.StatusInternalServerError, "商品迁移失败")
		}
		return
	}
	if s.Logger != nil {
		s.Logger.Info("商品迁移完成",
			"user_id", sess.UserID,
			"migrated", result.Migrated,
			"skipped", len(result.Skipped),
			"rules_moved", result.RulesMoved,
		)
	}
	// skipped 保证空结果序列化为空数组而不是 null。
	skipped := result.Skipped
	if skipped == nil {
		skipped = []string{}
	}
	writeJSON(w, http.StatusOK, itemMigrationResponse{
		Success: true, Migrated: result.Migrated, Skipped: skipped,
		RulesMoved: result.RulesMoved, KeywordsMoved: result.KeywordsMoved, RepliesMoved: result.RepliesMoved,
	})
}

// itemMigrationApplication 返回商品迁移用例。
func (s *Server) itemMigrationApplication() ItemMigrationPort {
	return s.applicationServiceSet().itemMigration
}
