package tools

import (
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/viper"
	"github.com/ztrade/ztrade/pkg/process/dbstore"
)

// RegisterAll registers all MCP tools on the server.
func RegisterAll(s *server.MCPServer, db *dbstore.DBStore, cfg *viper.Viper) {
	registerListData(s, db)
	registerListExchanges(s, cfg)
	registerListSymbols(s, cfg)
	registerQueryKline(s, db)
	registerFetchKline(s, cfg)
	registerDownloadKline(s, db, cfg)
	registerRunBacktest(s, db)
	registerBuildStrategy(s)
	registerCreateStrategy(s)
	registerStartTrade(s, cfg)
	registerStopTrade(s)
	registerTradeStatus(s)
}
