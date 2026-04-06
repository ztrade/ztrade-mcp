package tools

import (
	"math"
	"reflect"
	"testing"
)

func TestResolveIndicatorSpecAlias(t *testing.T) {
	spec, err := resolveIndicatorSpec("ssma")
	if err != nil {
		t.Fatalf("resolveIndicatorSpec returned error: %v", err)
	}
	if spec.Name != "SMMA" {
		t.Fatalf("expected canonical indicator SMMA, got %s", spec.Name)
	}
}

func TestParseIndicatorParamStringDefaults(t *testing.T) {
	params, err := parseIndicatorParamString("", []int{12, 26, 9})
	if err != nil {
		t.Fatalf("parseIndicatorParamString returned error: %v", err)
	}
	if !reflect.DeepEqual(params, []int{12, 26, 9}) {
		t.Fatalf("unexpected params: %+v", params)
	}
}

func TestParseIndicatorParamStringChineseComma(t *testing.T) {
	params, err := parseIndicatorParamString("12， 26，9", nil)
	if err != nil {
		t.Fatalf("parseIndicatorParamString returned error: %v", err)
	}
	if !reflect.DeepEqual(params, []int{12, 26, 9}) {
		t.Fatalf("unexpected params: %+v", params)
	}
}

func TestValidateIndicatorParamsRange(t *testing.T) {
	spec, err := resolveIndicatorSpec("MACD")
	if err != nil {
		t.Fatalf("resolveIndicatorSpec returned error: %v", err)
	}
	if err := validateIndicatorParams(spec, []int{12, 26}); err == nil {
		t.Fatal("expected error for wrong MACD param count")
	}
}

func TestBuildIndicatorSeries(t *testing.T) {
	candles := build1mCandles(1704067200, 6)
	points, err := buildIndicatorSeries("EMA", []int{3}, candles)
	if err != nil {
		t.Fatalf("buildIndicatorSeries returned error: %v", err)
	}
	if len(points) != len(candles) {
		t.Fatalf("expected %d points, got %d", len(candles), len(points))
	}
	last := points[len(points)-1]
	v, ok := last.Values["result"]
	if !ok {
		t.Fatalf("expected result key in %+v", last.Values)
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		t.Fatalf("invalid indicator value: %f", v)
	}
}

func TestBuildIndicatorSeriesOHLCIndicator(t *testing.T) {
	candles := build1mCandles(1704067200, 6)
	points, err := buildIndicatorSeries("ATR", []int{3}, candles)
	if err != nil {
		t.Fatalf("buildIndicatorSeries returned error: %v", err)
	}
	if len(points) != len(candles) {
		t.Fatalf("expected %d points, got %d", len(candles), len(points))
	}
	last := points[len(points)-1]
	v, ok := last.Values["result"]
	if !ok {
		t.Fatalf("expected result key in %+v", last.Values)
	}
	if v <= 0 {
		t.Fatalf("expected positive ATR value, got %f", v)
	}
}
