package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ztrade/ztrade/pkg/ctl"
)

func registerBuildStrategy(s *server.MCPServer) {
	tool := mcp.NewTool("build_strategy",
		mcp.WithDescription("Compile a Go strategy source file (.go) into a plugin (.so) that can be used for backtesting and live trading."),
		mcp.WithString("script", mcp.Required(), mcp.Description("Strategy source file path (.go)")),
		mcp.WithString("output", mcp.Description("Output file path (.so). Default: same name with .so extension")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		script := req.GetString("script", "")
		output := req.GetString("output", "")

		builder := ctl.NewBuilder(script, output)
		err := builder.Build()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build failed: %s", err.Error())), nil
		}

		result := map[string]interface{}{
			"status": "success",
			"script": script,
			"output": output,
		}
		if output == "" {
			result["output"] = script[:len(script)-3] + ".so"
		}

		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
