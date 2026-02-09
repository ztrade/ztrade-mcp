package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/viper"
	"github.com/ztrade/ztrade/pkg/ctl"
	"github.com/ztrade/ztrade/pkg/process/dbstore"
)

func registerDownloadKline(s *server.MCPServer, db *dbstore.DBStore, cfg *viper.Viper) {
	tool := mcp.NewTool("download_kline",
		mcp.WithDescription("Download historical K-line data from an exchange to local database. Requires exchange API configuration."),
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

		var d *ctl.DataDownload
		if auto {
			d = ctl.NewDataDownloadAuto(cfg, db, exchange, symbol, binSize)
		} else {
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
			d = ctl.NewDataDownload(cfg, db, exchange, symbol, binSize, start, end)
		}

		err := d.Run()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("download failed: %s", err.Error())), nil
		}

		result := map[string]interface{}{
			"status":   "completed",
			"exchange": exchange,
			"symbol":   symbol,
			"binSize":  binSize,
		}
		if !auto {
			result["start"] = startStr
			result["end"] = endStr
		} else {
			result["mode"] = "auto"
		}

		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
