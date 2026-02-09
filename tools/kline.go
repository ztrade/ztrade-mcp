package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ztrade/trademodel"
	"github.com/ztrade/ztrade/pkg/process/dbstore"
)

func registerQueryKline(s *server.MCPServer, db *dbstore.DBStore) {
	tool := mcp.NewTool("query_kline",
		mcp.WithDescription("Query K-line (candlestick) data from local database for analysis. Returns OHLCV data."),
		mcp.WithString("exchange", mcp.Required(), mcp.Description("Exchange name (e.g., binance, okx)")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Trading pair (e.g., BTCUSDT)")),
		mcp.WithString("binSize", mcp.Description("K-line period (1m/5m/15m/1h/1d). Default: 1m")),
		mcp.WithString("start", mcp.Required(), mcp.Description("Start time in format '2006-01-02 15:04:05'")),
		mcp.WithString("end", mcp.Required(), mcp.Description("End time in format '2006-01-02 15:04:05'")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of candles to return. Default: 500, Max: 5000")),
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
		limitF := req.GetFloat("limit", 0)

		if binSize == "" {
			binSize = "1m"
		}
		limit := int(limitF)
		if limit <= 0 {
			limit = 500
		}
		if limit > 5000 {
			limit = 5000
		}

		start, err := time.Parse("2006-01-02 15:04:05", startStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid start time: %s", err.Error())), nil
		}
		end, err := time.Parse("2006-01-02 15:04:05", endStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid end time: %s", err.Error())), nil
		}

		tbl := db.GetKlineTbl(exchange, symbol, binSize)
		datas, err := tbl.GetDatas(start, end, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query failed: %s", err.Error())), nil
		}

		type klineEntry struct {
			Time   string  `json:"time"`
			Open   float64 `json:"open"`
			High   float64 `json:"high"`
			Low    float64 `json:"low"`
			Close  float64 `json:"close"`
			Volume float64 `json:"volume"`
		}

		var entries []klineEntry
		for _, d := range datas {
			candle, ok := d.(*trademodel.Candle)
			if !ok {
				continue
			}
			entries = append(entries, klineEntry{
				Time:   candle.Time().Format("2006-01-02 15:04:05"),
				Open:   candle.Open,
				High:   candle.High,
				Low:    candle.Low,
				Close:  candle.Close,
				Volume: candle.Volume,
			})
		}

		result := map[string]interface{}{
			"exchange": exchange,
			"symbol":   symbol,
			"binSize":  binSize,
			"count":    len(entries),
			"candles":  entries,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
