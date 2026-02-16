package tools

import (
	"math"
	"testing"

	"github.com/ztrade/ztrade-mcp/internal/safenum"
	"github.com/ztrade/ztrade/pkg/report"
)

func TestSanitizeBacktestMetrics(t *testing.T) {
	metrics := report.ReportResult{
		WinRate:      0.5,
		ProfitFactor: math.Inf(1),
		SortinoRatio: math.NaN(),
		CalmarRatio:  -math.Inf(1),
	}

	changed := sanitizeBacktestMetrics(&metrics)
	if len(changed) != 3 {
		t.Fatalf("expected 3 changed fields, got %d: %v", len(changed), changed)
	}
	if metrics.WinRate != 0.5 {
		t.Fatalf("winRate should be unchanged, got %v", metrics.WinRate)
	}
	if metrics.ProfitFactor != safenum.MaxAbsFloat64ForStorage {
		t.Fatalf("profitFactor not clamped, got %v", metrics.ProfitFactor)
	}
	if metrics.SortinoRatio != 0 {
		t.Fatalf("sortinoRatio should be 0 after NaN sanitize, got %v", metrics.SortinoRatio)
	}
	if metrics.CalmarRatio != -safenum.MaxAbsFloat64ForStorage {
		t.Fatalf("calmarRatio not clamped, got %v", metrics.CalmarRatio)
	}
}
