package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/viper"
	"github.com/ztrade/exchange"
)

func registerListSymbols(s *server.MCPServer, cfg *viper.Viper) {
	tool := mcp.NewTool("list_symbols",
		mcp.WithDescription("List available trading symbols (pairs) from an exchange. Returns symbol name, precision, price/amount step, and other details."),
		mcp.WithString("exchange", mcp.Required(), mcp.Description("Exchange config name (e.g., binance, okx). Must be configured in the config file.")),
		mcp.WithString("keyword", mcp.Description("Filter symbols by keyword (e.g., BTC, ETH, USDT). Case insensitive. Optional.")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		exchangeName := req.GetString("exchange", "")

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

		// Fetch symbols
		symbols, err := ex.Symbols()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to fetch symbols: %s", err.Error())), nil
		}

		// Apply keyword filter
		keyword := strings.ToUpper(req.GetString("keyword", ""))

		type symbolEntry struct {
			Symbol          string  `json:"symbol"`
			Name            string  `json:"name,omitempty"`
			Type            string  `json:"type,omitempty"`
			Precision       int     `json:"precision"`
			AmountPrecision int     `json:"amountPrecision"`
			PriceStep       float64 `json:"priceStep"`
			AmountStep      float64 `json:"amountStep"`
		}

		var entries []symbolEntry
		for _, sym := range symbols {
			if keyword != "" && !strings.Contains(strings.ToUpper(sym.Symbol), keyword) &&
				!strings.Contains(strings.ToUpper(sym.Name), keyword) {
				continue
			}
			entries = append(entries, symbolEntry{
				Symbol:          sym.Symbol,
				Name:            sym.Name,
				Type:            sym.Type,
				Precision:       sym.Precision,
				AmountPrecision: sym.AmountPrecision,
				PriceStep:       sym.PriceStep,
				AmountStep:      sym.AmountStep,
			})
		}

		result := map[string]interface{}{
			"exchange": exchangeName,
			"total":    len(entries),
			"symbols":  entries,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
