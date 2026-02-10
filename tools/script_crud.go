package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ztrade/ztrade-mcp/store"
)

func registerSaveScript(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("save_script",
		mcp.WithDescription("Save a new strategy script to the database. The script content will be stored with version tracking. Use this to persist strategy scripts for management and backtesting."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Unique script name (e.g., 'ema_cross_v1')")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Full strategy source code (Go code)")),
		mcp.WithString("description", mcp.Description("Brief description of the strategy")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags (e.g., 'trend,ema,momentum')")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		script := &store.Script{
			Name:        req.GetString("name", ""),
			Content:     req.GetString("content", ""),
			Description: req.GetString("description", ""),
			Tags:        req.GetString("tags", ""),
			Language:    "go",
		}

		if err := st.CreateScript(script); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to save script: %s", err.Error())), nil
		}

		result := map[string]interface{}{
			"status":  "saved",
			"id":      script.ID,
			"name":    script.Name,
			"version": script.Version,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerGetScript(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("get_script",
		mcp.WithDescription("Retrieve a strategy script by ID or name. Returns full script content and metadata."),
		mcp.WithNumber("id", mcp.Description("Script ID")),
		mcp.WithString("name", mcp.Description("Script name. Used if id is not provided.")),
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

func registerListScripts(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("list_scripts",
		mcp.WithDescription("List all strategy scripts in the database with optional filters. Returns script metadata (without full content for brevity)."),
		mcp.WithString("status", mcp.Description("Filter by status: active, archived, deleted. Default: show all non-deleted.")),
		mcp.WithString("keyword", mcp.Description("Search keyword to filter by name, description, or tags.")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		status := req.GetString("status", "")
		keyword := req.GetString("keyword", "")

		scripts, err := st.ListScripts(status, keyword)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list scripts: %s", err.Error())), nil
		}

		// Return metadata only (omit full content for brevity)
		type scriptSummary struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Tags        string `json:"tags"`
			Status      string `json:"status"`
			Version     int    `json:"version"`
			Language    string `json:"language"`
			CreatedAt   string `json:"createdAt"`
			UpdatedAt   string `json:"updatedAt"`
		}

		var summaries []scriptSummary
		for _, sc := range scripts {
			summaries = append(summaries, scriptSummary{
				ID:          sc.ID,
				Name:        sc.Name,
				Description: sc.Description,
				Tags:        sc.Tags,
				Status:      sc.Status,
				Version:     sc.Version,
				Language:    sc.Language,
				CreatedAt:   sc.CreatedAt.Format("2006-01-02 15:04:05"),
				UpdatedAt:   sc.UpdatedAt.Format("2006-01-02 15:04:05"),
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

func registerUpdateScript(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("update_script",
		mcp.WithDescription("Update a strategy script's content. Automatically creates a new version. Use update_script_meta for metadata changes."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Script ID to update")),
		mcp.WithString("content", mcp.Required(), mcp.Description("New script content (full source code)")),
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

func registerUpdateScriptMeta(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("update_script_meta",
		mcp.WithDescription("Update a script's metadata (name, description, tags, status) without creating a new version."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Script ID to update")),
		mcp.WithString("name", mcp.Description("New script name")),
		mcp.WithString("description", mcp.Description("New description")),
		mcp.WithString("tags", mcp.Description("New tags (comma-separated)")),
		mcp.WithString("status", mcp.Description("New status: active, archived")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}

		id := int64(req.GetFloat("id", 0))

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

		if len(fields) == 0 {
			return mcp.NewToolResultError("at least one field must be provided to update"), nil
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

func registerDeleteScript(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("delete_script",
		mcp.WithDescription("Soft-delete a strategy script. The script is marked as 'deleted' but can still be queried if needed. Version history is preserved."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Script ID to delete")),
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
