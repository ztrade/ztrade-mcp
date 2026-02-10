package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/viper"
	"github.com/ztrade/exchange"
)

func registerFetchKline(s *server.MCPServer, cfg *viper.Viper) {
	tool := mcp.NewTool("fetch_kline",
		mcp.WithDescription("Fetch K-line (candlestick) data directly from an exchange API without saving to local database. Useful for quick analysis or checking recent market data."),
		mcp.WithString("exchange", mcp.Required(), mcp.Description("Exchange config name (e.g., binance, okx). Must be configured in the config file.")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Trading pair (e.g., BTCUSDT)")),
		mcp.WithString("binSize", mcp.Description("K-line period (1m/5m/15m/1h/4h/1d). Default: 1m")),
		mcp.WithString("start", mcp.Required(), mcp.Description("Start time in format '2006-01-02 15:04:05'")),
		mcp.WithString("end", mcp.Description("End time in format '2006-01-02 15:04:05'. Default: now")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of candles to return. Default: 500, Max: 1500")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		exchangeName := req.GetString("exchange", "")
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
		if limit > 1500 {
			limit = 1500
		}

		start, err := time.Parse("2006-01-02 15:04:05", startStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid start time: %s", err.Error())), nil
		}

		var end time.Time
		if endStr != "" {
			end, err = time.Parse("2006-01-02 15:04:05", endStr)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid end time: %s", err.Error())), nil
			}
		} else {
			end = time.Now()
		}

		// Get exchange type from config
		exchangeType := cfg.GetString(fmt.Sprintf("exchanges.%s.type", exchangeName))
		if exchangeType == "" {
			return mcp.NewToolResultError(fmt.Sprintf("exchange '%s' not found in config. Use list_exchanges to see configured exchanges.", exchangeName)), nil
		}

		// Create exchange client
		exchangeCfg := exchange.WrapViper(cfg)
		ex, err := exchange.NewExchange(exchangeType, exchangeCfg, exchangeName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create exchange client: %s", err.Error())), nil
		}

		// Fetch kline data from exchange API
		candles, err := ex.GetKline(symbol, binSize, start, end)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to fetch kline: %s", err.Error())), nil
		}

		// Apply limit
		if len(candles) > limit {
			candles = candles[len(candles)-limit:]
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
		for _, candle := range candles {
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
			"exchange": exchangeName,
			"symbol":   symbol,
			"binSize":  binSize,
			"count":    len(entries),
			"candles":  entries,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
