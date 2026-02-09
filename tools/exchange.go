package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/viper"
)

func registerListExchanges(s *server.MCPServer, cfg *viper.Viper) {
	tool := mcp.NewTool("list_exchanges",
		mcp.WithDescription("List all configured exchanges from the config file. Returns exchange name, type, kind, and whether API keys are set."),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		exchangesCfg := cfg.GetStringMap("exchanges")
		if len(exchangesCfg) == 0 {
			return mcp.NewToolResultText("No exchanges configured."), nil
		}

		var result []map[string]interface{}
		for name := range exchangesCfg {
			sub := cfg.Sub("exchanges." + name)
			if sub == nil {
				continue
			}
			info := map[string]interface{}{
				"name":      name,
				"type":      sub.GetString("type"),
				"kind":      sub.GetString("kind"),
				"hasKey":    sub.GetString("key") != "" && sub.GetString("key") != "YOUR_API_KEY_HERE",
				"hasSecret": sub.GetString("secret") != "" && sub.GetString("secret") != "YOUR_API_SECRET_HERE",
				"timeout":   sub.GetString("timeout"),
			}
			result = append(result, info)
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %s", err.Error())), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})
}
