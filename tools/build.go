package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ztrade/ztrade-mcp/store"
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

		// --- 支持从数据库查找策略 ---
		var goPath string
		var soPath string
		st := getStoreFromContext(ctx)
		if st != nil && script != "" && (isLikelyID(script) || isLikelyName(script)) {
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
			if err := writeFile(goPath, s.Content); err != nil {
				return mcp.NewToolResultError("failed to write temp go file: " + err.Error()), nil
			}
			script = goPath
			if output == "" {
				output = soPath
			}
		}

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
