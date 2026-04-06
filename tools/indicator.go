package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	indicatorlib "github.com/ztrade/indicator"
	"github.com/ztrade/trademodel"
	"github.com/ztrade/ztrade/pkg/process/dbstore"
)

type indicatorSpec struct {
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases,omitempty"`
	Description   string   `json:"description"`
	ParamHint     string   `json:"paramHint"`
	MinParams     int      `json:"minParams"`
	MaxParams     int      `json:"maxParams"`
	DefaultParams []int    `json:"defaultParams"`
}

type indicatorPoint struct {
	Time   string             `json:"time"`
	Values map[string]float64 `json:"values"`
}

var commonIndicatorSpecs = []indicatorSpec{
	{
		Name:          "EMA",
		Description:   "Exponential Moving Average",
		ParamHint:     "len or fast,slow",
		MinParams:     1,
		MaxParams:     2,
		DefaultParams: []int{20},
	},
	{
		Name:          "SMA",
		Description:   "Simple Moving Average",
		ParamHint:     "len or fast,slow",
		MinParams:     1,
		MaxParams:     2,
		DefaultParams: []int{20},
	},
	{
		Name:          "SMMA",
		Aliases:       []string{"SSMA"},
		Description:   "Smoothed Moving Average",
		ParamHint:     "len or fast,slow",
		MinParams:     1,
		MaxParams:     2,
		DefaultParams: []int{20},
	},
	{
		Name:          "MACD",
		Description:   "Moving Average Convergence Divergence",
		ParamHint:     "fast,slow,signal",
		MinParams:     3,
		MaxParams:     3,
		DefaultParams: []int{12, 26, 9},
	},
	{
		Name:          "SMAMACD",
		Description:   "MACD with SMA signal line",
		ParamHint:     "fast,slow,signal",
		MinParams:     3,
		MaxParams:     3,
		DefaultParams: []int{12, 26, 9},
	},
	{
		Name:          "BOLL",
		Description:   "Bollinger Bands",
		ParamHint:     "len,multiplier",
		MinParams:     2,
		MaxParams:     2,
		DefaultParams: []int{20, 2},
	},
	{
		Name:          "RSI",
		Description:   "Relative Strength Index",
		ParamHint:     "len or fast,slow",
		MinParams:     1,
		MaxParams:     2,
		DefaultParams: []int{14},
	},
	{
		Name:          "STOCHRSI",
		Description:   "Stochastic RSI",
		ParamHint:     "stochWindow,rsiWindow,k,d",
		MinParams:     4,
		MaxParams:     4,
		DefaultParams: []int{14, 14, 3, 3},
	},
	{
		Name:          "ATR",
		Description:   "Average True Range",
		ParamHint:     "len",
		MinParams:     1,
		MaxParams:     1,
		DefaultParams: []int{14},
	},
	{
		Name:          "ADX",
		Description:   "Average Directional Index",
		ParamHint:     "len",
		MinParams:     1,
		MaxParams:     1,
		DefaultParams: []int{14},
	},
}

var indicatorSpecLookup = buildIndicatorSpecLookup(commonIndicatorSpecs)

func buildIndicatorSpecLookup(specs []indicatorSpec) map[string]indicatorSpec {
	lookup := make(map[string]indicatorSpec, len(specs)+1)
	for _, spec := range specs {
		lookup[spec.Name] = spec
		for _, alias := range spec.Aliases {
			lookup[strings.ToUpper(strings.TrimSpace(alias))] = spec
		}
	}
	return lookup
}

func resolveIndicatorSpec(name string) (indicatorSpec, error) {
	key := strings.ToUpper(strings.TrimSpace(name))
	if key == "" {
		return indicatorSpec{}, fmt.Errorf("indicator is required")
	}
	spec, ok := indicatorSpecLookup[key]
	if !ok {
		return indicatorSpec{}, fmt.Errorf("unsupported indicator %q, use list_indicators to view supported values", name)
	}
	return spec, nil
}

func parseIndicatorParamString(raw string, defaults []int) ([]int, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "，", ","))
	if raw == "" {
		return append([]int(nil), defaults...), nil
	}

	parts := strings.Split(raw, ",")
	params := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid indicator params %q", raw)
		}
		params = append(params, v)
	}
	if len(params) == 0 {
		return nil, fmt.Errorf("indicator params cannot be empty")
	}
	return params, nil
}

func validateIndicatorParams(spec indicatorSpec, params []int) error {
	if len(params) < spec.MinParams || len(params) > spec.MaxParams {
		if spec.MinParams == spec.MaxParams {
			return fmt.Errorf("indicator %s requires exactly %d params", spec.Name, spec.MinParams)
		}
		return fmt.Errorf("indicator %s requires %d-%d params", spec.Name, spec.MinParams, spec.MaxParams)
	}
	for _, p := range params {
		if p <= 0 {
			return fmt.Errorf("indicator %s params must be positive", spec.Name)
		}
	}
	return nil
}

