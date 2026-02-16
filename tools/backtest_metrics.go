package tools

import (
	"github.com/ztrade/ztrade-mcp/internal/safenum"
	"github.com/ztrade/ztrade/pkg/report"
)

// sanitizeBacktestMetrics clamps non-finite metrics so JSON encoding and DB
// persistence do not fail when the upstream report contains Inf/NaN values.
func sanitizeBacktestMetrics(result *report.ReportResult) []string {
	if result == nil {
		return nil
	}

	fields := []struct {
		name string
		ptr  *float64
	}{
		{name: "winRate", ptr: &result.WinRate},
		{name: "totalProfit", ptr: &result.TotalProfit},
		{name: "profitPercent", ptr: &result.ProfitPercent},
		{name: "maxDrawdown", ptr: &result.MaxDrawdown},
		{name: "maxDrawdownValue", ptr: &result.MaxDrawdownValue},
		{name: "maxLose", ptr: &result.MaxLose},
		{name: "totalFee", ptr: &result.TotalFee},
		{name: "startBalance", ptr: &result.StartBalance},
		{name: "endBalance", ptr: &result.EndBalance},
		{name: "totalReturn", ptr: &result.TotalReturn},
		{name: "annualReturn", ptr: &result.AnnualReturn},
		{name: "sharpeRatio", ptr: &result.SharpeRatio},
		{name: "sortinoRatio", ptr: &result.SortinoRatio},
		{name: "volatility", ptr: &result.Volatility},
		{name: "profitFactor", ptr: &result.ProfitFactor},
		{name: "calmarRatio", ptr: &result.CalmarRatio},
		{name: "consistencyScore", ptr: &result.ConsistencyScore},
		{name: "smoothnessScore", ptr: &result.SmoothnessScore},
		{name: "overallScore", ptr: &result.OverallScore},
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
