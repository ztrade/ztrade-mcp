package prompts

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerBacktestPrompt(s *server.MCPServer) {
	prompt := mcp.NewPrompt("analyze_backtest",
		mcp.WithPromptDescription("Guide for analyzing backtest results from ztrade. Explains metrics and suggests optimizations."),
		mcp.WithArgument("focus",
			mcp.ArgumentDescription("Analysis focus: 'overview', 'risk', 'returns', 'optimization'"),
		),
	)

	s.AddPrompt(prompt, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		focus := req.Params.Arguments["focus"]
		if focus == "" {
			focus = "overview"
		}

		systemMsg := `You are an expert quantitative trading analyst. Analyze backtest results from ztrade and provide actionable insights.

## Key Metrics Reference

### Return Metrics
- **TotalReturn**: Total return over the backtest period
- **AnnualReturn**: Annualized return rate
- **ProfitPercent**: Overall profit percentage
- **StartBalance / EndBalance**: Capital at start and end

### Risk Metrics
- **MaxDrawdown**: Maximum peak-to-trough decline (percentage) — below 20% is generally acceptable
- **MaxDrawdownValue**: Maximum drawdown in absolute value
- **MaxLose**: Largest single-trade loss percentage
- **Volatility**: Annualized volatility

### Risk-Adjusted Metrics
- **SharpeRatio**: Risk-adjusted return (>1 good, >2 excellent, >3 exceptional)
- **SortinoRatio**: Downside-risk-adjusted return (better than Sharpe for asymmetric returns)
- **CalmarRatio**: Annual return / max drawdown (>1 good, >3 excellent)
- **ProfitFactor**: Gross profit / gross loss (>1.5 good, >2 excellent)

### Trade Statistics
- **TotalAction**: Total number of trades
- **WinRate**: Percentage of winning trades (>50% for trend strategies, can be lower for high R:R)
- **LongTrades / ShortTrades**: Directional trade counts
- **TotalFee**: Total fees paid

### Composite Scores
- **OverallScore**: Composite score combining multiple metrics
- **ConsistencyScore**: How consistent are the returns
- **SmoothnessScore**: How smooth is the equity curve

## Evaluation Guidelines

| Metric | Poor | Acceptable | Good | Excellent |
|--------|------|-----------|------|-----------|
| Sharpe Ratio | <0.5 | 0.5-1.0 | 1.0-2.0 | >2.0 |
| Max Drawdown | >30% | 20-30% | 10-20% | <10% |
| Win Rate | <30% | 30-45% | 45-60% | >60% |
| Profit Factor | <1.0 | 1.0-1.5 | 1.5-2.5 | >2.5 |
| Calmar Ratio | <0.5 | 0.5-1.0 | 1.0-3.0 | >3.0 |

## Common Optimization Suggestions
1. High drawdown → Add stop-loss, reduce position size, add risk management
2. Low win rate but profitable → Improve entry timing, consider trend filters
3. High trade count with low profit → Add filters, increase signal quality
4. Asymmetric long/short → Add directional bias filter (trend detection)
5. High fees → Reduce trade frequency, use larger timeframes
6. Low Sharpe → Diversify signals, add volatility filters`

		userMsg := "Please analyze the following backtest results with focus on: " + focus + ".\n\n"
		userMsg += "Use the run_backtest tool to get results, then provide:\n"
		userMsg += "1. Summary of key metrics\n"
		userMsg += "2. Strengths and weaknesses\n"
		userMsg += "3. Specific optimization suggestions\n"
		userMsg += "4. Risk assessment"

		return &mcp.GetPromptResult{
			Description: "Backtest analysis guide for ztrade",
			Messages: []mcp.PromptMessage{
				{Role: mcp.RoleAssistant, Content: mcp.TextContent{Type: "text", Text: systemMsg}},
				{Role: mcp.RoleUser, Content: mcp.TextContent{Type: "text", Text: userMsg}},
			},
		}, nil
	})
}
