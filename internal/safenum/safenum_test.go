package safenum

import (
	"math"
	"testing"
)

func TestClampFloat64ForStorage(t *testing.T) {
	tests := []struct {
		name    string
		in      float64
		want    float64
		changed bool
	}{
		{name: "finite value", in: 123.456, want: 123.456, changed: false},
		{name: "nan", in: math.NaN(), want: 0, changed: true},
		{name: "positive infinity", in: math.Inf(1), want: MaxAbsFloat64ForStorage, changed: true},
		{name: "negative infinity", in: math.Inf(-1), want: -MaxAbsFloat64ForStorage, changed: true},
		{name: "above limit", in: math.MaxFloat64, want: MaxAbsFloat64ForStorage, changed: true},
		{name: "below limit", in: -math.MaxFloat64, want: -MaxAbsFloat64ForStorage, changed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := ClampFloat64ForStorage(tt.in)
			if changed != tt.changed {
				t.Fatalf("changed mismatch: got %v, want %v", changed, tt.changed)
			}
			if got != tt.want {
				t.Fatalf("value mismatch: got %v, want %v", got, tt.want)
			}
		})
	}
}
