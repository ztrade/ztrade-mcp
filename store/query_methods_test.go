package store

import (
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"xorm.io/xorm"
)

func newQueryTestStore(t *testing.T) *Store {
	t.Helper()

	engine, err := xorm.NewEngine("sqlite", "file:store_query_methods?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	t.Cleanup(func() {
		_ = engine.Close()
	})

	if err := engine.Sync2(new(ScriptVersion), new(BacktestRecord), new(BacktestLog)); err != nil {
		t.Fatalf("sync schema: %v", err)
	}

	return &Store{engine: engine}
}

func TestVersionQueriesUseMappedColumns(t *testing.T) {
	store := newQueryTestStore(t)

	v1 := &ScriptVersion{ScriptID: 7, Version: 1, Content: "alpha", Message: "init"}
	v2 := &ScriptVersion{ScriptID: 7, Version: 2, Content: "beta", Message: "update"}
	if _, err := store.engine.Insert(v1, v2); err != nil {
		t.Fatalf("insert versions: %v", err)
	}

	version, err := store.GetVersion(7, 2)
	if err != nil {
		t.Fatalf("get version: %v", err)
	}
	if version.Content != "beta" {
		t.Fatalf("unexpected version content: %s", version.Content)
	}

	versions, err := store.ListVersions(7)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[0].Version != 2 || versions[1].Version != 1 {
		t.Fatalf("unexpected version order: %+v", versions)
	}
}

func TestBacktestQueriesUseMappedColumns(t *testing.T) {
	store := newQueryTestStore(t)

	now := time.Now().UTC().Round(time.Second)
	records := []BacktestRecord{
		{ScriptID: 9, ScriptVersion: 1, Exchange: "binance", Symbol: "BTCUSDT", StartTime: now.Add(-2 * time.Hour), EndTime: now.Add(-time.Hour), OverallScore: 1.2},
		{ScriptID: 9, ScriptVersion: 2, Exchange: "binance", Symbol: "BTCUSDT", StartTime: now.Add(-time.Hour), EndTime: now, OverallScore: 3.4},
	}
	if _, err := store.engine.Insert(&records); err != nil {
		t.Fatalf("insert records: %v", err)
	}

	listed, err := store.ListBacktestRecords(9, 0)
	if err != nil {
		t.Fatalf("list backtest records: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 records, got %d", len(listed))
	}

	best, err := store.GetBestBacktest(9)
	if err != nil {
		t.Fatalf("get best backtest: %v", err)
	}
	if best.ScriptVersion != 2 {
		t.Fatalf("expected best version 2, got %d", best.ScriptVersion)
	}
}

func TestBacktestLogQueriesUseMappedColumns(t *testing.T) {
	store := newQueryTestStore(t)

	logs := []BacktestLog{
		{RecordID: 11, LineNo: 2, Content: "second"},
		{RecordID: 11, LineNo: 1, Content: "first"},
	}
	if _, err := store.engine.Insert(&logs); err != nil {
		t.Fatalf("insert logs: %v", err)
	}

	listed, total, err := store.ListBacktestLogs(11, 0, 10)
	if err != nil {
		t.Fatalf("list backtest logs: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(listed))
	}
	if listed[0].LineNo != 1 || listed[1].LineNo != 2 {
		t.Fatalf("unexpected log order: %+v", listed)
	}
}
