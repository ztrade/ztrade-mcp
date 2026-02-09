package resources

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const strategyDocContent = `# ztrade 策略开发指南

## 策略结构

每个策略是一个 Go struct，需要实现以下方法：

### 必须实现的方法

#### 构造函数: NewXxx() *Xxx
` + "```go" + `
func NewMyStrategy() *MyStrategy {
	return new(MyStrategy)
}
` + "```" + `

#### Param() — 定义参数
` + "```go" + `
func (s *MyStrategy) Param() (paramInfo []Param) {
	paramInfo = []Param{
		StringParam("key", "标签", "描述", "默认值", &s.field),
		IntParam("key", "标签", "描述", 10, &s.intField),
		FloatParam("key", "标签", "描述", 1.0, &s.floatField),
	}
	return
}
` + "```" + `

#### Init(engine Engine, params ParamData) error — 初始化
` + "```go" + `
func (s *MyStrategy) Init(engine Engine, params ParamData) (err error) {
	s.engine = engine
	// 合并K线
	engine.Merge("1m", "15m", s.OnCandle15m)
	// 添加指标
	s.ema = engine.AddIndicator("EMA", 9, 26)
	return
}
` + "```" + `

#### OnCandle(candle *Candle) — 1m K线回调
- 回测中: candle.ID 是数据库中的 ID
- 实盘中: candle.ID == -1 表示历史数据

#### OnPosition(pos, price float64) — 仓位变化回调
- pos > 0: 多仓
- pos < 0: 空仓
- pos == 0: 空仓位

### 可选方法

- OnTrade(trade *Trade) — 自己的订单成交回调
- OnTradeMarket(trade *Trade) — 市场成交回调
- OnDepth(depth *Depth) — 深度数据回调

## 两种运行方式

### 1. 插件模式 (.so) — 推荐
` + "```bash" + `
ztrade build --script my_strategy.go --output my_strategy.so
ztrade backtest --script my_strategy.so --exchange binance --symbol BTCUSDT \
  --start "2024-01-01 08:00:00" --end "2024-06-01 08:00:00"
` + "```" + `

### 2. 源码模式 (.go) — 需要 ixgo 构建
` + "```bash" + `
ztrade backtest --script my_strategy.go --exchange binance --symbol BTCUSDT \
  --start "2024-01-01 08:00:00" --end "2024-06-01 08:00:00"
` + "```" + `

## 重要说明

1. **数据基础**: 回测和实盘的数据订阅固定以 1m 为基础
2. **合成大周期**: 使用 engine.Merge("1m", "15m", callback) 来合成更大的K线周期
3. **参数传递**: 通过 --param 传递 JSON 参数，引擎会自动解析到 Param() 中绑定的变量
`

func registerStrategyDoc(s *server.MCPServer) {
	resource := mcp.NewResource(
		"ztrade://doc/strategy",
		"Strategy Development Guide",
		mcp.WithResourceDescription("ztrade strategy development reference documentation, including strategy structure, Engine API, indicators, and best practices."),
		mcp.WithMIMEType("text/markdown"),
	)

	s.AddResource(resource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "ztrade://doc/strategy",
				MIMEType: "text/markdown",
				Text:     strategyDocContent,
			},
		}, nil
	})
}
