package cards

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestServiceCopyDuplicatesOwnedCard 验证复制用例的归属校验、副本命名和内容原样复制。
func TestServiceCopyDuplicatesOwnedCard(t *testing.T) {
	// sourceCard 是包含全部业务字段的复制源卡密组。
	sourceCard := Card{
		ID: 9, Name: "库存源", Type: "data",
		APIConfig:   `{"url":"https://example.com/api","method":"POST","timeout_seconds":10,"headers":{"Authorization":"secret"},"params":{},"body":{},"content_type":"application/json"}`,
		DataContent: "A\nB", Description: "说明", Enabled: true, DelaySeconds: 5,
		IsMultiSpec: true, SpecName: "颜色", SpecValue: "红", UserID: 7,
	}
	// stub 是记录复制写入结果的仓储替身。
	stub := &cardRepositoryStub{card: sourceCard, createdID: 12}
	// service 是注入替身的卡密应用服务。
	service := NewService(stub)
	// newID、copyErr 保存复制结果和错误。
	newID, copyErr := service.Copy(context.Background(), 7, 9)
	if copyErr != nil || newID != 12 {
		t.Fatalf("复制卡密组失败 id=%d err=%v", newID, copyErr)
	}
	// copied 是服务实际提交创建的副本卡密组。
	copied := stub.createdCard
	if copied.Name != "库存源-副本" {
		t.Fatalf("副本名称错误: %q", copied.Name)
	}
	if copied.Type != "data" || copied.DataContent != "A\nB" || copied.Description != "说明" ||
		!copied.Enabled || copied.DelaySeconds != 5 || !copied.IsMultiSpec ||
		copied.SpecName != "颜色" || copied.SpecValue != "红" {
		t.Fatalf("副本内容与源不一致: %+v", copied)
	}
	if copied.UserID != 7 {
		t.Fatalf("副本所有者错误: %d", copied.UserID)
	}
}

// TestServiceCopyOwnershipIdentifiersAndTemplates 验证越权、无效标识和 API 模板保留边界。
func TestServiceCopyOwnershipIdentifiersAndTemplates(t *testing.T) {
	// stub 预设属于用户 7 的源卡密组。
	stub := &cardRepositoryStub{card: Card{ID: 3, Name: "源", Type: "text", TextContent: "内容", UserID: 7}, createdID: 4}
	// service 是注入替身的卡密应用服务。
	service := NewService(stub)
	// 越权复制必须被拒绝。
	if _, err := service.Copy(context.Background(), 8, 3); !errors.Is(err, ErrForbidden) {
		t.Fatalf("越权复制应返回 ErrForbidden: %v", err)
	}
	// 无效标识必须被拒绝。
	if _, err := service.Copy(context.Background(), 7, 0); !errors.Is(err, ErrInvalidCardID) {
		t.Fatalf("无效卡券标识应返回 ErrInvalidCardID: %v", err)
	}
	// API 卡复制保留敏感模板并复用 Create 的同一条校验路径。
	stub.card = Card{ID: 5, Name: "接口源", Type: "api",
		APIConfig: `{"url":"https://example.com/card","method":"GET","timeout_seconds":10,"headers":{"Authorization":"secret"},"params":{},"body":{},"content_type":"application/json"}`,
		UserID:    7}
	// newID、copyErr 保存 API 复制结果和错误。
	newID, copyErr := service.Copy(context.Background(), 7, 5)
	if copyErr != nil || newID != 4 {
		t.Fatalf("API 卡复制失败 id=%d err=%v", newID, copyErr)
	}
	if !strings.Contains(stub.createdCard.APIConfig, "secret") {
		t.Fatal("API 复制必须保留敏感模板")
	}
	// 空名称源复制后获得固定回退名称，仍能通过非空校验。
	stub.card = Card{ID: 6, Name: "", Type: "text", TextContent: "内容", UserID: 7}
	// err 保存空名称源复制的错误结果。
	if _, err := service.Copy(context.Background(), 7, 6); err != nil {
		t.Fatalf("空名称复制失败: %v", err)
	}
	if stub.createdCard.Name != "卡密组-副本" {
		t.Fatalf("空名称副本回退错误: %q", stub.createdCard.Name)
	}
}
