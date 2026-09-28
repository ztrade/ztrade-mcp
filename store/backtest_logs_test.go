package store

import (
	"fmt"
	"testing"
)

func TestBacktestLogsInvalidRecordID(t *testing.T) {
	s := &Store{}
	if err := s.SaveBacktestLogs(0, []string{"a"}); err == nil {
		t.Fatalf("expected error for invalid record id")
	}
	if _, _, err := s.ListBacktestLogs(0, 0, 10); err == nil {
		t.Fatalf("expected error for invalid record id")
	}
}

func TestSaveBacktestLogsLargeBatch(t *testing.T) {
	store := newQueryTestStore(t)

	lines := make([]string, 12000)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i+1)
	}

	if err := store.SaveBacktestLogs(99, lines); err != nil {
		t.Fatalf("save backtest logs: %v", err)
	}

	firstPage, total, err := store.ListBacktestLogs(99, 0, 10)
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if total != int64(len(lines)) {
		t.Fatalf("expected total %d, got %d", len(lines), total)
	}
	if len(firstPage) != 10 {
		t.Fatalf("expected 10 logs on first page, got %d", len(firstPage))
	}
	if firstPage[0].LineNo != 1 || firstPage[0].Content != "line-1" {
		t.Fatalf("unexpected first log: %+v", firstPage[0])
	}

	lastPage, _, err := store.ListBacktestLogs(99, len(lines)-10, 10)
	if err != nil {
		t.Fatalf("list last page: %v", err)
	}
	if len(lastPage) != 10 {
		t.Fatalf("expected 10 logs on last page, got %d", len(lastPage))
	}
	if lastPage[9].LineNo != len(lines) || lastPage[9].Content != fmt.Sprintf("line-%d", len(lines)) {
		t.Fatalf("unexpected last log: %+v", lastPage[9])
	}
}
