package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ztrade/ztrade/pkg/ctl"
	"github.com/ztrade/ztrade/pkg/process/dbstore"
	"github.com/ztrade/ztrade/pkg/report"
)

// runBacktestCore executes the actual backtest logic and returns the result map or error.
func runBacktestCore(db *dbstore.DBStore, script, exchangeName, symbol, param string, start, end time.Time, balanceF, feeF, leverF float64) (map[string]interface{}, error) {
	bt, err := ctl.NewBacktest(db, exchangeName, symbol, param, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to create backtest: %s", err.Error())
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
	return result, nil
}

func registerRunBacktest(s *server.MCPServer, db *dbstore.DBStore, tm *TaskManager) {
	tool := mcp.NewTool("run_backtest",
		mcp.WithDescription("Run a backtest with a strategy script on historical data. Returns structured results including profit, win rate, sharpe ratio, max drawdown, etc. When the time range exceeds 30 days the task runs asynchronously — a task ID is returned immediately and you can poll progress with get_task_status / get_task_result."),
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
		       exchangeName := req.GetString("exchange", "")
		       symbol := req.GetString("symbol", "")
		       startStr := req.GetString("start", "")
		       endStr := req.GetString("end", "")
		       balanceF := req.GetFloat("balance", 0)
		       feeF := req.GetFloat("fee", 0)
		       leverF := req.GetFloat("lever", 0)
		       param := req.GetString("param", "")

		       // --- 自动从数据库读取策略并编译为so ---
		       var soPath string
		       var goPath string
		       var useSo bool
		       st := getStoreFromContext(ctx)
		       if st != nil && script != "" && (isLikelyID(script) || isLikelyName(script)) {
			       // 允许 script 传入策略ID或名称
			       var s *store.Script
			       var err error
			       if isLikelyID(script) {
				       id, _ := parseID(script)
				       s, err = st.GetScript(id)
			       } else {
				       s, err = st.GetScriptByName(script)
			       }
			       if err != nil {
				       return mcp.NewToolResultError("strategy not found: " + err.Error()), nil
			       }
			       goPath = fmt.Sprintf("/tmp/ztrade_plugins/%s_v%d.go", s.Name, s.Version)
			       soPath = fmt.Sprintf("/tmp/ztrade_plugins/%s_v%d.so", s.Name, s.Version)
			       // 写入go文件
			       if err := writeFile(goPath, s.Content); err != nil {
				       return mcp.NewToolResultError("failed to write temp go file: " + err.Error()), nil
			       }
			       // 编译so
			       builder := ctl.NewBuilder(goPath, soPath)
			       if err := builder.Build(); err != nil {
				       return mcp.NewToolResultError("build failed: " + err.Error()), nil
			       }
			       script = soPath
			       useSo = true
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

		       // If time range > threshold, run asynchronously
		       if ShouldRunAsync(start, end) {
			       taskID := tm.CreateTask("backtest", map[string]string{
				       "script":   script,
				       "exchange": exchangeName,
				       "symbol":   symbol,
				       "start":    startStr,
				       "end":      endStr,
			       })

			       go func() {
				       tm.StartTask(taskID)
				       doneCh := tm.ProgressEstimator(taskID, "backtest", start, end)

				       result, err := runBacktestCore(db, script, exchangeName, symbol, param, start, end, balanceF, feeF, leverF)
				       close(doneCh)

				       if err != nil {
					       log.Errorf("async backtest task %s failed: %s", taskID, err.Error())
					       tm.FailTask(taskID, err.Error())
					       return
				       }

				       data, _ := json.MarshalIndent(result, "", "  ")
				       tm.CompleteTask(taskID, string(data))
				       log.Infof("async backtest task %s completed", taskID)
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
		       result, err := runBacktestCore(db, script, exchangeName, symbol, param, start, end, balanceF, feeF, leverF)
		       if err != nil {
			       return mcp.NewToolResultError(err.Error()), nil
		       }

		       data, _ := json.MarshalIndent(result, "", "  ")
		       return mcp.NewToolResultText(string(data)), nil
	       })
	}

	// getStoreFromContext 尝试从 context 获取 *store.Store
	func getStoreFromContext(ctx context.Context) *store.Store {
	       v := ctx.Value("store")
	       if v == nil {
		       return nil
	       }
	       st, ok := v.(*store.Store)
	       if !ok {
		       return nil
	       }
	       return st
	}

	// isLikelyID 判断字符串是否为数字ID
	func isLikelyID(s string) bool {
	       _, err := parseID(s)
	       return err == nil
	}

	func parseID(s string) (int64, error) {
	       var id int64
	       _, err := fmt.Sscanf(s, "%d", &id)
	       return id, err
	}

	// isLikelyName 判断是否为合法策略名（可根据实际需求调整）
	func isLikelyName(s string) bool {
	       // 只要不是纯路径或.so/.go文件名就认为是名字
	       if len(s) == 0 {
		       return false
	       }
	       if len(s) > 3 && (s[len(s)-3:] == ".go" || s[len(s)-3:] == ".so") {
		       return false
	       }
	       if len(s) > 0 && (s[0] == '/' || s[0] == '.') {
		       return false
	       }
	       return true
	}

	// writeFile 写入文件
	func writeFile(path, content string) error {
	       f, err := os.Create(path)
	       if err != nil {
		       return err
	       }
	       defer f.Close()
	       _, err = f.WriteString(content)
	       return err
	})
}
