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

## ztrade Strategy Rules

1. **Data Foundation**: All data is based on 1m candles. Use engine.Merge("1m", "TARGET", callback) for larger timeframes.
2. **Strategy Structure**: Must implement NewXxx() constructor, Param(), Init(), OnCandle(), OnPosition().
3. **Imports**: Use ` + "`" + `. \"github.com/ztrade/trademodel\"` + "`" + ` (dot import for Candle, Trade, Engine, etc.)
4. **Package**: Strategy files use ` + "`" + `package strategy` + "`" + `
5. **Parameters**: Define with StringParam/IntParam/FloatParam in Param(), auto-parsed from --param JSON.
6. **Trading**: Use engine.OpenLong/CloseLong/OpenShort/CloseShort for orders.
7. **Position Tracking**: Use OnPosition callback, not manual tracking.
8. **Historical Data Detection**: In live trading, candle.ID == -1 means historical data; skip trading on historical candles.

## Available Indicators
- EMA(fast, slow) — Exponential MA cross
- SMA(period) — Simple MA
- MACD(fast, slow, signal) — MACD
- BOLL(period, multiplier) — Bollinger Bands
- RSI(period) — Relative Strength Index
- STOCHRSI(stochLen, rsiLen, kSmooth, dSmooth) — Stochastic RSI

## Reading Indicator Values
- ind.Result() — current value (or fast line for dual-line indicators)
- ind.Indicator() — map with detailed values: "fast", "slow", "crossUp", "crossDown", "top", "bottom"

## Available Resources
Read "ztrade://doc/strategy" and "ztrade://doc/engine" for full API reference.

## Available Tools
- create_strategy: Generate strategy skeleton code
- build_strategy: Compile .go to .so plugin
- run_backtest: Run backtest with results
- query_kline: Query historical data for analysis`

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
