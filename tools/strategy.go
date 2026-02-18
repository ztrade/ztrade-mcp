package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ztrade/ztrade-mcp/store"
)

const strategyTemplate = `package strategy

import (
	. "github.com/ztrade/trademodel"
)

// {{.Name}} - {{.Description}}
type {{.Name}} struct {
	engine   Engine
	position float64
{{range .Fields}}	{{.Name}} {{.Type}}
{{end}}}

func New{{.Name}}() *{{.Name}} {
	return new({{.Name}})
}

func (s *{{.Name}}) Param() (paramInfo []Param) {
	paramInfo = []Param{
{{range .Params}}		{{.ParamFunc}}("{{.Key}}", "{{.Label}}", "{{.Desc}}", {{.Default}}, &s.{{.FieldName}}),
{{end}}	}
	return
}

func (s *{{.Name}}) Init(engine Engine, params ParamData) (err error) {
	s.engine = engine
{{range .Indicators}}	engine.AddIndicator({{.Args}})
{{end}}{{range .Merges}}	engine.Merge("1m", "{{.Period}}", s.OnCandle{{.Suffix}})
{{end}}	return
}

// OnCandle is called on every 1m candle
func (s *{{.Name}}) OnCandle(candle *Candle) {
	// TODO: implement 1m candle logic
}

func (s *{{.Name}}) OnPosition(pos, price float64) {
	s.position = pos
}

func (s *{{.Name}}) OnTrade(trade *Trade) {
}

func (s *{{.Name}}) OnTradeMarket(trade *Trade) {
}

func (s *{{.Name}}) OnDepth(depth *Depth) {
}
{{range .Merges}}
// OnCandle{{.Suffix}} is called on every {{.Period}} candle
func (s *{{$.Name}}) OnCandle{{.Suffix}}(candle *Candle) {
	// TODO: implement {{.Period}} candle logic
}
{{end}}`

type strategyData struct {
	Name        string
	Description string
	Fields      []fieldData
	Params      []paramData
	Indicators  []indicatorData
	Merges      []mergeData
}

type fieldData struct {
	Name string
	Type string
}

type paramData struct {
	Key       string
	Label     string
	Desc      string
	Default   string
	FieldName string
	ParamFunc string
}

type indicatorData struct {
	Args string
}

type mergeData struct {
	Period string
	Suffix string
}

func registerCreateStrategy(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool("create_strategy",
		mcp.WithDescription("Create and save a strategy script to the database. Two modes: "+
			"1) Provide 'content' directly to save existing source code. "+
			"2) Omit 'content' to generate a code skeleton from a template with indicators and periods. "+
			"The script is saved to the database with version tracking."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Strategy name (e.g., 'EmaGoldenCross'). Used as struct name when generating from template.")),
		mcp.WithString("content", mcp.Description("Full strategy source code (Go code). If provided, saves directly without template generation.")),
		mcp.WithString("description", mcp.Description("Brief description of the strategy")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags (e.g., 'trend,ema,momentum')")),
		mcp.WithString("lifecycleStatus", mcp.Description("Lifecycle status: research, development, testing, stable. Default: research")),
		mcp.WithString("fieldDescriptions", mcp.Description("Detailed field-level descriptions. Suggested JSON object keyed by field/param name.")),
		mcp.WithString("indicators",
			mcp.Description("(Template mode only) Comma-separated indicators to include. "+
				"Format: NAME(params). Examples: EMA(9,26), MACD(12,26,9), BOLL(20,2), RSI(14), STOCHRSI(14,14,3,3)")),
		mcp.WithString("periods",
			mcp.Description("(Template mode only) Comma-separated K-line periods to merge. Examples: 5m,15m,1h")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := req.GetString("name", "")
		content := req.GetString("content", "")
		description := req.GetString("description", "")
		tags := req.GetString("tags", "")
		lifecycleStatus := req.GetString("lifecycleStatus", "")
		fieldDescriptions := req.GetString("fieldDescriptions", "")
		indicators := req.GetString("indicators", "")
		periods := req.GetString("periods", "")

		if description == "" {
			description = name + " strategy"
		}

		// Mode 1: content provided directly
		// Mode 2: generate from template
		if content == "" {
			data := strategyData{
				Name:        name,
				Description: description,
			}

			// Parse indicators
			if indicators != "" {
				for _, ind := range strings.Split(indicators, ",") {
					ind = strings.TrimSpace(ind)
					if ind == "" {
						continue
					}
					args := parseIndicator(ind)
					data.Indicators = append(data.Indicators, indicatorData{Args: args})
				}
			}

			// Parse merge periods
			if periods != "" {
				for _, p := range strings.Split(periods, ",") {
					p = strings.TrimSpace(p)
					if p == "" {
						continue
					}
					suffix := strings.ToUpper(strings.Replace(p, "m", "M", 1))
					suffix = strings.Replace(suffix, "h", "H", 1)
					suffix = strings.Replace(suffix, "d", "D", 1)
					data.Merges = append(data.Merges, mergeData{Period: p, Suffix: suffix})
				}
			}

			tmpl, err := template.New("strategy").Parse(strategyTemplate)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("template parse error: %s", err.Error())), nil
			}

			var buf bytes.Buffer
			err = tmpl.Execute(&buf, data)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("template execution error: %s", err.Error())), nil
			}
			content = buf.String()
		}

		// Save to database
		result := map[string]interface{}{
			"status": "success",
			"name":   name,
		}
		if st == nil {
			return mcp.NewToolResultError("script store not initialized (check database config)"), nil
		}
		script := &store.Script{
			Name:              name,
			Content:           content,
			Description:       description,
			Tags:              tags,
			Language:          "go",
			LifecycleStatus:   lifecycleStatus,
			FieldDescriptions: fieldDescriptions,
		}
		if err := st.CreateScript(script); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to save script: %s", err.Error())), nil
		}
		result["id"] = script.ID
		result["version"] = script.Version

		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(resultJSON)), nil
	})
}

// parseIndicator converts "EMA(9,26)" to `"EMA", 9, 26`
func parseIndicator(s string) string {
	idx := strings.Index(s, "(")
	if idx == -1 {
		return fmt.Sprintf(`"%s"`, s)
	}
	name := strings.TrimSpace(s[:idx])
	params := strings.TrimSuffix(strings.TrimSpace(s[idx+1:]), ")")
	return fmt.Sprintf(`"%s", %s`, name, params)
}
