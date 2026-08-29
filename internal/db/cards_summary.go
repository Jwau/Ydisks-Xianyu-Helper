package db

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// CardAPIConfigSummary 是数据库向普通查询路径提供的 API 卡配置摘要。
// Headers、Params 和 Body 是所有者编辑弹窗回显所需的已保存请求模板；
// 它们只在归属校验后的查询路径返回，发货执行仍走 GetForDelivery 专用路径。
type CardAPIConfigSummary struct {
	// URL 是 API 端点地址，不包含请求模板内容。
	URL string `json:"url"`
	// Method 是 API 请求方法。
	Method string `json:"method"`
	// TimeoutSeconds 是请求超时时间，单位为秒。
	TimeoutSeconds int `json:"timeout_seconds"`
	// Headers 是所有者编辑回显用的请求头模板；未配置或解析失败时为空对象。
	Headers map[string]any `json:"headers"`
	// Params 是所有者编辑回显用的查询参数模板；未配置或解析失败时为空对象。
	Params map[string]any `json:"params"`
	// Body 是所有者编辑回显用的请求正文模板；未配置或解析失败时为空对象。
	Body map[string]any `json:"body"`
	// MessageTemplate 是可选的发货文案模板；空值表示直接发送接口提取内容。
	MessageTemplate string `json:"message_template,omitempty"`
	// ResponsePath 是响应提取路径。
	ResponsePath string `json:"response_path,omitempty"`
	// RetryEnabled 表示是否启用幂等重试。
	RetryEnabled bool `json:"retry_enabled"`
	// HeadersConfigured 表示是否配置了请求头模板。
	HeadersConfigured bool `json:"headers_configured"`
	// ParamsConfigured 表示是否配置了请求参数模板。
	ParamsConfigured bool `json:"params_configured"`
	// Ready 表示配置能否进入自动化规则选择。
	Ready bool `json:"ready"`
	// ValidationError 是不包含秘密值的配置错误。
	ValidationError string `json:"validation_error,omitempty"`
}

// GetForDelivery 读取自动发货专用的完整卡券配置，普通查询不得调用此方法。
func (c *Cards) GetForDelivery(ctx context.Context, cardID int64) (*CardFull, error) {
	return c.Get(ctx, cardID)
}