func loadIndicatorCandles(db *dbstore.DBStore, exchange, symbol, binSize string, start, end time.Time, limit int) ([]*trademodel.Candle, string, error) {
	srcDur, dstDur, needMerge, err := parseKlineDurations(binSize)
	if err != nil {
		return nil, "", err
	}

	sourceBinSize := binSize
	sourceLimit := limit
	if needMerge {
		sourceBinSize = queryBaseBinSize
		sourceLimit, err = calcSourceLimit(limit, start, end, srcDur, dstDur)
		if err != nil {
			return nil, "", err
		}
	}

	tbl := db.GetKlineTbl(exchange, symbol, sourceBinSize)
	datas, err := tbl.GetDatas(start, end, sourceLimit)
	if err != nil {
		return nil, "", fmt.Errorf("query failed: %w", err)
	}

	candles := make([]*trademodel.Candle, 0, len(datas))
	for _, d := range datas {
		candle, ok := d.(*trademodel.Candle)
		if !ok {
			continue
		}
		candles = append(candles, candle)
	}

	if needMerge {
		candles, err = mergeCandles(candles, srcDur, dstDur, limit)
		if err != nil {
			return nil, "", fmt.Errorf("merge failed: %w", err)
		}
	} else if len(candles) > limit {
		candles = candles[:limit]
	}

	return candles, sourceBinSize, nil
}

func buildIndicatorSeries(name string, params []int, candles []*trademodel.Candle) ([]indicatorPoint, error) {
	ind, err := indicatorlib.NewCommonIndicator(name, params...)
	if err != nil {
		return nil, err
	}

	points := make([]indicatorPoint, 0, len(candles))
	for _, candle := range candles {
		if err := ind.Update(candle.Open, candle.High, candle.Low, candle.Close); err != nil {
			return nil, err
		}
		values := ind.Indicator()
		copied := make(map[string]float64, len(values))
		for k, v := range values {
			copied[k] = v
		}
		points = append(points, indicatorPoint{
			Time:   candle.Time().Format("2006-01-02 15:04:05"),
			Values: copied,
		})
	}

	return points, nil
}

func registerListIndicators(s *server.MCPServer) {
	tool := mcp.NewTool("list_indicators",
		mcp.WithDescription("List built-in common indicators supported by query_indicator, including parameter ranges and defaults."),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result := map[string]interface{}{
			"count":      len(commonIndicatorSpecs),
			"indicators": commonIndicatorSpecs,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerQueryIndicator(s *server.MCPServer, db *dbstore.DBStore) {
	tool := mcp.NewTool("query_indicator",
		mcp.WithDescription("Calculate common technical indicators from local database K-line data. Supports EMA/SMA/SMMA/MACD/SMAMACD/BOLL/RSI/STOCHRSI/ATR/ADX."),
		mcp.WithString("exchange", mcp.Required(), mcp.Description("Exchange name e.g. binance, okx")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Trading pair e.g. BTCUSDT")),
		mcp.WithString("indicator", mcp.Required(), mcp.Description("Indicator name. Use list_indicators to see available values.")),
		mcp.WithString("params", mcp.Description("Comma-separated integer params for the indicator. If omitted, common defaults are used.")),
		mcp.WithString("binSize", mcp.Description("K-line period 1m/5m/15m/1h/1d. Default: 1m")),
		mcp.WithString("start", mcp.Required(), mcp.Description("Start time in format 2006-01-02 15:04:05")),
		mcp.WithString("end", mcp.Required(), mcp.Description("End time in format 2006-01-02 15:04:05")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of rows to return. Default: 500, Max: 5000")),
		mcp.WithBoolean("includeCandles", mcp.Description("Whether to include candle OHLCV in the response. Default: false")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if db == nil {
			return mcp.NewToolResultError("database not initialized"), nil
		}

		exchange := req.GetString("exchange", "")
		symbol := req.GetString("symbol", "")
		indicatorName := req.GetString("indicator", "")
		paramsRaw := req.GetString("params", "")
		binSize := strings.ToLower(strings.TrimSpace(req.GetString("binSize", "")))
		startStr := req.GetString("start", "")
		endStr := req.GetString("end", "")
		limitF := req.GetFloat("limit", 0)
		includeCandles := req.GetBool("includeCandles", false)

		if binSize == "" {
			binSize = queryBaseBinSize
		}
		limit := int(limitF)
		if limit <= 0 {
			limit = queryKlineDefaultN
		}
		if limit > queryKlineMaxResult {
			limit = queryKlineMaxResult
		}

		spec, err := resolveIndicatorSpec(indicatorName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		params, err := parseIndicatorParamString(paramsRaw, spec.DefaultParams)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := validateIndicatorParams(spec, params); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		start, err := time.Parse("2006-01-02 15:04:05", startStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid start time: %s", err.Error())), nil
		}
		end, err := time.Parse("2006-01-02 15:04:05", endStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid end time: %s", err.Error())), nil
		}
		if !start.Before(end) {
			return mcp.NewToolResultError("start must be before end"), nil
		}

		candles, sourceBinSize, err := loadIndicatorCandles(db, exchange, symbol, binSize, start, end, limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		series, err := buildIndicatorSeries(spec.Name, params, candles)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("indicator calculation failed: %s", err.Error())), nil
		}

		result := map[string]interface{}{
			"exchange":      exchange,
			"symbol":        symbol,
			"binSize":       binSize,
			"sourceBinSize": sourceBinSize,
			"indicator":     spec.Name,
			"params":        params,
			"count":         len(series),
			"series":        series,
		}
		if len(series) > 0 {
			result["latest"] = series[len(series)-1]
		}
		if includeCandles {
			entries := make([]klineEntry, 0, len(candles))
			for _, candle := range candles {
				entries = append(entries, klineEntry{
					Time:   candle.Time().Format("2006-01-02 15:04:05"),
					Open:   candle.Open,
					High:   candle.High,
					Low:    candle.Low,
					Close:  candle.Close,
					Volume: candle.Volume,
				})
			}
			result["candles"] = entries
		}

		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
