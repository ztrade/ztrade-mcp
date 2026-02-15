package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"
	"os"

	"github.com/ztrade/ztrade-mcp/store"
	"github.com/ztrade/ztrade/pkg/ctl"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/viper"
	"github.com/ztrade/exchange"
	"github.com/ztrade/ztrade/pkg/ctl"
)

// tradeManager manages live trading instances
type tradeManager struct {
	mu     sync.RWMutex
	trades map[string]*tradeInstance
}

type tradeInstance struct {
	ID       string    `json:"id"`
	Exchange string    `json:"exchange"`
	Symbol   string    `json:"symbol"`
	Script   string    `json:"script"`
	Started  time.Time `json:"started"`
	trade    *ctl.Trade
}

var manager = &tradeManager{
	trades: make(map[string]*tradeInstance),
}

func registerStartTrade(s *server.MCPServer, cfg *viper.Viper) {
	tool := mcp.NewTool("start_trade",
		mcp.WithDescription("Start a live trading instance with a strategy. Requires exchange API credentials in config. Returns a trade ID for monitoring and stopping."),
		mcp.WithString("script", mcp.Required(), mcp.Description("Strategy file path (.go or .so)")),
		mcp.WithString("exchange", mcp.Required(), mcp.Description("Exchange name (e.g., binance)")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Trading pair (e.g., BTCUSDT)")),
		mcp.WithString("param", mcp.Description("Strategy parameters as JSON string")),
		mcp.WithNumber("recentDays", mcp.Description("Load recent N days of historical data. Default: 1")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Safety check
		if !cfg.GetBool("mcp.enableLiveTrade") {
			return mcp.NewToolResultError("live trading is disabled. Set mcp.enableLiveTrade: true in config to enable"), nil
		}

		script := req.GetString("script", "")
		exchangeName := req.GetString("exchange", "")
		symbol := req.GetString("symbol", "")
		param := req.GetString("param", "")
		recentDaysF := req.GetFloat("recentDays", 0)

		// --- 自动从数据库读取策略并编译为so ---
		var soPath string
		var goPath string
		st := getStoreFromContext(ctx)
		if st != nil && script != "" && (isLikelyID(script) || isLikelyName(script)) {
			var s *store.Script
			var err error
			if isLikelyID(script) {
				id, _ := parseID(script)
				s, err = st.GetScript(id)
			} else {
				s, err = st.GetScriptByName(script)
			}
			if err != nil {
				return mcp.NewToolResultError("strategy not found: " + err.Error()), nil
			}
			goPath = fmt.Sprintf("/tmp/ztrade_plugins/%s_v%d.go", s.Name, s.Version)
			soPath = fmt.Sprintf("/tmp/ztrade_plugins/%s_v%d.so", s.Name, s.Version)
			if err := writeFile(goPath, s.Content); err != nil {
				return mcp.NewToolResultError("failed to write temp go file: " + err.Error()), nil
			}
			builder := ctl.NewBuilder(goPath, soPath)
			if err := builder.Build(); err != nil {
				return mcp.NewToolResultError("build failed: " + err.Error()), nil
			}
			script = soPath
		}

		recentDays := int(recentDaysF)
		if recentDays <= 0 {
			recentDays = 1
		}

		exchangeCfg := exchange.WrapViper(cfg)
		trade, err := ctl.NewTradeWithConfig(exchangeCfg, exchangeName, symbol)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create trade: %s", err.Error())), nil
		}

		trade.SetLoadRecent(time.Duration(recentDays) * 24 * time.Hour)

		scriptName := filepath.Base(script)
		err = trade.AddScript(scriptName, script, param)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to add script: %s", err.Error())), nil
		}

		err = trade.Start()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to start trade: %s", err.Error())), nil
		}
// --- 以下为辅助函数，复用自 backtest.go ---
func getStoreFromContext(ctx context.Context) *store.Store {
	v := ctx.Value("store")
	if v == nil {
		return nil
	}
	st, ok := v.(*store.Store)
	if !ok {
		return nil
	}
	return st
}

func isLikelyID(s string) bool {
	_, err := parseID(s)
	return err == nil
}

func parseID(s string) (int64, error) {
	var id int64
	_, err := fmt.Sscanf(s, "%d", &id)
	return id, err
}

func isLikelyName(s string) bool {
	if len(s) == 0 {
		return false
	}
	if len(s) > 3 && (s[len(s)-3:] == ".go" || s[len(s)-3:] == ".so") {
		return false
	}
	if len(s) > 0 && (s[0] == '/' || s[0] == '.') {
		return false
	}
	return true
}

func writeFile(path, content string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

func writeFile(path, content string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

		tradeID := fmt.Sprintf("%s_%s_%d", exchangeName, symbol, time.Now().Unix())
		instance := &tradeInstance{
			ID:       tradeID,
			Exchange: exchangeName,
			Symbol:   symbol,
			Script:   script,
			Started:  time.Now(),
			trade:    trade,
		}

		manager.mu.Lock()
		manager.trades[tradeID] = instance
		manager.mu.Unlock()

		result := map[string]interface{}{
			"status":   "started",
			"tradeId":  tradeID,
			"exchange": exchangeName,
			"symbol":   symbol,
			"script":   script,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerStopTrade(s *server.MCPServer) {
	tool := mcp.NewTool("stop_trade",
		mcp.WithDescription("Stop a running live trading instance by its trade ID."),
		mcp.WithString("tradeId", mcp.Required(), mcp.Description("Trade instance ID returned by start_trade")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tradeID := req.GetString("tradeId", "")

		manager.mu.Lock()
		instance, ok := manager.trades[tradeID]
		if !ok {
			manager.mu.Unlock()
			return mcp.NewToolResultError(fmt.Sprintf("trade instance not found: %s", tradeID)), nil
		}
		delete(manager.trades, tradeID)
		manager.mu.Unlock()

		err := instance.trade.Stop()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to stop trade: %s", err.Error())), nil
		}

		_ = instance.trade.Wait()

		result := map[string]interface{}{
			"status":  "stopped",
			"tradeId": tradeID,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}

func registerTradeStatus(s *server.MCPServer) {
	tool := mcp.NewTool("trade_status",
		mcp.WithDescription("Get status of live trading instances. If tradeId is provided, returns status of that specific instance. Otherwise returns all running instances."),
		mcp.WithString("tradeId", mcp.Description("Optional: specific trade instance ID")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tradeID := req.GetString("tradeId", "")

		manager.mu.RLock()
		defer manager.mu.RUnlock()

		if tradeID != "" {
			instance, ok := manager.trades[tradeID]
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("trade instance not found: %s", tradeID)), nil
			}
			result := map[string]interface{}{
				"tradeId":  instance.ID,
				"exchange": instance.Exchange,
				"symbol":   instance.Symbol,
				"script":   instance.Script,
				"started":  instance.Started.Format("2006-01-02 15:04:05"),
				"running":  true,
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		}

		var instances []map[string]interface{}
		for _, inst := range manager.trades {
			instances = append(instances, map[string]interface{}{
				"tradeId":  inst.ID,
				"exchange": inst.Exchange,
				"symbol":   inst.Symbol,
				"script":   inst.Script,
				"started":  inst.Started.Format("2006-01-02 15:04:05"),
				"running":  true,
			})
		}

		result := map[string]interface{}{
			"totalInstances": len(instances),
			"instances":      instances,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	})
}
