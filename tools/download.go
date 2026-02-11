package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/viper"
	"github.com/ztrade/ztrade/pkg/ctl"
	"github.com/ztrade/ztrade/pkg/process/dbstore"
)

func registerDownloadKline(s *server.MCPServer, db *dbstore.DBStore, cfg *viper.Viper, tm *TaskManager) {
	tool := mcp.NewTool("download_kline",
		mcp.WithDescription("Download historical K-line data from an exchange to local database. Requires exchange API configuration. When the time range exceeds 30 days the task runs asynchronously — a task ID is returned immediately and you can poll progress with get_task_status / get_task_result."),
		mcp.WithString("exchange", mcp.Required(), mcp.Description("Exchange name (e.g., binance, okx)")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Trading pair (e.g., BTCUSDT)")),
		mcp.WithString("binSize", mcp.Description("K-line period (1m/5m/15m/1h/1d). Default: 1m")),
		mcp.WithString("start", mcp.Description("Start time in format '2006-01-02 15:04:05'. Required if auto=false.")),
		mcp.WithString("end", mcp.Description("End time in format '2006-01-02 15:04:05'. Required if auto=false.")),
		mcp.WithBoolean("auto", mcp.Description("Auto-continue download from the latest data in DB to now. Default: false")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if db == nil {
			return mcp.NewToolResultError("database not initialized"), nil
		}

		exchange := req.GetString("exchange", "")
		symbol := req.GetString("symbol", "")
		binSize := req.GetString("binSize", "")
		startStr := req.GetString("start", "")
		endStr := req.GetString("end", "")
		auto := req.GetBool("auto", false)

		if binSize == "" {
			binSize = "1m"
		}

		// For auto mode or manual mode, determine whether to run async
		if auto {
			// Auto mode: always run async since time range is unknown and could be large
			taskID := tm.CreateTask("download", map[string]string{
				"exchange": exchange,
				"symbol":   symbol,
				"binSize":  binSize,
				"mode":     "auto",
			})

			go func() {
				tm.StartTask(taskID)
				// Auto mode: estimate 90 days range for progress display
				estEnd := time.Now()
				estStart := estEnd.AddDate(0, -3, 0)
				doneCh := tm.ProgressEstimator(taskID, "download", estStart, estEnd)

				d := ctl.NewDataDownloadAuto(cfg, db, exchange, symbol, binSize)
				err := d.Run()
				close(doneCh)

				if err != nil {
					log.Errorf("async download task %s failed: %s", taskID, err.Error())
					tm.FailTask(taskID, fmt.Sprintf("download failed: %s", err.Error()))
					return
				}

				result := map[string]interface{}{
					"status":   "completed",
					"exchange": exchange,
					"symbol":   symbol,
					"binSize":  binSize,
					"mode":     "auto",
				}
				data, _ := json.MarshalIndent(result, "", "  ")
				tm.CompleteTask(taskID, string(data))
				log.Infof("async download task %s completed", taskID)
			}()

			asyncResult := map[string]interface{}{
				"async":   true,
				"taskId":  taskID,
				"message": fmt.Sprintf("Auto download started asynchronously. Use get_task_status with taskId '%s' to check progress, or get_task_result to retrieve the final result.", taskID),
			}
			data, _ := json.MarshalIndent(asyncResult, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		}

		// Manual mode: parse start/end
		if startStr == "" || endStr == "" {
			return mcp.NewToolResultError("start and end time are required when auto=false"), nil
		}
		start, err := time.Parse("2006-01-02 15:04:05", startStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid start time: %s", err.Error())), nil
		}
		end, err := time.Parse("2006-01-02 15:04:05", endStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid end time: %s", err.Error())), nil
		}

		// If time range > threshold, run asynchronously
		if ShouldRunAsync(start, end) {
			taskID := tm.CreateTask("download", map[string]string{
				"exchange": exchange,
				"symbol":   symbol,
				"binSize":  binSize,
				"start":    startStr,
				"end":      endStr,
			})

			go func() {
				tm.StartTask(taskID)
				doneCh := tm.ProgressEstimator(taskID, "download", start, end)

				d := ctl.NewDataDownload(cfg, db, exchange, symbol, binSize, start, end)
				err := d.Run()
				close(doneCh)

				if err != nil {
					log.Errorf("async download task %s failed: %s", taskID, err.Error())
					tm.FailTask(taskID, fmt.Sprintf("download failed: %s", err.Error()))
					return
				}

				result := map[string]interface{}{
					"status":   "completed",
					"exchange": exchange,
					"symbol":   symbol,
					"binSize":  binSize,
					"start":    startStr,
					"end":      endStr,
				}
				data, _ := json.MarshalIndent(result, "", "  ")
				tm.CompleteTask(taskID, string(data))
				log.Infof("async download task %s completed", taskID)
			}()

			asyncResult := map[string]interface{}{
				"async":   true,
				"taskId":  taskID,
				"message": fmt.Sprintf("Download time range exceeds %d days, running asynchronously. Use get_task_status with taskId '%s' to check progress, or get_task_result to retrieve the final result.", AsyncThresholdDays, taskID),
			}
			data, _ := json.MarshalIndent(asyncResult, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		}

		// Synchronous execution for short time ranges
		d := ctl.NewDataDownload(cfg, db, exchange, symbol, binSize, start, end)
		err = d.Run()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("download failed: %s", err.Error())), nil
		}

		result := map[string]interface{}{
			"status":   "completed",
			"exchange": exchange,
			"symbol":   symbol,
			"binSize":  binSize,
			"start":    startStr,
			"end":      endStr,
		}

		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
