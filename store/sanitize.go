package store

import (
	"github.com/ztrade/ztrade-mcp/internal/safenum"
)

func sanitizeBacktestRecordForInsert(record *BacktestRecord) []string {
	if record == nil {
		return nil
	}

	fields := []struct {
		name string
		ptr  *float64
	}{
		{name: "initBalance", ptr: &record.InitBalance},
		{name: "fee", ptr: &record.Fee},
		{name: "lever", ptr: &record.Lever},
		{name: "winRate", ptr: &record.WinRate},
		{name: "totalProfit", ptr: &record.TotalProfit},
		{name: "profitPercent", ptr: &record.ProfitPercent},
		{name: "maxDrawdown", ptr: &record.MaxDrawdown},
		{name: "maxDrawdownValue", ptr: &record.MaxDrawdownValue},
		{name: "maxLose", ptr: &record.MaxLose},
		{name: "totalFee", ptr: &record.TotalFee},
		{name: "startBalance", ptr: &record.StartBalance},
		{name: "endBalance", ptr: &record.EndBalance},
		{name: "totalReturn", ptr: &record.TotalReturn},
		{name: "annualReturn", ptr: &record.AnnualReturn},
		{name: "sharpeRatio", ptr: &record.SharpeRatio},
		{name: "sortinoRatio", ptr: &record.SortinoRatio},
		{name: "volatility", ptr: &record.Volatility},
		{name: "profitFactor", ptr: &record.ProfitFactor},
		{name: "calmarRatio", ptr: &record.CalmarRatio},
		{name: "overallScore", ptr: &record.OverallScore},
	}

	changed := make([]string, 0)
	for _, field := range fields {
		if v, ok := safenum.ClampFloat64ForStorage(*field.ptr); ok {
			*field.ptr = v
			changed = append(changed, field.name)
		}
	}

	return changed
}
