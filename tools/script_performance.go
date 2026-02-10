package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ztrade/ztrade-mcp/store"
	"github.com/ztrade/ztrade/pkg/ctl"
	"github.com/ztrade/ztrade/pkg/process/dbstore"
	"github.com/ztrade/ztrade/pkg/report"
)

func registerRunBacktestManaged(s *server.MCPServer, db *dbstore.DBStore, st *store.Store) {
	tool := mcp.NewTool("run_backtest_managed",
		mcp.WithDescription("Run a backtest using a managed script from the database. The script is extracted from DB, backtested, and results are automatically saved for performance tracking."),
		mcp.WithNumber("scriptId", mcp.Required(), mcp.Description("Script ID in the database")),
		mcp.WithString("exchange", mcp.Required(), mcp.Description("Exchange name (e.g., binance)")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Trading pair (e.g., BTCUSDT)")),
		mcp.WithString("start", mcp.Required(), mcp.Description("Backtest start time in format '2006-01-02 15:04:05'")),
		mcp.WithString("end", mcp.Required(), mcp.Description("Backtest end time in format '2006-01-02 15:04:05'")),
		mcp.WithNumber("balance", mcp.Description("Initial balance. Default: 100000")),
		mcp.WithNumber("fee", mcp.Description("Trading fee rate. Default: 0.0001")),
		mcp.WithNumber("lever", mcp.Description("Leverage multiplier. Default: 1")),
		mcp.WithString("param", mcp.Description("Strategy parameters as JSON string")),
		mcp.WithNumber("version", mcp.Description("Script version to use. Default: latest version.")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if db == nil {
			return mcp.NewToolResultError("database not initialized"), nil
		}
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		scriptID := int64(req.GetFloat("scriptId", 0))
		exchangeName := req.GetString("exchange", "")
		symbol := req.GetString("symbol", "")
		startStr := req.GetString("start", "")
		endStr := req.GetString("end", "")
		balanceF := req.GetFloat("balance", 0)
		feeF := req.GetFloat("fee", 0)
		leverF := req.GetFloat("lever", 0)
		param := req.GetString("param", "")
		versionF := req.GetFloat("version", 0)

		// Get script from DB
		script, err := st.GetScript(scriptID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get script: %s", err.Error())), nil
		}

		// If a specific version is requested, get that version's content
		scriptContent := script.Content
		scriptVersion := script.Version
		if versionF > 0 {
			ver, err := st.GetVersion(scriptID, int(versionF))
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
			feeF = 0.0001
		}
		if leverF <= 0 {
			leverF = 1
		}

		// Write script to temp file for backtesting
		tmpFile := fmt.Sprintf("/tmp/ztrade_script_%d_v%d.go", scriptID, scriptVersion)
		if err := writeFile(tmpFile, scriptContent); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to write temp script: %s", err.Error())), nil
		}

		bt, err := ctl.NewBacktest(db, exchangeName, symbol, param, start, end)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create backtest: %s", err.Error())), nil
		}

		bt.SetScript(tmpFile)
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

		// Save backtest record
		record := &store.BacktestRecord{
			ScriptID:         scriptID,
			ScriptVersion:    scriptVersion,
			Exchange:         exchangeName,
			Symbol:           symbol,
			StartTime:        start,
			EndTime:          end,
			InitBalance:      balanceF,
			Fee:              feeF,
			Lever:            leverF,
			Param:            param,
			TotalActions:     resultData.TotalAction,
			WinRate:          resultData.WinRate,
			TotalProfit:      resultData.TotalProfit,
			ProfitPercent:    resultData.ProfitPercent,
			MaxDrawdown:      resultData.MaxDrawdown,
			MaxDrawdownValue: resultData.MaxDrawdownValue,
			MaxLose:          resultData.MaxLose,
			TotalFee:         resultData.TotalFee,
			StartBalance:     resultData.StartBalance,
			EndBalance:       resultData.EndBalance,
			TotalReturn:      resultData.TotalReturn,
			AnnualReturn:     resultData.AnnualReturn,
			SharpeRatio:      resultData.SharpeRatio,
			SortinoRatio:     resultData.SortinoRatio,
			Volatility:       resultData.Volatility,
			ProfitFactor:     resultData.ProfitFactor,
			CalmarRatio:      resultData.CalmarRatio,
			OverallScore:     resultData.OverallScore,
			LongTrades:       resultData.LongTrades,
			ShortTrades:      resultData.ShortTrades,
		}

		if err := st.SaveBacktestRecord(record); err != nil {
			// Don't fail the whole operation, just warn
			return mcp.NewToolResultText(fmt.Sprintf("backtest completed but failed to save record: %s", err.Error())), nil
		}

		result := map[string]interface{}{
			"recordId":         record.ID,
			"scriptId":         scriptID,
			"scriptName":       script.Name,
			"scriptVersion":    scriptVersion,
			"exchange":         exchangeName,
			"symbol":           symbol,
			"totalActions":     resultData.TotalAction,
			"winRate":          resultData.WinRate,
			"totalProfit":      resultData.TotalProfit,
			"profitPercent":    resultData.ProfitPercent,
			"maxDrawdown":      resultData.MaxDrawdown,
			"maxDrawdownValue": resultData.MaxDrawdownValue,
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

func registerListBacktestRecords(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("list_backtest_records",
		mcp.WithDescription("List backtest history for a script. Returns all backtest runs with performance metrics, ordered by most recent first."),
		mcp.WithNumber("scriptId", mcp.Required(), mcp.Description("Script ID")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of records to return. Default: 20")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		scriptID := int64(req.GetFloat("scriptId", 0))
		limit := int(req.GetFloat("limit", 0))
		if limit <= 0 {
			limit = 20
		}

		records, err := st.ListBacktestRecords(scriptID, limit)
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
			"scriptId": scriptID,
			"total":    len(summaries),
			"records":  summaries,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerScriptPerformance(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("script_performance",
		mcp.WithDescription("Get aggregated performance summary for a script across all backtests. Includes best/worst runs, average score, and key metrics ranges."),
		mcp.WithNumber("scriptId", mcp.Required(), mcp.Description("Script ID")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		scriptID := int64(req.GetFloat("scriptId", 0))

		// Get script info
		script, err := st.GetScript(scriptID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get script: %s", err.Error())), nil
		}

		summary, err := st.GetBacktestSummary(scriptID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get performance summary: %s", err.Error())), nil
		}

		summary["scriptId"] = scriptID
		summary["scriptName"] = script.Name
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
