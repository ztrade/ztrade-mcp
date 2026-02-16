package tools

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ztrade/ztrade/pkg/ctl"
)

// ensurePluginScript compiles a .go strategy into a plugin and returns the runtime path.
// Non-.go scripts are returned as-is.
func ensurePluginScript(script string) (string, error) {
	if strings.ToLower(filepath.Ext(script)) != ".go" {
		return script, nil
	}

	if err := os.MkdirAll("/tmp/ztrade_plugins", 0755); err != nil {
		return "", fmt.Errorf("failed to create plugin temp dir: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(script), filepath.Ext(script))
	sum := sha1.Sum([]byte(script))
	soPath := filepath.Join("/tmp/ztrade_plugins", fmt.Sprintf("%s_%x.so", base, sum[:6]))

	builder := ctl.NewBuilder(script, soPath)
	if err := builder.Build(); err != nil {
		return "", fmt.Errorf("failed to build so: %w", err)
	}

	return soPath, nil
}
