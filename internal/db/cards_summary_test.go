package db

import "testing"

// TestSummarizeCardAPIConfigCarriesOwnerTemplates 验证摘要视图回显请求模板供所有者编辑使用。
func TestSummarizeCardAPIConfigCarriesOwnerTemplates(t *testing.T) {
	// raw 是已保存的完整 API 配置，包含请求头、参数和正文模板。
	raw := `{"url":"https://example.com/card","method":"POST","timeout_seconds":10,` +
		`"headers":{"Authorization":"Bearer token-value"},"params":{"code":"{order_id}"},` +
		`"body":{"order_id":"{order_id}"},"content_type":"application/json"}`
	// summary 是解析后的所有者查询视图。
	summary := summarizeCardAPIConfig("api", raw)
	if summary == nil || !summary.Ready {
		t.Fatalf("API 卡摘要应可使用: %+v", summary)
	}
	if summary.Headers["Authorization"] != "Bearer token-value" {
		t.Fatalf("请求头模板应原样回显: %+v", summary.Headers)
	}
	if summary.Params["code"] != "{order_id}" {
		t.Fatalf("查询参数模板应原样回显: %+v", summary.Params)
	}
	if summary.Body["order_id"] != "{order_id}" {
		t.Fatalf("请求正文模板应原样回显: %+v", summary.Body)
	}
	if !summary.HeadersConfigured || !summary.ParamsConfigured {
		t.Fatalf("模板配置标记错误: %+v", summary)
	}
}

// TestSummarizeCardAPIConfigInvalidConfigKeepsEmptyTemplates 验证无效配置的模板字段是空对象而不是 null。
func TestSummarizeCardAPIConfigInvalidConfigKeepsEmptyTemplates(t *testing.T) {
	// invalid 是 JSON 解析失败的配置。
	invalid := summarizeCardAPIConfig("api", "not-json")
	if invalid == nil || invalid.Ready || invalid.ValidationError == "" {
		t.Fatalf("无效配置必须带校验错误: %+v", invalid)
	}
	if invalid.Headers == nil || invalid.Params == nil || invalid.Body == nil {
		t.Fatalf("无效配置的模板字段必须是空对象: %+v", invalid)
	}
	// nonAPI 表示非 API 卡密不生成摘要。
	if summarizeCardAPIConfig("text", "{}") != nil {
		t.Fatal("非 API 卡密不应生成 API 摘要")
	}
}
