package resources

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const engineDocContent = `# ztrade Engine API 参考

## Engine 接口

Engine 是策略与引擎交互的核心接口。在 Init() 中通过参数获取，存储到策略 struct 中供后续使用。

### 交易操作

| 方法 | 说明 | 返回 |
|------|------|------|
| OpenLong(price, amount float64) | 开多仓 | order ID |
| CloseLong(price, amount float64) | 平多仓 | order ID |
| OpenShort(price, amount float64) | 开空仓 | order ID |
| CloseShort(price, amount float64) | 平空仓 | order ID |
| StopLong(price, amount float64) | 多仓止损 | order ID |
| StopShort(price, amount float64) | 空仓止损 | order ID |
| CancelOrder(id string) | 取消订单 | - |
| CancelAllOrder() | 取消全部订单 | - |
| DoOrder(typ TradeType, price, amount float64) | 通用下单 | order ID |

### 状态查询

| 方法 | 说明 |
|------|------|
| Position() (pos, price float64) | 获取当前仓位和开仓均价 |
| Balance() float64 | 获取当前余额 |

### K线合并

| 方法 | 说明 |
|------|------|
| Merge(src, dst string, fn CandleFn) | 合并K线周期 |

- src: 源周期 (固定为 "1m")
- dst: 目标周期 ("5m", "15m", "30m", "1h", "4h", "1d" 等)
- fn: 回调函数 func(candle *Candle)

### 指标管理

| 方法 | 说明 |
|------|------|
| AddIndicator(name string, params ...int) CommonIndicator | 添加技术指标 |

### 其他

| 方法 | 说明 |
|------|------|
| Log(v ...interface{}) | 日志输出 |
| SendNotify(title, content, contentType string) | 发送通知 |
| SetBalance(balance float64) | 设置余额（仅回测有效） |
| UpdateStatus(status int, msg string) | 更新策略状态 |
| Watch(watchType string) | 添加订阅事件 |

## 内置指标

| 指标 | 参数 | 示例 | 说明 |
|------|------|------|------|
| EMA | 1个: 单线; 2个: 交叉 | AddIndicator("EMA", 9, 26) | 指数移动平均 |
| SMA | 1个: 单线; 2个: 交叉 | AddIndicator("SMA", 20) | 简单移动平均 |
| SSMA | 1个: 单线; 2个: 交叉 | AddIndicator("SSMA", 9, 26) | 平滑移动平均 |
| MACD | 快线、慢线、DEA | AddIndicator("MACD", 12, 26, 9) | MACD |
| SMAMACD | 快线、慢线、DEA | AddIndicator("SMAMACD", 12, 26, 9) | SMA计算的MACD |
| BOLL | 长度、倍数 | AddIndicator("BOLL", 20, 2) | 布林带 |
| RSI | 1个: 单线; 2个: 交叉 | AddIndicator("RSI", 14) | 相对强弱指数 |
| STOCHRSI | STOCH窗口、RSI窗口、K平滑、D平滑 | AddIndicator("STOCHRSI", 14, 14, 3, 3) | 随机RSI |

### 指标返回值

CommonIndicator 接口:
- Result() float64 — 当前值(双线时返回快线值)
- Indicator() map[string]float64 — 详细值

双线指标 (EMA/SMA/SSMA/RSI 双参数):
- result, fast, slow — 线值
- crossUp (1=金叉), crossDown (1=死叉)

BOLL指标:
- result — 中轨
- top — 上轨
- bottom — 下轨
`

func registerEngineDoc(s *server.MCPServer) {
	resource := mcp.NewResource(
		"ztrade://doc/engine",
		"Engine API Reference",
		mcp.WithResourceDescription("ztrade Engine interface API reference, including trading operations, indicators, and K-line merging."),
		mcp.WithMIMEType("text/markdown"),
	)

	s.AddResource(resource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "ztrade://doc/engine",
				MIMEType: "text/markdown",
				Text:     engineDocContent,
			},
		}, nil
	})
}
