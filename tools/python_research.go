package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/viper"
)

type pyResearchRequest struct {
	Exchange   string `json:"exchange"`
	Symbol     string `json:"symbol"`
	BinSize    string `json:"binSize"`
	Start      int64  `json:"start"`
	End        int64  `json:"end"`
	Limit      int    `json:"limit,omitempty"`
	TimeoutSec int    `json:"timeoutSec,omitempty"`
	Code       string `json:"code"`
}

type pyResearchImage struct {
	Data     string `json:"data"`
	MIMEType string `json:"mimeType"`
	Name     string `json:"name,omitempty"`
}

type pyResearchResponse struct {
	OK              bool              `json:"ok"`
	Error           string            `json:"error,omitempty"`
	Meta            map[string]any    `json:"meta,omitempty"`
	Stdout          string            `json:"stdout,omitempty"`
	StdoutTruncated bool              `json:"stdoutTruncated,omitempty"`
	Stderr          string            `json:"stderr,omitempty"`
	StderrTruncated bool              `json:"stderrTruncated,omitempty"`
	Result          any               `json:"result,omitempty"`
	Images          []pyResearchImage `json:"images,omitempty"`
}

func newPyResearchResult(resp pyResearchResponse) *mcp.CallToolResult {
	summary := map[string]any{
		"ok":              resp.OK,
		"error":           resp.Error,
		"meta":            resp.Meta,
		"stdout":          resp.Stdout,
		"stdoutTruncated": resp.StdoutTruncated,
		"stderr":          resp.Stderr,
		"stderrTruncated": resp.StderrTruncated,
		"result":          resp.Result,
	}

	content := make([]mcp.Content, 0, 1+len(resp.Images))
	imageMeta := make([]map[string]any, 0, len(resp.Images))
	for _, img := range resp.Images {
		if img.Data == "" {
			continue
		}

		mimeType := strings.TrimSpace(img.MIMEType)
		if mimeType == "" {
			mimeType = "image/png"
		}
		if !strings.HasPrefix(mimeType, "image/") {
			continue
		}

		name := strings.TrimSpace(img.Name)
		imageMeta = append(imageMeta, map[string]any{
			"name":     name,
			"mimeType": mimeType,
		})

		content = append(content, mcp.ImageContent{
			Type:     mcp.ContentTypeImage,
			Data:     img.Data,
			MIMEType: mimeType,
		})
	}
	if len(imageMeta) > 0 {
		summary["images"] = imageMeta
	}

	pretty, _ := json.MarshalIndent(summary, "", "  ")
	content = append([]mcp.Content{
		mcp.TextContent{Type: mcp.ContentTypeText, Text: string(pretty)},
	}, content...)

	return &mcp.CallToolResult{Content: content, IsError: !resp.OK}
}

func registerRunPythonResearch(s *server.MCPServer, cfg *viper.Viper) {
	tool := mcp.NewTool("run_python_research",
		mcp.WithDescription("Execute Python research code in an isolated python-runner container. The python-runner reads K-line data directly from the configured database (no large OHLCV payloads over HTTP)."),
		mcp.WithString("exchange", mcp.Required(), mcp.Description("Exchange name (e.g., binance, okx)")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Trading pair (e.g., BTCUSDT)")),
		mcp.WithString("binSize", mcp.Description("K-line period (1m/5m/15m/1h/4h/1d). Default: 1m")),
		mcp.WithString("start", mcp.Required(), mcp.Description("Start time in format 2006-01-02 15:04:05")),
		mcp.WithString("end", mcp.Required(), mcp.Description("End time in format 2006-01-02 15:04:05")),
		mcp.WithNumber("limit", mcp.Description("Optional max rows to load into pandas. Default: 0 (runner decides).")),
		mcp.WithNumber("timeoutSec", mcp.Description("Execution timeout in seconds. Default: runner config.")),
		mcp.WithString("code", mcp.Required(), mcp.Description("Python code to execute. The runner provides a pandas DataFrame df with OHLCV columns.")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url := strings.TrimSpace(cfg.GetString("pyrunner.url"))
		if url == "" {
			url = "http://python-runner:9000"
		}
		token := strings.TrimSpace(cfg.GetString("pyrunner.token"))
		clientTimeout := cfg.GetDuration("pyrunner.clientTimeout")
		if clientTimeout <= 0 {
			clientTimeout = 90 * time.Second
		}

		exchange := req.GetString("exchange", "")
		symbol := req.GetString("symbol", "")
		binSize := req.GetString("binSize", "")
		startStr := req.GetString("start", "")
		endStr := req.GetString("end", "")
		limitF := req.GetFloat("limit", 0)
		timeoutSecF := req.GetFloat("timeoutSec", 0)
		code := req.GetString("code", "")

		if binSize == "" {
			binSize = "1m"
		}
		limit := int(limitF)
		if limit < 0 {
			limit = 0
		}
		timeoutSec := int(timeoutSecF)
		if timeoutSec < 0 {
			timeoutSec = 0
		}

		start, err := time.Parse("2006-01-02 15:04:05", startStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid start time: %s", err.Error())), nil
		}
		end, err := time.Parse("2006-01-02 15:04:05", endStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid end time: %s", err.Error())), nil
		}

		payload := pyResearchRequest{
			Exchange:   exchange,
			Symbol:     symbol,
			BinSize:    binSize,
			Start:      start.Unix(),
			End:        end.Unix(),
			Limit:      limit,
			TimeoutSec: timeoutSec,
			Code:       code,
		}
		body, _ := json.Marshal(payload)

		httpClient := &http.Client{Timeout: clientTimeout}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(url, "/")+"/v1/research/run", bytes.NewReader(body))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build request: %s", err.Error())), nil
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if token != "" {
			httpReq.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := httpClient.Do(httpReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("python-runner request failed: %s", err.Error())), nil
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // cap tool output to 4MiB

		if resp.StatusCode != http.StatusOK {
			log.WithField("status", resp.StatusCode).Warn("python-runner returned non-200")
			return mcp.NewToolResultError(fmt.Sprintf("python-runner error (status=%d): %s", resp.StatusCode, string(respBody))), nil
		}

		var runResp pyResearchResponse
		if err := json.Unmarshal(respBody, &runResp); err == nil {
			return newPyResearchResult(runResp), nil
		}

		// Fallback: raw text body (should not happen in normal runner responses).
		return mcp.NewToolResultText(string(respBody)), nil
	})
}
