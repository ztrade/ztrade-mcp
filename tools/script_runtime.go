package tools

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"github.com/ztrade/ztrade-mcp/store"
	"github.com/ztrade/ztrade/pkg/ctl"
)

const defaultStrategyPluginDir = "/tmp/ztrade_plugins"

var strategyCompileMu sync.Mutex

func getConfiguredModuleRoot(cfg *viper.Viper) string {
	if cfg == nil {
		return ""
	}
	return cfg.GetString("mcp.strategy.moduleRoot")
}

func getConfiguredPluginOutputDir(cfg *viper.Viper) string {
	if cfg == nil {
		return defaultStrategyPluginDir
	}
	dir := strings.TrimSpace(cfg.GetString("mcp.strategy.outputDir"))
	if dir == "" {
		return defaultStrategyPluginDir
	}
	return filepath.Clean(dir)
}

func newStrategyBuilder(script, output string, cfg *viper.Viper) *ctl.Builder {
	fmt.Print("strategy compile output directory: ", filepath.Dir(output), "\n")
	builder := ctl.NewBuilder(script, output)
	moduleRoot := getConfiguredModuleRoot(cfg)
	if moduleRoot != "" {
		builder.SetModuleRoot(moduleRoot)
	}
	return builder
}

// ensurePluginScript compiles a .go strategy into a plugin and returns the runtime path.
// Non-.go scripts are returned as-is.
func ensurePluginScript(script string, cfg *viper.Viper) (string, error) {
	if strings.ToLower(filepath.Ext(script)) != ".go" {
		return script, nil
	}

	pluginDir := getConfiguredPluginOutputDir(cfg)

	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create plugin temp dir: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(script), filepath.Ext(script))
	sum := sha1.Sum([]byte(script))
	soPath := filepath.Join(pluginDir, fmt.Sprintf("%s_%x.so", base, sum[:6]))

	builder := newStrategyBuilder(script, soPath, cfg)
	if err := builder.Build(); err != nil {
		return "", fmt.Errorf("failed to build so: %w", err)
	}

	return soPath, nil
}

func resolveScriptFromStoreInput(st *store.Store, scriptInput string) (*store.Script, bool, error) {
	if st == nil || scriptInput == "" {
		return nil, false, nil
	}
	if !(isLikelyID(scriptInput) || isLikelyName(scriptInput)) {
		return nil, false, nil
	}

	if isLikelyID(scriptInput) {
		id, _ := parseID(scriptInput)
		s, err := st.GetScript(id)
		if err != nil {
			return nil, false, fmt.Errorf("strategy not found: %w", err)
		}
		return s, true, nil
	}

	s, err := st.GetScriptByName(scriptInput)
	if err != nil {
		return nil, false, fmt.Errorf("strategy not found: %w", err)
	}
	return s, true, nil
}

func compileStoredScriptWithCache(s *store.Script, cfg *viper.Viper) (string, error) {
	if s == nil {
		return "", fmt.Errorf("script is nil")
	}
	return compileScriptContentWithCache(s.ID, s.Version, s.Content, cfg)
}

func compileScriptContentWithCache(scriptID int64, scriptVersion int, scriptContent string, cfg *viper.Viper) (string, error) {
	pluginDir := getConfiguredPluginOutputDir(cfg)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create plugin temp dir: %w", err)
	}

	cacheKey := buildStrategyCacheKey(runtime.Version(), scriptID, scriptVersion)
	goPath := filepath.Join(pluginDir, cacheKey+".go")
	soPath := filepath.Join(pluginDir, cacheKey+".so")

	strategyCompileMu.Lock()
	defer strategyCompileMu.Unlock()

	if _, err := os.Stat(soPath); err == nil {
		return soPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check cached plugin: %w", err)
	}

	if err := writeFile(goPath, scriptContent); err != nil {
		return "", fmt.Errorf("failed to write temp go file: %w", err)
	}

	builder := newStrategyBuilder(goPath, soPath, cfg)
	if err := builder.Build(); err != nil {
		return "", fmt.Errorf("build failed: %w", err)
	}

	return soPath, nil
}

func buildStrategyCacheKey(goVersion string, scriptID int64, scriptVersion int) string {
	cleanVersion := strings.NewReplacer("/", "_", " ", "_", ".", "_").Replace(goVersion)
	return fmt.Sprintf("%s_sid%d_v%d", cleanVersion, scriptID, scriptVersion)
}

func copyFile(srcPath, dstPath string) error {
	if srcPath == dstPath {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	return nil
}
