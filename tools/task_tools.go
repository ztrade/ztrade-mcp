package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerGetTaskStatus(s *server.MCPServer, tm *TaskManager) {
	tool := mcp.NewTool("get_task_status",
		mcp.WithDescription("Get the current status and progress of an async task (backtest or download). Returns task status (pending/running/completed/failed), progress description and completion percentage."),
		mcp.WithString("taskId", mcp.Required(), mcp.Description("The task ID returned by an async backtest or download call")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID := req.GetString("taskId", "")

		task, err := tm.GetTask(taskID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		status := map[string]interface{}{
			"taskId":   task.ID,
			"type":     task.Type,
			"status":   task.Status,
			"progress": task.Progress,
			"percent":  task.Percent,
			"params":   task.Params,
		}
		if task.StartedAt != nil {
			status["startedAt"] = task.StartedAt.Format("2006-01-02 15:04:05")
		}
		if task.EndedAt != nil {
			status["endedAt"] = task.EndedAt.Format("2006-01-02 15:04:05")
			status["duration"] = task.EndedAt.Sub(*task.StartedAt).String()
		}
		if task.Status == TaskStatusFailed {
			status["error"] = task.Error
		}

		data, _ := json.MarshalIndent(status, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerGetTaskResult(s *server.MCPServer, tm *TaskManager) {
	tool := mcp.NewTool("get_task_result",
		mcp.WithDescription("Get the final result of a completed async task (backtest or download). Returns the full result data if the task is completed, or current status if still running."),
		mcp.WithString("taskId", mcp.Required(), mcp.Description("The task ID returned by an async backtest or download call")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID := req.GetString("taskId", "")

		task, err := tm.GetTask(taskID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		switch task.Status {
		case TaskStatusCompleted:
			result := map[string]interface{}{
				"taskId": task.ID,
				"type":   task.Type,
				"status": task.Status,
			}
			if task.StartedAt != nil && task.EndedAt != nil {
				result["duration"] = task.EndedAt.Sub(*task.StartedAt).String()
			}

			// Parse and embed the result JSON
			var resultData interface{}
			if json.Unmarshal([]byte(task.Result), &resultData) == nil {
				result["result"] = resultData
			} else {
				result["result"] = task.Result
			}

			data, _ := json.MarshalIndent(result, "", "  ")
			return mcp.NewToolResultText(string(data)), nil

		case TaskStatusFailed:
			result := map[string]interface{}{
				"taskId": task.ID,
				"type":   task.Type,
				"status": task.Status,
				"error":  task.Error,
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			return mcp.NewToolResultError(string(data)), nil

		default:
			// Still running or pending
			result := map[string]interface{}{
				"taskId":   task.ID,
				"type":     task.Type,
				"status":   task.Status,
				"progress": task.Progress,
				"percent":  task.Percent,
				"message":  fmt.Sprintf("Task is still %s. Use get_task_status to continue polling.", task.Status),
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		}
	})
}

func registerListTasks(s *server.MCPServer, tm *TaskManager) {
	tool := mcp.NewTool("list_tasks",
		mcp.WithDescription("List all async tasks. Optionally filter by type (backtest/download) and status (pending/running/completed/failed)."),
		mcp.WithString("type", mcp.Description("Filter by task type: 'backtest' or 'download'")),
		mcp.WithString("status", mcp.Description("Filter by status: 'pending', 'running', 'completed', 'failed'")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskType := req.GetString("type", "")
		status := req.GetString("status", "")

		tasks := tm.ListTasks(taskType, status)

		type taskSummary struct {
			ID        string     `json:"id"`
			Type      string     `json:"type"`
			Status    TaskStatus `json:"status"`
			Progress  string     `json:"progress"`
			Percent   int        `json:"percent"`
			CreatedAt string     `json:"createdAt"`
			Duration  string     `json:"duration,omitempty"`
		}

		var summaries []taskSummary
		for _, t := range tasks {
			s := taskSummary{
				ID:        t.ID,
				Type:      t.Type,
				Status:    t.Status,
				Progress:  t.Progress,
				Percent:   t.Percent,
				CreatedAt: t.CreatedAt.Format("2006-01-02 15:04:05"),
			}
			if t.StartedAt != nil && t.EndedAt != nil {
				s.Duration = t.EndedAt.Sub(*t.StartedAt).String()
			}
			summaries = append(summaries, s)
		}

		result := map[string]interface{}{
			"total": len(summaries),
			"tasks": summaries,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
