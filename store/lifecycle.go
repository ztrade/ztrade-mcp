package store

const (
	StrategyLifecycleResearch    = "research"
	StrategyLifecycleDevelopment = "development"
	StrategyLifecycleTesting     = "testing"
	StrategyLifecycleStable      = "stable"
)

func IsValidStrategyLifecycleStatus(status string) bool {
	switch status {
	case StrategyLifecycleResearch, StrategyLifecycleDevelopment, StrategyLifecycleTesting, StrategyLifecycleStable:
		return true
	default:
		return false
	}
}

func IsStrategyLockedForEdit(status string) bool {
	return status == StrategyLifecycleStable
}
