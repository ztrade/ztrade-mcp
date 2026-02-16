package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	log "github.com/sirupsen/logrus"
	"github.com/ztrade/ztrade-mcp/store"
	"github.com/ztrade/ztrade/pkg/ctl"
	"github.com/ztrade/ztrade/pkg/process/dbstore"
	"github.com/ztrade/ztrade/pkg/report"
)

func registerRunBacktestManaged(s *server.MCPServer, db *dbstore.DBStore, st *store.Store, tm *TaskManager) {
	tool := mcp.NewTool("run_backtest_managed",
		mcp.WithDescription("Run a backtest using a managed strategy from the database. The strategy is extracted from DB, backtested, and results are automatically saved for performance tracking. When the time range exceeds 30 days the task runs asynchronously — a task ID is returned immediately and you can poll progress with get_task_status / get_task_result."),
		mcp.WithNumber("strategyId", mcp.Required(), mcp.Description("Strategy ID in the database")),
		mcp.WithString("exchange", mcp.Required(), mcp.Description("Exchange name (e.g., binance)")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Trading pair (e.g., BTCUSDT)")),
		mcp.WithString("start", mcp.Required(), mcp.Description("Backtest start time in format '2006-01-02 15:04:05'")),
		mcp.WithString("end", mcp.Required(), mcp.Description("Backtest end time in format '2006-01-02 15:04:05'")),
		mcp.WithNumber("balance", mcp.Description("Initial balance. Default: 100000")),
		mcp.WithNumber("fee", mcp.Description("Trading fee rate. Default: 0.0005")),
		mcp.WithNumber("lever", mcp.Description("Leverage multiplier. Default: 1")),
		mcp.WithString("param", mcp.Description("Strategy parameters as JSON string")),
		mcp.WithNumber("version", mcp.Description("Strategy version to use. Default: latest version.")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if db == nil {
			return mcp.NewToolResultError("database not initialized"), nil
		}
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		strategyID := int64(req.GetFloat("strategyId", 0))
		exchangeName := req.GetString("exchange", "")
		symbol := req.GetString("symbol", "")
		startStr := req.GetString("start", "")
		endStr := req.GetString("end", "")
		balanceF := req.GetFloat("balance", 0)
		feeF := req.GetFloat("fee", 0)
		leverF := req.GetFloat("lever", 0)
		param := req.GetString("param", "")
		versionF := req.GetFloat("version", 0)

		// Get strategy from DB
		script, err := st.GetScript(strategyID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get script: %s", err.Error())), nil
		}

		// If a specific version is requested, get that version's content
		scriptContent := script.Content
		scriptVersion := script.Version
		if versionF > 0 {
			ver, err := st.GetVersion(strategyID, int(versionF))
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get version: %s", err.Error())), nil
			}
			scriptContent = ver.Content
			scriptVersion = ver.Version
		}

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
			feeF = 0.0005
		}
		if leverF <= 0 {
			leverF = 1
		}

		// Write script to temp file for backtesting
		tmpFile := fmt.Sprintf("/tmp/ztrade_script_%d_v%d.go", strategyID, scriptVersion)
		if err := writeFile(tmpFile, scriptContent); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to write temp script: %s", err.Error())), nil
		}

		// --- 自动编译为 so ---
		soFile := fmt.Sprintf("/tmp/ztrade_script_%d_v%d.so", strategyID, scriptVersion)
		builder := ctl.NewBuilder(tmpFile, soFile)
		if err := builder.Build(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build so: %s", err.Error())), nil
		}

		// runManagedBacktest is the core logic shared by sync and async paths
		runManagedBacktest := func() (map[string]interface{}, error) {
			bt, err := ctl.NewBacktest(db, exchangeName, symbol, param, start, end)
			if err != nil {
				return nil, fmt.Errorf("failed to create backtest: %s", err.Error())
			}

			// In default (non-ixgo) builds, GoEngine only supports plugin files (.so/.dll/.dylib).
			// Use the compiled plugin instead of the temporary .go source file.
			bt.SetScript(soFile)
			bt.SetBalanceInit(balanceF, feeF)
			bt.SetLever(leverF)

			rpt := report.NewReportSimple()
			rpt.SetTimeRange(start, end)
			rpt.SetFee(feeF)
			rpt.SetLever(leverF)
			bt.SetReporter(rpt)

			if err := bt.Run(); err != nil {
				return nil, fmt.Errorf("backtest failed: %s", err.Error())
			}

			rawResult, err := bt.Result()
			if err != nil {
				return nil, fmt.Errorf("failed to get result: %s", err.Error())
			}

			resultData, ok := rawResult.(report.ReportResult)
			if !ok {
				return nil, fmt.Errorf("unexpected result type")
			}

			// Save backtest record
			record := &store.BacktestRecord{
				ScriptID: strategyID, ScriptVersion: scriptVersion,
				Exchange: exchangeName, Symbol: symbol,
				StartTime: start, EndTime: end,
				InitBalance: balanceF, Fee: feeF, Lever: leverF, Param: param,
				TotalActions: resultData.TotalAction, WinRate: resultData.WinRate,
				TotalProfit: resultData.TotalProfit, ProfitPercent: resultData.ProfitPercent,
				MaxDrawdown: resultData.MaxDrawdown, MaxDrawdownValue: resultData.MaxDrawdownValue,
				MaxLose: resultData.MaxLose, TotalFee: resultData.TotalFee,
				StartBalance: resultData.StartBalance, EndBalance: resultData.EndBalance,
				TotalReturn: resultData.TotalReturn, AnnualReturn: resultData.AnnualReturn,
				SharpeRatio: resultData.SharpeRatio, SortinoRatio: resultData.SortinoRatio,
				Volatility: resultData.Volatility, ProfitFactor: resultData.ProfitFactor,
				CalmarRatio: resultData.CalmarRatio, OverallScore: resultData.OverallScore,
				LongTrades: resultData.LongTrades, ShortTrades: resultData.ShortTrades,
			}
			if saveErr := st.SaveBacktestRecord(record); saveErr != nil {
				log.Warnf("backtest completed but failed to save record: %s", saveErr.Error())
			}

			result := map[string]interface{}{
				"recordId": record.ID, "strategyId": strategyID,
				"strategyName": script.Name, "strategyVersion": scriptVersion,
				"exchange": exchangeName, "symbol": symbol,
				"totalActions": resultData.TotalAction, "winRate": resultData.WinRate,
				"totalProfit": resultData.TotalProfit, "profitPercent": resultData.ProfitPercent,
				"maxDrawdown": resultData.MaxDrawdown, "maxDrawdownValue": resultData.MaxDrawdownValue,
				"totalReturn": resultData.TotalReturn, "annualReturn": resultData.AnnualReturn,
				"sharpeRatio": resultData.SharpeRatio, "sortinoRatio": resultData.SortinoRatio,
				"volatility": resultData.Volatility, "profitFactor": resultData.ProfitFactor,
				"calmarRatio": resultData.CalmarRatio, "overallScore": resultData.OverallScore,
				"longTrades": resultData.LongTrades, "shortTrades": resultData.ShortTrades,
			}
			return result, nil
		}

		// If time range > threshold, run asynchronously
		if ShouldRunAsync(start, end) {
			taskID := tm.CreateTask("backtest_managed", map[string]string{
				"strategyId": fmt.Sprintf("%d", strategyID),
				"exchange":   exchangeName,
				"symbol":     symbol,
				"start":      startStr,
				"end":        endStr,
			})

			go func() {
				tm.StartTask(taskID)
				doneCh := tm.ProgressEstimator(taskID, "backtest_managed", start, end)

				result, err := runManagedBacktest()
				close(doneCh)

				if err != nil {
					log.Errorf("async managed backtest task %s failed: %s", taskID, err.Error())
					tm.FailTask(taskID, err.Error())
					return
				}

				data, _ := json.MarshalIndent(result, "", "  ")
				tm.CompleteTask(taskID, string(data))
				log.Infof("async managed backtest task %s completed", taskID)
			}()

			asyncResult := map[string]interface{}{
				"async":   true,
				"taskId":  taskID,
				"message": fmt.Sprintf("Backtest time range exceeds %d days, running asynchronously. Use get_task_status with taskId '%s' to check progress, or get_task_result to retrieve the final result.", AsyncThresholdDays, taskID),
			}
			data, _ := json.MarshalIndent(asyncResult, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		}

		// Synchronous execution for short time ranges
		result, err := runManagedBacktest()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerListBacktestRecords(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("list_backtest_records",
		mcp.WithDescription("List backtest history for a strategy. Returns all backtest runs with performance metrics, ordered by most recent first."),
		mcp.WithNumber("strategyId", mcp.Required(), mcp.Description("Strategy ID")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of records to return. Default: 20")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		strategyID := int64(req.GetFloat("strategyId", 0))
		limit := int(req.GetFloat("limit", 0))
		if limit <= 0 {
			limit = 20
		}

		records, err := st.ListBacktestRecords(strategyID, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list records: %s", err.Error())), nil
		}

		type recordSummary struct {
			ID            int64   `json:"id"`
			ScriptVersion int     `json:"scriptVersion"`
			Exchange      string  `json:"exchange"`
			Symbol        string  `json:"symbol"`
			StartTime     string  `json:"startTime"`
			EndTime       string  `json:"endTime"`
			Param         string  `json:"param,omitempty"`
			WinRate       float64 `json:"winRate"`
			TotalReturn   float64 `json:"totalReturn"`
			SharpeRatio   float64 `json:"sharpeRatio"`
			MaxDrawdown   float64 `json:"maxDrawdown"`
			OverallScore  float64 `json:"overallScore"`
			CreatedAt     string  `json:"createdAt"`
		}

		var summaries []recordSummary
		for _, r := range records {
			summaries = append(summaries, recordSummary{
				ID:            r.ID,
				ScriptVersion: r.ScriptVersion,
				Exchange:      r.Exchange,
				Symbol:        r.Symbol,
				StartTime:     r.StartTime.Format("2006-01-02 15:04:05"),
				EndTime:       r.EndTime.Format("2006-01-02 15:04:05"),
				Param:         r.Param,
				WinRate:       r.WinRate,
				TotalReturn:   r.TotalReturn,
				SharpeRatio:   r.SharpeRatio,
				MaxDrawdown:   r.MaxDrawdown,
				OverallScore:  r.OverallScore,
				CreatedAt:     r.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}

		result := map[string]interface{}{
			"strategyId": strategyID,
			"total":      len(summaries),
			"records":    summaries,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerStrategyPerformance(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("strategy_performance",
		mcp.WithDescription("Get aggregated performance summary for a strategy across all backtests. Includes best/worst runs, average score, and key metrics ranges."),
		mcp.WithNumber("strategyId", mcp.Required(), mcp.Description("Strategy ID")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		strategyID := int64(req.GetFloat("strategyId", 0))

		// Get strategy info
		script, err := st.GetScript(strategyID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get script: %s", err.Error())), nil
		}

		summary, err := st.GetBacktestSummary(strategyID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get performance summary: %s", err.Error())), nil
		}

		summary["strategyId"] = strategyID
		summary["strategyName"] = script.Name
		summary["currentVersion"] = script.Version

		data, _ := json.MarshalIndent(summary, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

// writeFile is a helper to write content to a file.
func writeFile(path, content string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}
