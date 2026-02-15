package resources

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const strategyDocContent = `# ztrade 策略开发指南

每个策略是一个 Go struct，需实现：
- NewXxx() *Xxx
- Param() []Param
- Init(engine Engine, params ParamData) error
- OnCandle(candle *Candle)
- OnPosition(pos, price float64)
- 可选：OnCandle15m/OnTrade/OnTradeMarket/OnDepth

## 参数定义（Param）
ztrade 策略参数通过 Param() 方法定义，支持三种类型：

- StringParam(key, label, desc, default, &field)
- IntParam(key, label, desc, default, &field)
- FloatParam(key, label, desc, default, &field)

示例：


func (s *MyStrategy) Param() (paramInfo []Param) {
    paramInfo = []Param{
        StringParam("mode", "模式", "交易模式", "trend", &s.Mode),
        IntParam("fast", "快线", "EMA快线周期", 9, &s.Fast),
        FloatParam("threshold", "阈值", "开仓阈值", 0.01, &s.Threshold),
    }
    return
}

所有参数会自动从 --param 传入的 JSON 解析并绑定到对应字段。

type MyStrategy struct {
	engine Engine
	// 参数字段
}

func NewMyStrategy() *MyStrategy {
	return new(MyStrategy)
}

func (s *MyStrategy) Param() (paramInfo []Param) {
	paramInfo = []Param{
		// StringParam("key", "标签", "描述", "默认值", &s.field),
		// IntParam("key", "标签", "描述", 10, &s.intField),
		// FloatParam("key", "标签", "描述", 1.0, &s.floatField),
	}
	return
}

func (s *MyStrategy) Init(engine Engine, params ParamData) (err error) {
	s.engine = engine
	// 合并K线
	engine.Merge("1m", "15m", s.OnCandle15m)
	// 添加指标
	// s.ema = engine.AddIndicator("EMA", 9, 26)
	return
}

func (s *MyStrategy) OnCandle(candle *Candle) {
	// 1m K线回调
}

func (s *MyStrategy) OnCandle15m(candle *Candle) {
	// 15m K线回调
}

func (s *MyStrategy) OnPosition(pos, price float64) {
	// 仓位变化回调
}

// 可选: 成交/深度等回调
// func (s *MyStrategy) OnTrade(trade *Trade) {}
// func (s *MyStrategy) OnTradeMarket(trade *Trade) {}
// func (s *MyStrategy) OnDepth(depth *Depth) {}


## Engine API
详见 "ztrade://doc/engine"，支持下单、合成K线、添加指标、日志、通知等。

## 指标用法
见 Engine API 文档，支持 EMA/SMA/SSMA/MACD/BOLL/RSI/STOCHRSI 等。

## 运行方式
ztrade build --script my_strategy.go --output my_strategy.so
ztrade backtest --script my_strategy.so --exchange binance --symbol BTCUSDT \
  --start "2024-01-01 08:00:00" --end "2024-06-01 08:00:00"
 
ztrade backtest --script my_strategy.go --exchange binance --symbol BTCUSDT \
  --start "2024-01-01 08:00:00" --end "2024-06-01 08:00:00"
 

## 重要说明
1. 数据基础：回测和实盘数据订阅均以1m为基础
2. 合成大周期：用 engine.Merge("1m", "15m", cb)
3. 参数传递：--param 传JSON，自动绑定 Param() 字段
4. 实盘 candle.ID == -1 表示历史数据，回测为数据库ID
5. 下单必须用 engine.* 系列方法，仓位跟踪用 OnPosition
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
