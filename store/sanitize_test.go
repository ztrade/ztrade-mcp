package store

import (
	"math"
	"testing"

	"github.com/ztrade/ztrade-mcp/internal/safenum"
)

func TestSanitizeBacktestRecordForInsert(t *testing.T) {
	rec := &BacktestRecord{
		WinRate:      0.45,
		ProfitFactor: math.Inf(1),
		SharpeRatio:  math.NaN(),
		CalmarRatio:  -math.Inf(1),
	}

	changed := sanitizeBacktestRecordForInsert(rec)
	if len(changed) != 3 {
		t.Fatalf("expected 3 changed fields, got %d: %v", len(changed), changed)
	}
	if rec.WinRate != 0.45 {
		t.Fatalf("unexpected winRate change: %v", rec.WinRate)
	}
	if rec.ProfitFactor != safenum.MaxAbsFloat64ForStorage {
		t.Fatalf("profitFactor not clamped: %v", rec.ProfitFactor)
	}
	if rec.SharpeRatio != 0 {
		t.Fatalf("sharpeRatio should be 0 after NaN sanitize: %v", rec.SharpeRatio)
	}
	if rec.CalmarRatio != -safenum.MaxAbsFloat64ForStorage {
		t.Fatalf("calmarRatio not clamped: %v", rec.CalmarRatio)
	}
}
