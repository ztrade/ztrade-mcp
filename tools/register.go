package tools

import (
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/viper"
	"github.com/ztrade/ztrade-mcp/store"
	"github.com/ztrade/ztrade/pkg/process/dbstore"
)

// RegisterAll registers all MCP tools on the server.
func RegisterAll(s *server.MCPServer, db *dbstore.DBStore, cfg *viper.Viper, st *store.Store) {
	// Create shared task manager for async operations
	tm := NewTaskManager()

	registerListData(s, db)
	registerListExchanges(s, cfg)
	registerListSymbols(s, cfg)
	registerQueryKline(s, db)
	registerFetchKline(s, cfg)
	registerDownloadKline(s, db, cfg, tm)
	registerRunBacktest(s, db, tm)
	registerBuildStrategy(s)
	registerCreateStrategy(s, st)
	registerStartTrade(s, cfg)
	registerStopTrade(s)
	registerTradeStatus(s)

	// Strategy management tools
	registerGetStrategy(s, st)
	registerListStrategies(s, st)
	registerUpdateStrategy(s, st)
	registerUpdateStrategyMeta(s, st)
	registerDeleteStrategy(s, st)

	// Strategy version management
	registerListStrategyVersions(s, st)
	registerGetStrategyVersion(s, st)
	registerDiffStrategyVersions(s, st)
	registerRollbackStrategy(s, st)

	// Strategy performance tracking
	registerRunBacktestManaged(s, db, st, tm)
	registerListBacktestRecords(s, st)
	registerGetBacktestLogs(s, st)
	registerStrategyPerformance(s, st)

	// Async task management tools
	registerGetTaskStatus(s, tm)
	registerGetTaskResult(s, tm)
	registerListTasks(s, tm)
}
