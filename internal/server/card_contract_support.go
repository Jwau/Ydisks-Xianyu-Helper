package server

import (
	"encoding/json"

	cardsapp "xianyu-go/internal/application/cards"
)

// apiCardConfigResponse 是 API 卡券查询接口返回给所有者的配置 DTO。
// Headers、Params 和 Body 是已保存请求模板的回显值，仅出现在归属校验后的查询响应中，
// 供编辑弹窗展示和测试请求复用；不包含平台账号凭证。
type apiCardConfigResponse struct {
	// URL 是固定的 HTTP(S) 请求地址，不包含请求凭据。
	URL string `json:"url"`
	// Method 是 GET 或 POST。
	Method string `json:"method"`
	// TimeoutSeconds 是单次 API 请求超时秒数。
	TimeoutSeconds int `json:"timeout_seconds"`
	// ContentType 是 POST 请求正文的 Content-Type。
	ContentType string `json:"content_type"`
	// Headers 是所有者编辑回显用的已保存请求头模板对象；键名由用户自定义，以原始 JSON 透传。
	Headers json.RawMessage `json:"headers"`
	// Params 是所有者编辑回显用的已保存查询参数模板对象；以原始 JSON 透传。
	Params json.RawMessage `json:"params"`
	// Body 是所有者编辑回显用的已保存请求正文模板对象；以原始 JSON 透传。
	Body json.RawMessage `json:"body"`
	// MessageTemplate 是可选的发货文案模板；空值表示直接发送接口提取内容。
	MessageTemplate string `json:"message_template,omitempty"`
	// ResponsePath 是可选的响应提取路径。
	ResponsePath string `json:"response_path,omitempty"`
	// RetryEnabled 表示是否启用带幂等键的暂时性失败重试。
	RetryEnabled bool `json:"retry_enabled"`
	// HeadersConfigured 表示服务端是否保存了非空请求头模板。
	HeadersConfigured bool `json:"headers_configured"`
	// ParamsConfigured 表示服务端是否保存了非空请求参数模板。
	ParamsConfigured bool `json:"params_configured"`
	// Ready 表示当前配置可被规则选择和自动化执行。
	Ready bool `json:"ready"`
	// ValidationError 保存脱敏后的配置校验错误。
	ValidationError string `json:"validation_error,omitempty"`
}

// newCardResponse 将应用层卡券模型转换为 HTTP DTO。
func newCardResponse(card cardsapp.Card) cardResponse {
	// apiConfig 是仅对 API 卡券生成的配置摘要视图。
	var apiConfig *apiCardConfigResponse
	if card.Type == "api" {
		// summary 保存应用层校验后的 API 配置状态。
		var summary cardsapp.APIConfigSummary
		if card.APIConfigSummary != nil {
			summary = *card.APIConfigSummary
		} else {
			summary = cardsapp.SummarizeAPIConfig(card.APIConfig)
		}
		apiConfig = &apiCardConfigResponse{
			URL: summary.URL, Method: summary.Method, TimeoutSeconds: summary.TimeoutSeconds, ContentType: summary.ContentType,
			Headers: nonNilAPITemplateJSON(summary.Headers), Params: nonNilAPITemplateJSON(summary.Params), Body: nonNilAPITemplateJSON(summary.Body),
			MessageTemplate: summary.MessageTemplate,
			ResponsePath:    summary.ResponsePath, RetryEnabled: summary.RetryEnabled,
			HeadersConfigured: summary.HeadersConfigured, ParamsConfigured: summary.ParamsConfigured,
			Ready: summary.Ready, ValidationError: summary.ValidationError,
		}
	}
	return cardResponse{
		ID: card.ID, Name: card.Name, Type: card.Type, APIConfig: apiConfig,
		TextContent: card.TextContent, DataContent: card.DataContent, ImageURL: card.ImageURL, ImageID: card.ImageID,
		Description: card.Description, Enabled: card.Enabled, DelaySeconds: card.DelaySeconds,
		IsMultiSpec: card.IsMultiSpec, SpecName: card.SpecName, SpecValue: card.SpecValue, UserID: card.UserID,
	}
}

// nonNilAPITemplateJSON 把模板对象序列化为原始 JSON；空模板固定输出空对象而不是 null。
func nonNilAPITemplateJSON(value map[string]any) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage("{}")
	}
	// encoded 是模板对象的 JSON 编码结果；编码失败按空对象兜底。
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return encoded
}

// newCardResponses 批量转换应用层卡券列表，避免 HTTP 层暴露数据库模型。
func newCardResponses(cards []cardsapp.Card) []cardResponse {
	// result 是转换后的卡券 DTO 列表。
	result := make([]cardResponse, 0, len(cards))
	// card 是当前待转换的卡券应用模型。
	for _, card := range cards {
		result = append(result, newCardResponse(card))
	}
	return result
}
