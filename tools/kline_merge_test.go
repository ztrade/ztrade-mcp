package tools

import (
	"testing"
	"time"

	"github.com/ztrade/trademodel"
)

func build1mCandles(start int64, n int) []*trademodel.Candle {
	candles := make([]*trademodel.Candle, 0, n)
	for i := 0; i < n; i++ {
		op := 100.0 + float64(i)
		candles = append(candles, &trademodel.Candle{
			Start:  start + int64(i*60),
			Open:   op,
			High:   op + 2,
			Low:    op - 1,
			Close:  op + 1,
			Volume: float64(i + 1),
		})
	}
	return candles
}

func TestMergeCandles5m(t *testing.T) {
	candles := build1mCandles(1704067200, 10) // 2024-01-01 00:00:00 UTC
	merged, err := mergeCandles(candles, time.Minute, 5*time.Minute, 10)
	if err != nil {
		t.Fatalf("mergeCandles returned error: %v", err)
	}
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged candles, got %d", len(merged))
	}

	first := merged[0]
	if first.Start != 1704067200 {
		t.Fatalf("unexpected first start: %d", first.Start)
	}
	if first.Open != 100 || first.Close != 105 {
		t.Fatalf("unexpected first OHLC: open=%f close=%f", first.Open, first.Close)
	}
	if first.High != 106 || first.Low != 99 {
		t.Fatalf("unexpected first high/low: high=%f low=%f", first.High, first.Low)
	}
	if first.Volume != 15 {
		t.Fatalf("unexpected first volume: %f", first.Volume)
	}

	second := merged[1]
	if second.Start != 1704067500 {
		t.Fatalf("unexpected second start: %d", second.Start)
	}
	if second.Open != 105 || second.Close != 110 {
		t.Fatalf("unexpected second OHLC: open=%f close=%f", second.Open, second.Close)
	}
	if second.High != 111 || second.Low != 104 {
		t.Fatalf("unexpected second high/low: high=%f low=%f", second.High, second.Low)
	}
	if second.Volume != 40 {
		t.Fatalf("unexpected second volume: %f", second.Volume)
	}
}

func TestCalcSourceLimitWindowClamp(t *testing.T) {
	start := time.Unix(1704067200, 0)
	end := start.Add(30 * time.Minute)
	limit, err := calcSourceLimit(500, start, end, time.Minute, 5*time.Minute)
	if err != nil {
		t.Fatalf("calcSourceLimit returned error: %v", err)
	}
	if limit != 31 {
		t.Fatalf("expected source limit 31, got %d", limit)
	}
}

func TestParseKlineDurationsRejectSubMinute(t *testing.T) {
	_, _, _, err := parseKlineDurations("30s")
	if err == nil {
		t.Fatal("expected error for binSize smaller than 1m")
	}
}
