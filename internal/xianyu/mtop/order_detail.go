// Package mtop: 订单详情域 — mtop.idle.web.trade.order.detail 调用与重试。
package mtop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"xianyu-go/internal/xianyu/protocol"
)

// OrderDetailResult 是订单详情接口中自动发货需要的字段。
type OrderDetailResult struct {
	Quantity       string
	SpecName       string
	SpecValue      string
	OrderStatus    string
	Amount         string
	UpdatedCookies string
}

// FetchOrderDetail 获取订单真实成交价、数量、状态和规格；token 过期时自动重签重试。
func (c *ClientImpl) FetchOrderDetail(ctx context.Context, cookiesStr, orderID string) (*OrderDetailResult, error) {
	// result 是共享重试循环返回的解析结果；业务链路不需要原始响应体。
	result, err := c.fetchOrderDetailLoop(ctx, cookiesStr, orderID, nil)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// FetchOrderDetailRaw 与 FetchOrderDetail 使用相同重试语义，并额外返回最后一次调用的原始响应体。
// 仅供运维诊断工具核对平台真实返回结构；业务链路应继续使用 FetchOrderDetail。
func (c *ClientImpl) FetchOrderDetailRaw(ctx context.Context, cookiesStr, orderID string) (*OrderDetailResult, []byte, error) {
	// raw 保存最后一次调用读取到的原始响应体。
	var raw []byte
	// result、err 是共享重试循环的解析结果和错误。
	result, err := c.fetchOrderDetailLoop(ctx, cookiesStr, orderID, func(body []byte) {
		raw = body
	})
	if err != nil {
		return nil, nil, err
	}
	return result, raw, nil
}

// fetchOrderDetailLoop 是订单详情调用的共享重试循环；capture 非空时回传每次调用的原始响应体。
func (c *ClientImpl) fetchOrderDetailLoop(ctx context.Context, cookiesStr, orderID string, capture func([]byte)) (*OrderDetailResult, error) {
	// currentCookies 用于本次流程后续判断的currentCookies
	currentCookies := cookiesStr
	if // session 用于本次流程后续判断的会话
	session := cookieSessionFromContext(ctx); session != nil {
		currentCookies, _, _ = session.State()
	}
	// lastRet 用于本次流程后续判断的lastRet
	var lastRet []string
	for // attempt 用于本次流程后续判断的尝试次数
	attempt := 0; attempt < 4; attempt++ {
		// previousCookies 用于本次流程后续判断的previousCookies
		previousCookies := currentCookies
		// result、ret、updated、raw、err 是本次调用的解析结果、平台返回、更新 Cookie、原始响应体和错误。
		result, ret, updated, raw, err := c.fetchOrderDetailOnce(ctx, currentCookies, orderID)
		if err != nil {
			return nil, err
		}
		// capture 非空时把本次原始响应体回传给诊断调用方。
		if capture != nil {
			capture(raw)
		}
		lastRet = ret
		if updated != "" {
			currentCookies = updated
		}
		if result != nil {
			result.UpdatedCookies = currentCookies
			return result, nil
		}
		if isSessionExpiredRet(ret) {
			return nil, sessionExpiredError("订单详情接口", ret)
		}
		if !isTokenExpiredRet(ret) {
			return nil, fmt.Errorf("订单详情接口返回非成功: ret=%v", ret)
		}
		if attempt == 3 {
			break
		}
		if currentCookies == previousCookies {
			// refreshed、refreshErr 用于本次流程后续判断的refreshed、refreshErr
			refreshed, refreshErr := c.RefreshTokenContext(ctx, currentCookies)
			if refreshErr != nil {
				return nil, fmt.Errorf("订单详情 token 刷新失败: %w", refreshErr)
			}
			currentCookies = refreshed.UpdatedCookies
		}
		if // err 用于本次流程后续判断的err
		err := sleepCtx(ctx, MTopRetryGap); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("订单详情 token 重试失败: ret=%v", lastRet)
}

// fetchOrderDetailOnce 封装fetch订单DetailOnce业务协调。
func (c *ClientImpl) fetchOrderDetailOnce(ctx context.Context, cookiesStr, orderID string) (*OrderDetailResult, []string, string, []byte, error) {
	// hc 用于本次流程后续判断的hc
	hc := c.httpClient()
	// endpoint 用于本次流程后续判断的endpoint
	endpoint := c.OrderDetailURL
	if endpoint == "" {
		endpoint = OrderDetailAPI
	}
	// documentURL 用于本次流程后续判断的documentURL
	documentURL := "https://www.goofish.com/order-detail?orderId=" + url.QueryEscape(orderID) + "&role=seller"
	// signingCookies、requestCookies 用于本次流程后续判断的signingCookies、requestCookies
	signingCookies, requestCookies := mtopRequestCookies(ctx, cookiesStr, documentURL, endpoint)
	// t 用于本次流程后续判断的t
	t := strconv.FormatInt(time.Now().UnixMilli(), 10)
	// dataVal 用于本次流程后续判断的数据Val
	dataVal := `{"tid":"` + orderID + `"}`
	// sign 用于本次流程后续判断的sign
	sign := protocol.GenerateSign(t, protocol.SignToken(signingCookies), dataVal)
	// req、err 用于本次流程后续判断的req、err
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+buildOrderDetailQuery(t, sign), strings.NewReader("data="+url.QueryEscape(dataVal)))
	if err != nil {
		return nil, nil, cookiesStr, nil, err
	}
	setCommonHeaders(req, requestCookies)
	req.Header.Set("Referer", documentURL)
	// resp、err 用于本次流程后续判断的resp、err
	resp, err := hc.Do(req)
	if err != nil {
		return nil, nil, cookiesStr, nil, fmt.Errorf("订单详情请求失败: %w", err)
	}
	defer resp.Body.Close()
	// updated 用于本次流程后续判断的updated
	updated := absorbMTopResponseCookies(ctx, cookiesStr, resp)
	// raw、err 用于本次流程后续判断的raw、err
	raw, err := readMTopBody(resp)
	if err != nil {
		return nil, nil, updated, raw, err
	}
	// decoded 用于本次流程后续判断的decoded
	var decoded struct {
		Ret  []string       `json:"ret"`
		Data map[string]any `json:"data"`
	}
	if // err 用于本次流程后续判断的err
	err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, nil, updated, raw, fmt.Errorf("解析订单详情响应失败: %w (body=%s)", err, truncate(string(raw), 300))
	}
	if !hasMTopSuccess(decoded.Ret) {
		return nil, decoded.Ret, updated, raw, nil
	}
	// result 用于本次流程后续判断的结果
	result := &OrderDetailResult{Quantity: "1"}
	if // utArgs、ok 用于本次流程后续判断的utArgs、ok
	utArgs, ok := decoded.Data["utArgs"].(map[string]any); ok {
		result.OrderStatus = mtopString(utArgs["orderStatus"])
	}
	// components 用于本次流程后续判断的components
	components, _ := decoded.Data["components"].([]any)
	// component 表示当前遍历过程中的component
	for _, component := range components {
		// cm 用于本次流程后续判断的cm
		cm, _ := component.(map[string]any)
		if cm["render"] != "orderInfoVO" {
			continue
		}
		// componentData 用于本次流程后续判断的component数据
		componentData, _ := cm["data"].(map[string]any)
		if // itemInfo、ok 用于本次流程后续判断的商品Info、ok
		itemInfo, ok := componentData["itemInfo"].(map[string]any); ok {
			if // value 用于本次流程后续判断的值
			value := mtopString(itemInfo["buyAmount"]); value != "" {
				result.Quantity = value
			}
			result.SpecName = mtopString(itemInfo["specName"])
			result.SpecValue = mtopString(itemInfo["specValue"])
			// 真实返回把多规格放在合并字段 skuInfo（"规格名:规格值"）；specName/specValue
			// 缺省时从这里拆分，确保多规格订单的自动化规格匹配有事实依据。
			if result.SpecName == "" && result.SpecValue == "" {
				// skuText 是平台原始合并规格文本。
				if skuText := mtopString(itemInfo["skuInfo"]); skuText != "" {
					// name、value、ok 是拆分出的规格名称、规格值和分隔符存在标记。
					if name, value, ok := strings.Cut(skuText, ":"); ok {
						result.SpecName = strings.TrimSpace(name)
						result.SpecValue = strings.TrimSpace(value)
					} else {
						result.SpecValue = strings.TrimSpace(skuText)
					}
				}
			}
		}
		if // priceInfo、ok 用于本次流程后续判断的priceInfo、ok
		priceInfo, ok := componentData["priceInfo"].(map[string]any); ok {
			if // amount、ok 用于本次流程后续判断的amount、ok
			amount, ok := priceInfo["amount"].(map[string]any); ok {
				result.Amount = mtopString(amount["value"])
			}
		}
	}
	return result, decoded.Ret, updated, raw, nil
}

// buildOrderDetailQuery 封装build订单Detail查询业务协调。
func buildOrderDetailQuery(t, sign string) string {
	return "jsv=2.7.2&appKey=" + protocol.SignAppKey +
		"&t=" + t + "&sign=" + sign +
		"&v=1.0&type=originaljson&accountSite=xianyu&dataType=json&timeout=20000" +
		"&api=mtop.idle.web.trade.order.detail&sessionOption=AutoLoginOnly&valueType=string"
}
