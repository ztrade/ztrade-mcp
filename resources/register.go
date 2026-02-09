package resources

import (
	"github.com/mark3labs/mcp-go/server"
)

// RegisterAll registers all MCP resources on the server.
func RegisterAll(s *server.MCPServer) {
	registerStrategyDoc(s)
	registerEngineDoc(s)
}
