package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/server"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/ztrade/exchange"
	_ "github.com/ztrade/exchange/include"
	"github.com/ztrade/ztrade/pkg/process/dbstore"

	"github.com/ztrade/ztrade-mcp/auth"
	"github.com/ztrade/ztrade-mcp/prompts"
	"github.com/ztrade/ztrade-mcp/resources"
	"github.com/ztrade/ztrade-mcp/store"
	"github.com/ztrade/ztrade-mcp/tools"
)

var (
	Version = "dev"
)

func main() {
	cfgFile := flag.String("config", "", "config file path")
	transport := flag.String("transport", "stdio", "transport mode: stdio, http")
	listen := flag.String("listen", ":8080", "listen address for http transport")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	// Load config
	cfg := loadConfig(*cfgFile)

	// Init DB
	db, err := dbstore.LoadDB(cfg)
	if err != nil {
		log.Warnf("init db failed: %s (some tools may not work)", err.Error())
	}

	// Wrap config for exchange (used by cmd/trade etc.)
	_ = exchange.WrapViper(cfg)

	// Init script store
	var scriptStore *store.Store
	scriptStore, err = store.NewStore(cfg)
	if err != nil {
		log.Warnf("init script store failed: %s (script management tools may not work)", err.Error())
	}

	// Load auth config
	authCfg := auth.LoadConfig(cfg)

	// Build server options
	serverOpts := []server.ServerOption{
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithRecovery(),
	}

	// Add auth middleware if enabled
	if authCfg.Enabled {
		serverOpts = append(serverOpts, server.WithToolHandlerMiddleware(auth.ToolAuthMiddleware(authCfg)))
	}

	// Create MCP server
	mcpServer := server.NewMCPServer("ztrade", Version, serverOpts...)

	// Register tools
	tools.RegisterAll(mcpServer, db, cfg, scriptStore)

	// Register resources
	resources.RegisterAll(mcpServer)

	// Register prompts
	prompts.RegisterAll(mcpServer)

	// Start server based on transport mode
	switch *transport {
	case "stdio":
		log.Info("Starting ztrade MCP server in stdio mode")
		if err := server.ServeStdio(mcpServer); err != nil {
			log.Fatalf("stdio server error: %s", err.Error())
		}

	case "http":
		addr := cfg.GetString("mcp.listen")
		if addr == "" {
			addr = *listen
		}

		opts := []server.StreamableHTTPOption{
			server.WithEndpointPath("/mcp"),
		}

		if authCfg.Enabled {
			opts = append(opts, server.WithHTTPContextFunc(auth.HTTPContextFunc(authCfg)))
		}

		httpServer := server.NewStreamableHTTPServer(mcpServer, opts...)

		mux := http.NewServeMux()
		mux.Handle("/mcp", httpServer)
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "ok",
				"version": Version,
			})
		})

		if authCfg.Enabled {
			log.Infof("Starting ztrade MCP server on %s with auth enabled (type: %s)", addr, authCfg.Type)
			handler := auth.HTTPMiddleware(authCfg)(mux)
			srv := &http.Server{Addr: addr, Handler: handler}
			if err := srv.ListenAndServe(); err != nil {
				log.Fatalf("http server error: %s", err.Error())
			}
		} else {
			log.Infof("Starting ztrade MCP server on %s (no auth)", addr)
			srv := &http.Server{Addr: addr, Handler: mux}
			if err := srv.ListenAndServe(); err != nil {
				log.Fatalf("http server error: %s", err.Error())
			}
		}

	default:
		log.Fatalf("unknown transport: %s", *transport)
	}
}

func loadConfig(cfgFile string) *viper.Viper {
	v := viper.New()
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			v.AddConfigPath(filepath.Join(home, ".configs"))
		}
		v.AddConfigPath("./configs")
		ex, err := os.Executable()
		if err == nil {
			exPath := filepath.Dir(ex)
			v.AddConfigPath(filepath.Join(exPath, "configs"))
		}
		v.SetConfigName("ztrade")
	}
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", v.ConfigFileUsed())
	}
	return v
}
