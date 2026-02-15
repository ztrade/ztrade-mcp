package prompts

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerStrategyPrompt(s *server.MCPServer) {
	prompt := mcp.NewPrompt("create_strategy",
		mcp.WithPromptDescription("Guide for developing a ztrade trading strategy. Provides strategy structure, available APIs, indicators, and best practices."),
		mcp.WithArgument("strategyType",
			mcp.ArgumentDescription("Type of strategy to create (e.g., 'trend-following', 'mean-reversion', 'breakout', 'oscillator')"),
		),
		mcp.WithArgument("indicators",
			mcp.ArgumentDescription("Comma-separated indicators to use (e.g., 'EMA,MACD,BOLL')"),
		),
		mcp.WithArgument("timeframe",
			mcp.ArgumentDescription("Primary trading timeframe (e.g., '15m', '1h', '4h')"),
		),
	)

	s.AddPrompt(prompt, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		strategyType := req.Params.Arguments["strategyType"]
		indicators := req.Params.Arguments["indicators"]
		timeframe := req.Params.Arguments["timeframe"]

		if strategyType == "" {
			strategyType = "trend-following"
		}
		if timeframe == "" {
			timeframe = "1h"
		}

		systemMsg := `You are an expert quantitative trading strategy developer for the ztrade framework.

# ztrade 策略开发规范

## 策略结构
1. 策略为 Go struct，需实现：
	- NewXxx() *Xxx
	- Param() []Param
	- Init(engine Engine, params ParamData) error
	- OnCandle(candle *Candle)
	- OnPosition(pos, price float64)
	- 可选：OnCandle15m/OnTrade/OnTradeMarket/OnDepth
2. 包名统一为 "package strategy"
3. 必须使用 '. "github.com/ztrade/trademodel"' 点导入

## Engine API（核心接口）
- OpenLong/CloseLong/OpenShort/CloseShort/StopLong/StopShort/CancelOrder/CancelAllOrder/DoOrder
- Position()/Balance()/SetBalance()
- Merge(src, dst, fn) 合成大周期K线
- AddIndicator(name, ...params) 添加指标
- Log()/SendNotify()/UpdateStatus()/Watch()

## 内置指标
| 指标      | 参数                | 示例                              | 说明         |
|-----------|---------------------|-----------------------------------|--------------|
| EMA       | 1或2: 单线/交叉     | AddIndicator("EMA", 9, 26)        | 指数均线      |
| SMA       | 1或2: 单线/交叉     | AddIndicator("SMA", 20)           | 简单均线      |
| SSMA      | 1或2: 单线/交叉     | AddIndicator("SSMA", 9, 26)       | 平滑均线      |
| MACD      | 3:快/慢/DEA         | AddIndicator("MACD", 12,26,9)     | MACD         |
| SMAMACD   | 3:快/慢/DEA         | AddIndicator("SMAMACD",12,26,9)   | SMA版MACD     |
| BOLL      | 长度、倍数          | AddIndicator("BOLL", 20, 2)       | 布林带        |
| RSI       | 1或2: 单线/交叉     | AddIndicator("RSI", 14)           | 相对强弱      |
| STOCHRSI  | 4:窗口/平滑         | AddIndicator("STOCHRSI",14,14,3,3)| 随机RSI       |

### 指标返回值
- ind.Result() 当前值（双线时为快线）
- ind.Indicator() map[string]float64 详细值（fast/slow/crossUp/crossDown/top/bottom等）

## 重要说明
1. 数据基础：所有数据以1m为基础，合成大周期用 engine.Merge("1m", "15m", cb)
2. 参数传递：通过 --param 传递JSON，自动绑定 Param() 字段
3. 实盘 candle.ID == -1 表示历史数据，回测为数据库ID
4. 下单必须用 engine.* 系列方法，仓位跟踪用 OnPosition

## 参考文档
请查阅 "ztrade://doc/strategy" 和 "ztrade://doc/engine" 获取完整API和开发说明。

## 可用工具
- create_strategy: 生成策略代码模板
- build_strategy: 编译 .go 为 .so 插件
- run_backtest: 回测（大于30天自动异步）
- run_backtest_managed: 托管策略回测并自动记录（大于30天自动异步）
- download_kline: 下载K线（大于30天或auto自动异步）
- query_kline: 查询历史K线
- get_task_status: 查询异步任务进度
- get_task_result: 获取异步任务结果
- list_tasks: 列出所有异步任务

## 异步任务说明
当回测/下载时间范围超过30天时，工具会返回taskId。用 get_task_status 轮询进度，get_task_result 获取最终结果。`

		userMsg := "Please help me create a " + strategyType + " strategy"
		if indicators != "" {
			userMsg += " using " + indicators + " indicators"
		}
		userMsg += " for the " + timeframe + " timeframe."
		userMsg += "\n\nPlease:\n1. First read the strategy documentation resources\n2. Generate the strategy code using create_strategy tool\n3. Explain the strategy logic clearly"

		return &mcp.GetPromptResult{
			Description: "Strategy development guide for ztrade",
			Messages: []mcp.PromptMessage{
				{Role: mcp.RoleAssistant, Content: mcp.TextContent{Type: "text", Text: systemMsg}},
				{Role: mcp.RoleUser, Content: mcp.TextContent{Type: "text", Text: userMsg}},
			},
		}, nil
	})
}
