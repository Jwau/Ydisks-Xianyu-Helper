// orderdetail-dump 调用一次订单详情接口并输出平台原始返回，用于诊断自动发货
// 规格解析问题。工具读取与应用相同的 DATABASE_URL / XIANYU_DATA_KEY 环境变量，
// 建议通过 docker exec 在应用容器内运行，直接复用容器的数据库连接与数据密钥。
//
// 用法（应用容器内）：
//
//	/app/orderdetail-dump -cookie-id 1806822441 -order-id 3316818937027012082
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"xianyu-go/internal/db"
	"xianyu-go/internal/logsafe"
	"xianyu-go/internal/xianyu/mtop"
)

// main 是订单详情诊断 CLI 的进程入口；原始响应只输出到标准输出，不写入日志文件。
func main() {
	// cookieID 是账号 Cookie 主键（cookies 表 id 字段）。
	cookieID := flag.String("cookie-id", "", "账号 Cookie ID（cookies 表 id）")
	// orderID 是待诊断的闲鱼订单号。
	orderID := flag.String("order-id", "", "订单号")
	flag.Parse()
	if *cookieID == "" || *orderID == "" {
		fmt.Println("用法: orderdetail-dump -cookie-id <账号ID> -order-id <订单号>")
		os.Exit(1)
	}
	// dbURL 是数据库连接地址；必须与应用容器使用同一 DATABASE_URL。
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Println("缺少 DATABASE_URL 环境变量")
		os.Exit(1)
	}
	// ctx 限制本次诊断调用的最长耗时。
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// database、dialect、err 是数据库连接、方言和打开错误。
	database, dialect, err := db.Open(ctx, dbURL)
	if err != nil {
		fmt.Printf("打开数据库失败: %s\n", logsafe.Error(err))
		os.Exit(1)
	}
	defer database.Close()
	// store 提供凭证读取能力；解密依赖容器内的 XIANYU_DATA_KEY。
	store := db.NewStore(database, dialect)
	// cookieStr 是解密后的账号 Cookie，仅在进程内短暂使用，不打印。
	cookieStr, err := store.Cookies.GetValue(ctx, *cookieID)
	if err != nil {
		fmt.Printf("读取账号 Cookie 失败: %s\n", logsafe.Error(err))
		os.Exit(1)
	}
	// client 是 MTOP 客户端；诊断调用与业务链路使用同一签名与重试语义。
	client := mtop.NewClient()
	// result、raw、err 分别是解析结果、原始响应体和调用错误。
	result, raw, err := client.FetchOrderDetailRaw(ctx, cookieStr, *orderID)
	if err != nil {
		fmt.Printf("调用订单详情失败: %s\n", logsafe.Error(err))
		os.Exit(1)
	}
	fmt.Println("== 解析结果 ==")
	// 逐字段输出解析结果；UpdatedCookies 是完整凭证，不得打印到标准输出。
	fmt.Printf("Quantity=%q SpecName=%q SpecValue=%q OrderStatus=%q Amount=%q\n",
		result.Quantity, result.SpecName, result.SpecValue, result.OrderStatus, result.Amount)
	fmt.Println("== 原始响应 ==")
	// pretty 是重新缩进后的响应 JSON，便于人工核对字段位置；err 是 JSON 解析错误。
	var pretty any
	// err 保存原始响应 JSON 的解析错误。
	if err := json.Unmarshal(raw, &pretty); err != nil {
		fmt.Println("响应不是合法 JSON:")
		fmt.Println(string(raw))
		os.Exit(1)
	}
	// encoded 是缩进序列化结果；序列化失败时回退输出原始文本。
	encoded, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		fmt.Println(string(raw))
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}
