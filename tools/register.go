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
	registerCreateStrategy(s)
	registerStartTrade(s, cfg)
	registerStopTrade(s)
	registerTradeStatus(s)

	// Script management tools
	registerSaveScript(s, st)
	registerGetScript(s, st)
	registerListScripts(s, st)
	registerUpdateScript(s, st)
	registerUpdateScriptMeta(s, st)
	registerDeleteScript(s, st)

	// Script version management
	registerListScriptVersions(s, st)
	registerGetScriptVersion(s, st)
	registerDiffScriptVersions(s, st)
	registerRollbackScript(s, st)

	// Script performance tracking
	registerRunBacktestManaged(s, db, st, tm)
	registerListBacktestRecords(s, st)
	registerScriptPerformance(s, st)

	// Async task management tools
	registerGetTaskStatus(s, tm)
	registerGetTaskResult(s, tm)
	registerListTasks(s, tm)
}
