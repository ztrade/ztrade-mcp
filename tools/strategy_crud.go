package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ztrade/ztrade-mcp/store"
)

func registerGetStrategy(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("get_strategy",
		mcp.WithDescription("Retrieve a strategy by ID or name. Returns full strategy content and metadata."),
		mcp.WithNumber("id", mcp.Description("Strategy ID")),
		mcp.WithString("name", mcp.Description("Strategy name. Used if id is not provided.")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		idF := req.GetFloat("id", 0)
		name := req.GetString("name", "")

		var script *store.Script
		var err error

		if idF > 0 {
			script, err = st.GetScript(int64(idF))
		} else if name != "" {
			script, err = st.GetScriptByName(name)
		} else {
			return mcp.NewToolResultError("either 'id' or 'name' must be provided"), nil
		}

		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get script: %s", err.Error())), nil
		}

		data, _ := json.MarshalIndent(script, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerListStrategies(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("list_strategies",
		mcp.WithDescription("List all strategies in the database with optional filters. Returns strategy metadata (without full content for brevity)."),
		mcp.WithString("status", mcp.Description("Filter by status: active, archived, deleted. Default: show all non-deleted.")),
		mcp.WithString("lifecycleStatus", mcp.Description("Filter by lifecycle status: research, development, testing, stable.")),
		mcp.WithString("keyword", mcp.Description("Search keyword to filter by name, description, or tags.")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		status := req.GetString("status", "")
		lifecycleStatus := req.GetString("lifecycleStatus", "")
		keyword := req.GetString("keyword", "")

		scripts, err := st.ListScripts(status, lifecycleStatus, keyword)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list scripts: %s", err.Error())), nil
		}

		// Return metadata only (omit full content for brevity)
		type scriptSummary struct {
			ID              int64  `json:"id"`
			Name            string `json:"name"`
			Description     string `json:"description"`
			Tags            string `json:"tags"`
			Status          string `json:"status"`
			LifecycleStatus string `json:"lifecycleStatus"`
			Version         int    `json:"version"`
			Language        string `json:"language"`
			CreatedAt       string `json:"createdAt"`
			UpdatedAt       string `json:"updatedAt"`
		}

		var summaries []scriptSummary
		for _, sc := range scripts {
			summaries = append(summaries, scriptSummary{
				ID:              sc.ID,
				Name:            sc.Name,
				Description:     sc.Description,
				Tags:            sc.Tags,
				Status:          sc.Status,
				LifecycleStatus: sc.LifecycleStatus,
				Version:         sc.Version,
				Language:        sc.Language,
				CreatedAt:       sc.CreatedAt.Format("2006-01-02 15:04:05"),
				UpdatedAt:       sc.UpdatedAt.Format("2006-01-02 15:04:05"),
			})
		}

		result := map[string]interface{}{
			"total":   len(summaries),
			"scripts": summaries,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerUpdateStrategy(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("update_strategy",
		mcp.WithDescription("Update a strategy's content. Automatically creates a new version. Use update_strategy_meta for metadata changes."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Strategy ID to update")),
		mcp.WithString("content", mcp.Required(), mcp.Description("New strategy content (full source code)")),
		mcp.WithString("message", mcp.Description("Version message describing the change (e.g., 'optimize EMA parameters')")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		id := int64(req.GetFloat("id", 0))
		content := req.GetString("content", "")
		message := req.GetString("message", "")

		if message == "" {
			message = "update content"
		}

		script, err := st.UpdateScript(id, content, message)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to update script: %s", err.Error())), nil
		}

		result := map[string]interface{}{
			"status":  "updated",
			"id":      script.ID,
			"name":    script.Name,
			"version": script.Version,
			"message": message,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerUpdateStrategyMeta(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("update_strategy_meta",
		mcp.WithDescription("Update a strategy's metadata (name, description, tags, status, lifecycleStatus, fieldDescriptions) without creating a new version. If a strategy is in lifecycleStatus=stable, you must first change lifecycleStatus to research/development/testing before editing other fields."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Strategy ID to update")),
		mcp.WithString("name", mcp.Description("New strategy name")),
		mcp.WithString("description", mcp.Description("New description")),
		mcp.WithString("tags", mcp.Description("New tags (comma-separated)")),
		mcp.WithString("status", mcp.Description("New status: active, archived")),
		mcp.WithString("lifecycleStatus", mcp.Description("Lifecycle status: research, development, testing, stable")),
		mcp.WithString("fieldDescriptions", mcp.Description("Detailed field-level descriptions for the strategy. Recommended format: JSON object keyed by field/param name.")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		id := int64(req.GetFloat("id", 0))
		script, err := st.GetScript(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get script: %s", err.Error())), nil
		}

		fields := make(map[string]interface{})
		if name := req.GetString("name", ""); name != "" {
			fields["name"] = name
		}
		if desc := req.GetString("description", ""); desc != "" {
			fields["description"] = desc
		}
		if tags := req.GetString("tags", ""); tags != "" {
			fields["tags"] = tags
		}
		if status := req.GetString("status", ""); status != "" {
			if status != "active" && status != "archived" {
				return mcp.NewToolResultError("status must be 'active' or 'archived'"), nil
			}
			fields["status"] = status
		}
		if lifecycleStatus := req.GetString("lifecycleStatus", ""); lifecycleStatus != "" {
			if !store.IsValidStrategyLifecycleStatus(lifecycleStatus) {
				return mcp.NewToolResultError("lifecycleStatus must be one of: research, development, testing, stable"), nil
			}
			fields["lifecycle_status"] = lifecycleStatus
		}
		if fieldDescriptions := req.GetString("fieldDescriptions", ""); fieldDescriptions != "" {
			fields["field_descriptions"] = fieldDescriptions
		}

		if len(fields) == 0 {
			return mcp.NewToolResultError("at least one field must be provided to update"), nil
		}

		// If strategy is stable, require lifecycle unlock first
		if store.IsStrategyLockedForEdit(script.LifecycleStatus) {
			nextLifecycle, hasLifecycle := fields["lifecycle_status"]
			if !hasLifecycle || len(fields) != 1 {
				return mcp.NewToolResultError("strategy is stable; update lifecycleStatus first (research/development/testing)"), nil
			}
			ls, ok := nextLifecycle.(string)
			if !ok || ls == store.StrategyLifecycleStable {
				return mcp.NewToolResultError("strategy is stable; set lifecycleStatus to research/development/testing before other edits"), nil
			}
		}

		if err := st.UpdateScriptMeta(id, fields); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to update script meta: %s", err.Error())), nil
		}

		result := map[string]interface{}{
			"status":  "updated",
			"id":      id,
			"updated": fields,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerDeleteStrategy(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("delete_strategy",
		mcp.WithDescription("Soft-delete a strategy. The strategy is marked as 'deleted' but can still be queried if needed. Version history is preserved."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Strategy ID to delete")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		id := int64(req.GetFloat("id", 0))

		// Verify the script exists
		script, err := st.GetScript(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to find script: %s", err.Error())), nil
		}

		if err := st.DeleteScript(id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to delete script: %s", err.Error())), nil
		}

		result := map[string]interface{}{
			"status": "deleted",
			"id":     id,
			"name":   script.Name,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
