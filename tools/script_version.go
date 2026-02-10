package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ztrade/ztrade-mcp/store"
)

func registerListScriptVersions(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("list_script_versions",
		mcp.WithDescription("List all versions of a strategy script. Returns version number, change message, and creation time for each version."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Script ID")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		id := int64(req.GetFloat("id", 0))

		// Get script info
		script, err := st.GetScript(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get script: %s", err.Error())), nil
		}

		versions, err := st.ListVersions(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list versions: %s", err.Error())), nil
		}

		type versionSummary struct {
			Version   int    `json:"version"`
			Message   string `json:"message"`
			CreatedAt string `json:"createdAt"`
		}

		var summaries []versionSummary
		for _, v := range versions {
			summaries = append(summaries, versionSummary{
				Version:   v.Version,
				Message:   v.Message,
				CreatedAt: v.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}

		result := map[string]interface{}{
			"scriptId":       id,
			"scriptName":     script.Name,
			"currentVersion": script.Version,
			"totalVersions":  len(summaries),
			"versions":       summaries,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerGetScriptVersion(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("get_script_version",
		mcp.WithDescription("Get the full content of a specific version of a script. Useful for reviewing or comparing historical versions."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Script ID")),
		mcp.WithNumber("version", mcp.Required(), mcp.Description("Version number to retrieve")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		id := int64(req.GetFloat("id", 0))
		version := int(req.GetFloat("version", 0))

		ver, err := st.GetVersion(id, version)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get version: %s", err.Error())), nil
		}

		result := map[string]interface{}{
			"scriptId":  id,
			"version":   ver.Version,
			"message":   ver.Message,
			"content":   ver.Content,
			"createdAt": ver.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerDiffScriptVersions(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("diff_script_versions",
		mcp.WithDescription("Compare two versions of a script by showing both versions' content side by side. Use this to review changes between versions."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Script ID")),
		mcp.WithNumber("version1", mcp.Required(), mcp.Description("First (older) version number")),
		mcp.WithNumber("version2", mcp.Required(), mcp.Description("Second (newer) version number")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		id := int64(req.GetFloat("id", 0))
		v1 := int(req.GetFloat("version1", 0))
		v2 := int(req.GetFloat("version2", 0))

		ver1, ver2, err := st.DiffVersions(id, v1, v2)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to diff versions: %s", err.Error())), nil
		}

		// Simple line-based diff
		lines1 := strings.Split(ver1.Content, "\n")
		lines2 := strings.Split(ver2.Content, "\n")

		var diffLines []string
		maxLen := len(lines1)
		if len(lines2) > maxLen {
			maxLen = len(lines2)
		}

		for i := 0; i < maxLen; i++ {
			var l1, l2 string
			if i < len(lines1) {
				l1 = lines1[i]
			}
			if i < len(lines2) {
				l2 = lines2[i]
			}
			if l1 != l2 {
				if i < len(lines1) {
					diffLines = append(diffLines, fmt.Sprintf("- [v%d L%d] %s", v1, i+1, l1))
				}
				if i < len(lines2) {
					diffLines = append(diffLines, fmt.Sprintf("+ [v%d L%d] %s", v2, i+1, l2))
				}
			}
		}

		result := map[string]interface{}{
			"scriptId": id,
			"version1": map[string]interface{}{
				"version":   ver1.Version,
				"message":   ver1.Message,
				"createdAt": ver1.CreatedAt.Format("2006-01-02 15:04:05"),
				"content":   ver1.Content,
			},
			"version2": map[string]interface{}{
				"version":   ver2.Version,
				"message":   ver2.Message,
				"createdAt": ver2.CreatedAt.Format("2006-01-02 15:04:05"),
				"content":   ver2.Content,
			},
			"changes": len(diffLines),
			"diff":    strings.Join(diffLines, "\n"),
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerRollbackScript(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("rollback_script",
		mcp.WithDescription("Rollback a strategy script to a previous version. Creates a new version with the rolled-back content."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Script ID")),
		mcp.WithNumber("version", mcp.Required(), mcp.Description("Target version to rollback to")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		id := int64(req.GetFloat("id", 0))
		version := int(req.GetFloat("version", 0))

		script, err := st.RollbackScript(id, version)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to rollback: %s", err.Error())), nil
		}

		result := map[string]interface{}{
			"status":         "rolled back",
			"id":             script.ID,
			"name":           script.Name,
			"rolledBackTo":   version,
			"currentVersion": script.Version,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
