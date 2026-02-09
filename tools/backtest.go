package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ztrade/ztrade/pkg/ctl"
	"github.com/ztrade/ztrade/pkg/process/dbstore"
	"github.com/ztrade/ztrade/pkg/report"
)

func registerRunBacktest(s *server.MCPServer, db *dbstore.DBStore) {
	tool := mcp.NewTool("run_backtest",
		mcp.WithDescription("Run a backtest with a strategy script on historical data. Returns structured results including profit, win rate, sharpe ratio, max drawdown, etc."),
		mcp.WithString("script", mcp.Required(), mcp.Description("Strategy file path (.go or .so)")),
		mcp.WithString("exchange", mcp.Required(), mcp.Description("Exchange name (e.g., binance)")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Trading pair (e.g., BTCUSDT)")),
		mcp.WithString("start", mcp.Required(), mcp.Description("Backtest start time in format '2006-01-02 15:04:05'")),
		mcp.WithString("end", mcp.Required(), mcp.Description("Backtest end time in format '2006-01-02 15:04:05'")),
		mcp.WithNumber("balance", mcp.Description("Initial balance. Default: 100000")),
		mcp.WithNumber("fee", mcp.Description("Trading fee rate. Default: 0.0001")),
		mcp.WithNumber("lever", mcp.Description("Leverage multiplier. Default: 1")),
		mcp.WithString("param", mcp.Description("Strategy parameters as JSON string")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if db == nil {
			return mcp.NewToolResultError("database not initialized"), nil
		}

		script := req.GetString("script", "")
		exchange := req.GetString("exchange", "")
		symbol := req.GetString("symbol", "")
		startStr := req.GetString("start", "")
		endStr := req.GetString("end", "")
		balanceF := req.GetFloat("balance", 0)
		feeF := req.GetFloat("fee", 0)
		leverF := req.GetFloat("lever", 0)
		param := req.GetString("param", "")

		start, err := time.Parse("2006-01-02 15:04:05", startStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid start time: %s", err.Error())), nil
		}
		end, err := time.Parse("2006-01-02 15:04:05", endStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid end time: %s", err.Error())), nil
		}

		if balanceF <= 0 {
			balanceF = 100000
		}
		if feeF <= 0 {
			feeF = 0.0001
		}
		if leverF <= 0 {
			leverF = 1
		}

		bt, err := ctl.NewBacktest(db, exchange, symbol, param, start, end)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create backtest: %s", err.Error())), nil
		}

		bt.SetScript(script)
		bt.SetBalanceInit(balanceF, feeF)
		bt.SetLever(leverF)

		rpt := report.NewReportSimple()
		rpt.SetTimeRange(start, end)
		rpt.SetFee(feeF)
		rpt.SetLever(leverF)
		bt.SetReporter(rpt)

		err = bt.Run()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("backtest failed: %s", err.Error())), nil
		}

		rawResult, err := bt.Result()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get result: %s", err.Error())), nil
		}

		resultData, ok := rawResult.(report.ReportResult)
		if !ok {
			return mcp.NewToolResultError("unexpected result type"), nil
		}

		result := map[string]interface{}{
			"totalActions":     resultData.TotalAction,
			"winRate":          resultData.WinRate,
			"totalProfit":      resultData.TotalProfit,
			"profitPercent":    resultData.ProfitPercent,
			"maxDrawdown":      resultData.MaxDrawdown,
			"maxDrawdownValue": resultData.MaxDrawdownValue,
			"maxLose":          resultData.MaxLose,
			"totalFee":         resultData.TotalFee,
			"startBalance":     resultData.StartBalance,
			"endBalance":       resultData.EndBalance,
			"totalReturn":      resultData.TotalReturn,
			"annualReturn":     resultData.AnnualReturn,
			"sharpeRatio":      resultData.SharpeRatio,
			"sortinoRatio":     resultData.SortinoRatio,
			"volatility":       resultData.Volatility,
			"profitFactor":     resultData.ProfitFactor,
			"calmarRatio":      resultData.CalmarRatio,
			"overallScore":     resultData.OverallScore,
			"longTrades":       resultData.LongTrades,
			"shortTrades":      resultData.ShortTrades,
		}

		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
