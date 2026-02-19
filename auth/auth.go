package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/spf13/viper"
)

type contextKey string

const userContextKey contextKey = "ztrade_user"

// User represents an authenticated user
type User struct {
	Name  string `json:"name"`
	Role  string `json:"role"` // "admin", "trader", "reader"
	Token string `json:"-"`
}

// TokenEntry represents a configured token
type TokenEntry struct {
	Token string `mapstructure:"token"`
	Name  string `mapstructure:"name"`
	Role  string `mapstructure:"role"`
}

// APIKeyEntry represents a configured API key
type APIKeyEntry struct {
	Key  string `mapstructure:"key"`
	Name string `mapstructure:"name"`
	Role string `mapstructure:"role"`
}

// Config holds authentication configuration
type Config struct {
	Enabled bool          `mapstructure:"enabled"`
	Type    string        `mapstructure:"type"` // "token", "apikey"
	Tokens  []TokenEntry  `mapstructure:"tokens"`
	Header  string        `mapstructure:"header"` // for apikey mode
	Keys    []APIKeyEntry `mapstructure:"keys"`

	// internal lookup maps
	tokenMap  map[string]*User
	apiKeyMap map[string]*User
}

// LoadConfig loads auth configuration from viper
func LoadConfig(cfg *viper.Viper) *Config {
	c := &Config{}
	c.Enabled = cfg.GetBool("mcp.auth.enabled")
	c.Type = cfg.GetString("mcp.auth.type")
	c.Header = cfg.GetString("mcp.auth.header")

	if c.Header == "" {
		c.Header = "X-API-Key"
	}

	// Load tokens
	var tokens []TokenEntry
	if err := cfg.UnmarshalKey("mcp.auth.tokens", &tokens); err == nil {
		c.Tokens = tokens
	}

	// Load API keys
	var keys []APIKeyEntry
	if err := cfg.UnmarshalKey("mcp.auth.keys", &keys); err == nil {
		c.Keys = keys
	}

	// Build lookup maps
	c.tokenMap = make(map[string]*User)
	for _, t := range c.Tokens {
		role := t.Role
		if role == "" {
			role = "reader"
		}
		c.tokenMap[t.Token] = &User{Name: t.Name, Role: role, Token: t.Token}
	}

	c.apiKeyMap = make(map[string]*User)
	for _, k := range c.Keys {
		role := k.Role
		if role == "" {
			role = "reader"
		}
		c.apiKeyMap[k.Key] = &User{Name: k.Name, Role: role, Token: k.Key}
	}

	return c
}

// Authenticate validates credentials from an HTTP request
func (c *Config) Authenticate(r *http.Request) *User {
	if !c.Enabled {
		return &User{Name: "anonymous", Role: "admin"}
	}

	switch c.Type {
	case "token":
		return c.authenticateToken(r)
	case "apikey":
		return c.authenticateAPIKey(r)
	default:
		return c.authenticateToken(r)
	}
}

func (c *Config) authenticateToken(r *http.Request) *User {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return nil
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if token == auth {
		return nil // no "Bearer " prefix
	}
	return c.tokenMap[token]
}

func (c *Config) authenticateAPIKey(r *http.Request) *User {
	// Try header first
	key := r.Header.Get(c.Header)
	if key == "" {
		// Try query parameter
		key = r.URL.Query().Get("api_key")
	}
	if key == "" {
		return nil
	}
	return c.apiKeyMap[key]
}

// UserFromContext extracts User from context
func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userContextKey).(*User)
	return u
}

// ContextWithUser returns a new context with user info
func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// role permission definitions
var rolePermissions = map[string]map[string]bool{
	"admin": {
		"list_data":           true,
		"query_kline":         true,
		"download_kline":      true,
		"run_backtest":        true,
		"run_python_research": true,
		"build_strategy":      true,
		"create_strategy":     true,
		"start_trade":         true,
		"stop_trade":          true,
		"trade_status":        true,
	},
	"trader": {
		"list_data":           true,
		"query_kline":         true,
		"download_kline":      true,
		"run_backtest":        true,
		"run_python_research": true,
		"build_strategy":      true,
		"create_strategy":     true,
		"start_trade":         true,
		"stop_trade":          true,
		"trade_status":        true,
	},
	"reader": {
		"list_data":           true,
		"query_kline":         true,
		"download_kline":      false,
		"run_backtest":        true,
		"run_python_research": true,
		"build_strategy":      false,
		"create_strategy":     true,
		"start_trade":         false,
		"stop_trade":          false,
		"trade_status":        true,
	},
}

// HasPermission checks if a role has permission to use a tool
func HasPermission(role, toolName string) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	allowed, ok := perms[toolName]
	if !ok {
		return true // unknown tools are allowed by default
	}
	return allowed
}
