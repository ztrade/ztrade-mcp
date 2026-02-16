package safenum

import "math"

const (
	// MaxAbsFloat64ForStorage keeps values within a conservative range
	// that remains valid for JSON encoding and MySQL DOUBLE columns.
	MaxAbsFloat64ForStorage = 1e308
)

// ClampFloat64ForStorage normalizes non-finite or extreme values so they can
// be safely marshaled to JSON and persisted to MySQL DOUBLE fields.
func ClampFloat64ForStorage(v float64) (float64, bool) {
	switch {
	case math.IsNaN(v):
		return 0, true
	case math.IsInf(v, 1):
		return MaxAbsFloat64ForStorage, true
	case math.IsInf(v, -1):
		return -MaxAbsFloat64ForStorage, true
	case v > MaxAbsFloat64ForStorage:
		return MaxAbsFloat64ForStorage, true
	case v < -MaxAbsFloat64ForStorage:
		return -MaxAbsFloat64ForStorage, true
	default:
		return v, false
	}
}
