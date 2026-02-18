package store

import "testing"

func TestBacktestLogsInvalidRecordID(t *testing.T) {
	s := &Store{}
	if err := s.SaveBacktestLogs(0, []string{"a"}); err == nil {
		t.Fatalf("expected error for invalid record id")
	}
	if _, _, err := s.ListBacktestLogs(0, 0, 10); err == nil {
		t.Fatalf("expected error for invalid record id")
	}
}
