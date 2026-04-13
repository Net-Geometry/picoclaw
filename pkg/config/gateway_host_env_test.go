package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeGatewayHostTestConfig(t *testing.T, host string) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.json")
	raw := fmt.Sprintf(`{"version":2,"gateway":{"host":%q,"port":18790}}`, host)
	if err := os.WriteFile(configPath, []byte(raw), 0o600); err != nil {
		t.Fatalf("WriteFile(configPath): %v", err)
	}
	return configPath
}

func TestLoadConfig_GatewayHostEnvTrimmed(t *testing.T) {
	configPath := writeGatewayHostTestConfig(t, "127.0.0.1")
	t.Setenv(EnvGatewayHost, "  ::1  ")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if cfg.Gateway.Host != "::1" {
		t.Fatalf("cfg.Gateway.Host = %q, want %q", cfg.Gateway.Host, "::1")
	}
}

func TestLoadConfig_GatewayHostBlankEnvFallsBackToConfigHost(t *testing.T) {
	configPath := writeGatewayHostTestConfig(t, "  localhost  ")
	t.Setenv(EnvGatewayHost, "   ")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if cfg.Gateway.Host != "localhost" {
		t.Fatalf("cfg.Gateway.Host = %q, want %q", cfg.Gateway.Host, "localhost")
	}
}

func TestLoadConfig_GatewayHostBlankEnvAndConfigFallsBackToDefault(t *testing.T) {
	configPath := writeGatewayHostTestConfig(t, "   ")
	t.Setenv(EnvGatewayHost, "   ")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	defaultHost := strings.TrimSpace(DefaultConfig().Gateway.Host)
	if cfg.Gateway.Host != defaultHost {
		t.Fatalf("cfg.Gateway.Host = %q, want %q", cfg.Gateway.Host, defaultHost)
	}
}