// GetSummary 读取单个卡券的所有者查询摘要；API 卡返回含请求模板的完整配置视图，原配置文本不离开数据库层。
func (c *Cards) GetSummary(ctx context.Context, cardID int64) (*CardFull, error) {
	// card 是内部完整读取后立即转为摘要视图的卡券记录。
	card, err := c.Get(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card.Type == "api" {
		card.APIConfigSummary = summarizeCardAPIConfig(card.Type, card.APIConfig)
		card.APIConfig = ""
	}
	return card, nil
}

// AllForUserSummary 读取用户卡券列表的摘要视图；API 卡返回含请求模板的完整配置视图，原配置文本不离开数据库层。
func (c *Cards) AllForUserSummary(ctx context.Context, userID int64) ([]CardFull, error) {
	// cards 是内部完整读取后逐条转为摘要视图的卡券记录。
	cards, err := c.AllForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	// i 表示当前需要脱敏处理的卡券列表下标。
	for i := range cards {
		if cards[i].Type == "api" {
			cards[i].APIConfigSummary = summarizeCardAPIConfig(cards[i].Type, cards[i].APIConfig)
			cards[i].APIConfig = ""
		}
	}
	return cards, nil
}

// summarizeCardAPIConfig 将已解密的 API 配置转换为所有者查询视图；
// 请求头、参数和正文模板原样回读供编辑使用，原始配置 JSON 不进入上层。
func summarizeCardAPIConfig(cardType, raw string) *CardAPIConfigSummary {
	if cardType != "api" {
		return nil
	}
	// summary 是所有者查询使用的配置结果；模板字段初始化为空对象，保证 JSON 输出不是 null。
	summary := &CardAPIConfigSummary{Headers: map[string]any{}, Params: map[string]any{}, Body: map[string]any{}}
	// fields 是用于解析公开字段和请求模板的 JSON 对象。
	var fields map[string]json.RawMessage
	// err 表示 API 配置 JSON 解析错误。
	if err := json.Unmarshal([]byte(raw), &fields); err != nil || fields == nil {
		summary.ValidationError = "API 配置 JSON 无效"
		return summary
	}
	summary.URL = summaryRawString(fields["url"])
	summary.Method = strings.ToUpper(summaryRawString(fields["method"]))
	if summary.Method == "" {
		summary.Method = "GET"
	}
	summary.TimeoutSeconds = parseSummaryTimeout(fields)
	summary.Headers = apiConfigTemplate(fields["headers"])
	summary.Params = apiConfigTemplate(fields["params"])
	summary.Body = apiConfigTemplate(fields["body"])
	summary.MessageTemplate = summaryRawString(fields["message_template"])
	summary.ResponsePath = summaryRawString(fields["response_path"])
	summary.RetryEnabled = strings.EqualFold(summaryRawString(fields["retry_enabled"]), "true")
	summary.HeadersConfigured = templateConfigured(fields["headers"])
	summary.ParamsConfigured = templateConfigured(fields["params"])
	// templates 是校验幂等占位符所需的请求头和参数模板，与摘要回显字段同源。
	templates := map[string]any{"headers": summary.Headers, "params": summary.Params}
	// err 表示摘要公开字段或重试约束校验错误。
	if err := validateSummaryAPIConfig(*summary, templates); err != nil {
		summary.ValidationError = err.Error()
		return summary
	}
	summary.Ready = true
	return summary
}

// apiConfigTemplate 将 API 模板字段解析为对象；空值、null 或非对象一律返回空对象。
func apiConfigTemplate(raw json.RawMessage) map[string]any {
	// value 保存模板字段的动态 JSON 对象。
	var value map[string]any
	if len(bytes.TrimSpace(raw)) == 0 || json.Unmarshal(raw, &value) != nil || value == nil {
		return map[string]any{}
	}
	return value
}

// parseSummaryTimeout 兼容历史配置中的 timeout 字段并返回摘要超时。
func parseSummaryTimeout(fields map[string]json.RawMessage) int {
	// raw 保存优先使用的新字段或历史字段。
	raw := fields["timeout_seconds"]
	if len(raw) == 0 {
		raw = fields["timeout"]
	}
	// value 保存兼容超时字段的文本值。
	value := summaryRawString(raw)
	if value == "" {
		return 10
	}
	// timeout 保存解析后的秒数。
	timeout, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return timeout
}

// summaryRawString 读取摘要解析所需的 JSON 标量文本，不保留原始模板值。
func summaryRawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// value 保存字符串类型 JSON 标量。
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	// generic 保存数字或布尔类型 JSON 标量。
	var generic any
	if json.Unmarshal(raw, &generic) == nil {
		return fmt.Sprint(generic)
	}
	return ""
}

// templateConfigured 判断模板字段是否存在且不是空对象或空字符串。
func templateConfigured(raw json.RawMessage) bool {
	// value 保存模板字段的紧凑 JSON 形式，仅用于判断是否已配置。
	value := strings.TrimSpace(string(raw))
	return value != "" && value != "null" && value != "{}" && value != `""`
}

// validateSummaryAPIConfig 校验摘要中的公开字段及重试占位符约束。
func validateSummaryAPIConfig(summary CardAPIConfigSummary, templates map[string]any) error {
	// parsed 保存公开 API 地址解析结果。
	parsed, err := url.Parse(strings.TrimSpace(summary.URL))
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("API 地址必须是 HTTP(S) 地址且不能包含用户凭据")
	}
	if summary.Method != "GET" && summary.Method != "POST" {
		return errors.New("API 请求方法只能是 GET 或 POST")
	}
	if summary.TimeoutSeconds < 1 || summary.TimeoutSeconds > 60 {
		return errors.New("API 超时时间必须在 1 到 60 秒之间")
	}
	if summary.RetryEnabled && !summaryTemplateContains(templates, "{idempotency_key}") {
		return errors.New("启用 API 重试时，请求头或请求参数必须包含 {idempotency_key}")
	}
	return nil
}

// summaryTemplateContains 递归检查模板是否包含指定占位符。
func summaryTemplateContains(value any, placeholder string) bool {
	// current 保存当前递归节点的模板值。
	switch current := value.(type) {
	case string:
		return strings.Contains(current, placeholder)
	case map[string]any:
		// child 表示当前对象中的模板子节点。
		for _, child := range current {
			if summaryTemplateContains(child, placeholder) {
				return true
			}
		}
	case []any:
		// child 表示当前数组中的模板子节点。
		for _, child := range current {
			if summaryTemplateContains(child, placeholder) {
				return true
			}
		}
	}
	return false
}
