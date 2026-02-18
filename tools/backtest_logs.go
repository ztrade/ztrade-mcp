package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ztrade/ztrade-mcp/store"
)

func registerGetBacktestLogs(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("get_backtest_logs",
		mcp.WithDescription("Get captured backtest logs (engine.Log output) for a saved backtest record."),
		mcp.WithNumber("recordId", mcp.Required(), mcp.Description("Backtest record ID")),
		mcp.WithNumber("offset", mcp.Description("Pagination offset (default: 0)")),
		mcp.WithNumber("limit", mcp.Description("Max lines to return (default: 200, max: 2000)")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		recordID := int64(req.GetFloat("recordId", 0))
		offset := int(req.GetFloat("offset", 0))
		limit := int(req.GetFloat("limit", 0))

		logs, total, err := st.ListBacktestLogs(recordID, offset, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list backtest logs: %s", err.Error())), nil
		}

		lines := make([]string, 0, len(logs))
		for _, l := range logs {
			lines = append(lines, l.Content)
		}

		result := map[string]interface{}{
			"recordId": recordID,
			"total":    total,
			"offset":   offset,
			"limit":    limit,
			"lines":    lines,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
