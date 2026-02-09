package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// HTTPMiddleware returns a curried middleware that wraps an http.Handler with authentication.
// Usage: handler := auth.HTTPMiddleware(authCfg)(mux)
func HTTPMiddleware(cfg *Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if !cfg.Enabled {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for health check endpoint
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			user := cfg.Authenticate(r)
			if user == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := ContextWithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// HTTPContextFunc returns a function compatible with mcp-go's WithHTTPContextFunc.
// It extracts the user from the request context (set by HTTPMiddleware) and
// injects it into the MCP context.
func HTTPContextFunc(cfg *Config) func(ctx context.Context, r *http.Request) context.Context {
	return func(ctx context.Context, r *http.Request) context.Context {
		// User may already be in request context from middleware
		user := UserFromContext(r.Context())
		if user != nil {
			return ContextWithUser(ctx, user)
		}
		// Try authenticating directly
		if cfg.Enabled {
			user = cfg.Authenticate(r)
			if user != nil {
				return ContextWithUser(ctx, user)
			}
		} else {
			return ContextWithUser(ctx, &User{Name: "anonymous", Role: "admin"})
		}
		return ctx
	}
}

// ToolAuthMiddleware returns an mcp-go tool middleware that checks RBAC permissions.
func ToolAuthMiddleware(cfg *Config) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			user := UserFromContext(ctx)
			if user == nil && cfg.Enabled {
				return nil, fmt.Errorf("authentication required")
			}
			if user != nil && !HasPermission(user.Role, req.Params.Name) {
				return nil, fmt.Errorf("permission denied: role '%s' cannot use tool '%s'", user.Role, req.Params.Name)
			}
			return next(ctx, req)
		}
	}
}

// ToolAuthCheck checks if the current user has permission for the given tool.
// Returns an error message if unauthorized, or empty string if allowed.
func ToolAuthCheck(ctx context.Context, toolName string) string {
	user := UserFromContext(ctx)
	if user == nil {
		return "authentication required"
	}
	if !HasPermission(user.Role, toolName) {
		return "permission denied: role '" + user.Role + "' cannot use tool '" + toolName + "'"
	}
	return ""
}
