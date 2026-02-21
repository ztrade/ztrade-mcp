package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	basecommon "github.com/ztrade/base/common"
	"github.com/ztrade/trademodel"
	"github.com/ztrade/ztrade/pkg/process/dbstore"
)

const (
	queryBaseBinSize    = "1m"
	queryKlineDefaultN  = 500
	queryKlineMaxResult = 5000
)

type klineEntry struct {
	Time   string  `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

func parseKlineDurations(binSize string) (srcDur, dstDur time.Duration, needMerge bool, err error) {
	srcDur, err = basecommon.GetBinSizeDuration(queryBaseBinSize)
	if err != nil {
		return 0, 0, false, fmt.Errorf("invalid base binSize %q: %w", queryBaseBinSize, err)
	}
	dstDur, err = basecommon.GetBinSizeDuration(binSize)
	if err != nil {
		return 0, 0, false, fmt.Errorf("invalid binSize %q: %w", binSize, err)
	}
	if dstDur < srcDur {
		return 0, 0, false, fmt.Errorf("binSize %q is smaller than %s", binSize, queryBaseBinSize)
	}
	if dstDur%srcDur != 0 {
		return 0, 0, false, fmt.Errorf("binSize %q is not a multiple of %s", binSize, queryBaseBinSize)
	}
	return srcDur, dstDur, dstDur != srcDur, nil
}

func calcSourceLimit(limit int, start, end time.Time, srcDur, dstDur time.Duration) (int, error) {
	if !start.Before(end) {
		return 0, fmt.Errorf("start must be before end")
	}
	if limit <= 0 {
		return 0, fmt.Errorf("limit must be greater than 0")
	}

	ratio := int64(dstDur / srcDur)
	if ratio <= 0 {
		return 0, fmt.Errorf("invalid merge ratio")
	}

	needed := int64(limit)*ratio + ratio

	window := end.Sub(start)
	windowRows := int64(window / srcDur)
	if window%srcDur != 0 {
		windowRows++
	}
	windowRows++
	if windowRows > 0 && needed > windowRows {
		needed = windowRows
	}
	if needed <= 0 {
		needed = 1
	}
	if needed > int64(math.MaxInt) {
		return 0, fmt.Errorf("requested source limit too large")
	}
	return int(needed), nil
}

func mergeCandles(candles []*trademodel.Candle, srcDur, dstDur time.Duration, limit int) ([]*trademodel.Candle, error) {
	if limit <= 0 {
		return []*trademodel.Candle{}, nil
	}
	if dstDur == srcDur {
		if len(candles) <= limit {
			return candles, nil
		}
		return candles[:limit], nil
	}
	if dstDur < srcDur || dstDur%srcDur != 0 {
		return nil, fmt.Errorf("invalid merge duration: src=%s dst=%s", srcDur, dstDur)
	}

	merger := basecommon.NewKlineMerge(srcDur, dstDur)
	merged := make([]*trademodel.Candle, 0, minInt(limit, len(candles)))
	for _, candle := range candles {
		ret := merger.Update(candle)
		if ret == nil {
			continue
		}
		mergedCandle, ok := ret.(*trademodel.Candle)
		if !ok {
			continue
		}
		merged = append(merged, mergedCandle)
		if len(merged) >= limit {
			break
		}
	}
	return merged, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func registerQueryKline(s *server.MCPServer, db *dbstore.DBStore) {
	tool := mcp.NewTool("query_kline",
		mcp.WithDescription("Query K-line candlestick data from local database for analysis. If binSize is larger than 1m, data is auto-merged from 1m candles."),
		mcp.WithString("exchange", mcp.Required(), mcp.Description("Exchange name e.g. binance, okx")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Trading pair e.g. BTCUSDT")),
		mcp.WithString("binSize", mcp.Description("K-line period 1m/5m/15m/1h/1d. Default: 1m")),
		mcp.WithString("start", mcp.Required(), mcp.Description("Start time in format 2006-01-02 15:04:05")),
		mcp.WithString("end", mcp.Required(), mcp.Description("End time in format 2006-01-02 15:04:05")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of candles to return. Default: 500, Max: 5000")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if db == nil {
			return mcp.NewToolResultError("database not initialized"), nil
		}

		exchange := req.GetString("exchange", "")
		symbol := req.GetString("symbol", "")
		binSize := strings.ToLower(strings.TrimSpace(req.GetString("binSize", "")))
		startStr := req.GetString("start", "")
		endStr := req.GetString("end", "")
		limitF := req.GetFloat("limit", 0)

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

		srcDur, dstDur, needMerge, err := parseKlineDurations(binSize)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sourceBinSize := binSize
		sourceLimit := limit
		if needMerge {
			sourceBinSize = queryBaseBinSize
			sourceLimit, err = calcSourceLimit(limit, start, end, srcDur, dstDur)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		tbl := db.GetKlineTbl(exchange, symbol, sourceBinSize)
		datas, err := tbl.GetDatas(start, end, sourceLimit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query failed: %s", err.Error())), nil
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
				return mcp.NewToolResultError(fmt.Sprintf("merge failed: %s", err.Error())), nil
			}
		} else if len(candles) > limit {
			candles = candles[:limit]
		}

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

		result := map[string]interface{}{
			"exchange":      exchange,
			"symbol":        symbol,
			"binSize":       binSize,
			"sourceBinSize": sourceBinSize,
			"count":         len(entries),
			"candles":       entries,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
