package store

import "testing"

func TestLifecycleHelpers(t *testing.T) {
	valid := []string{StrategyLifecycleResearch, StrategyLifecycleDevelopment, StrategyLifecycleTesting, StrategyLifecycleStable}
	for _, s := range valid {
		if !IsValidStrategyLifecycleStatus(s) {
			t.Fatalf("expected %s to be valid", s)
		}
	}
	if IsValidStrategyLifecycleStatus("") {
		t.Fatalf("empty status should be invalid")
	}
	if IsValidStrategyLifecycleStatus("unknown") {
		t.Fatalf("unknown status should be invalid")
	}
	if !IsStrategyLockedForEdit(StrategyLifecycleStable) {
		t.Fatalf("stable should be locked")
	}
	if IsStrategyLockedForEdit(StrategyLifecycleResearch) {
		t.Fatalf("research should not be locked")
	}
}
