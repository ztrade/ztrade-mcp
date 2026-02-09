package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ztrade/ztrade/pkg/ctl"
	"github.com/ztrade/ztrade/pkg/process/dbstore"
)

func registerListData(s *server.MCPServer, db *dbstore.DBStore) {
	tool := mcp.NewTool("list_data",
		mcp.WithDescription("List all available K-line data stored in the local database. Returns exchange, symbol, binSize, start time, and end time for each dataset."),
		mcp.WithString("exchange", mcp.Description("Filter by exchange name (e.g., binance, okx). Optional.")),
		mcp.WithString("symbol", mcp.Description("Filter by trading pair (e.g., BTCUSDT). Optional.")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if db == nil {
			return mcp.NewToolResultError("database not initialized"), nil
		}

		ld, err := ctl.NewLocalData(db)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create local data: %s", err.Error())), nil
		}

		infos, err := ld.ListAll()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list data: %s", err.Error())), nil
		}

		// Apply filters
		exchangeFilter := req.GetString("exchange", "")
		symbolFilter := req.GetString("symbol", "")

		var filtered []map[string]interface{}
		for _, info := range infos {
			if exchangeFilter != "" && !strings.EqualFold(info.Exchange, exchangeFilter) {
				continue
			}
			if symbolFilter != "" && !strings.EqualFold(info.Symbol, symbolFilter) {
				continue
			}
			filtered = append(filtered, map[string]interface{}{
				"exchange": info.Exchange,
				"symbol":   info.Symbol,
				"binSize":  info.BinSize,
				"start":    info.Start.Format("2006-01-02 15:04:05"),
				"end":      info.End.Format("2006-01-02 15:04:05"),
			})
		}

		data, _ := json.MarshalIndent(filtered, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
