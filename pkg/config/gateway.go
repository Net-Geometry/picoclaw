package config

import (
	"encoding/json"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/sipeed/picoclaw/pkg/logger"
)

const DefaultGatewayLogLevel = "warn"

type GatewayConfig struct {
	Host      string `json:"host"                env:"PICOCLAW_GATEWAY_HOST"`
	Port      int    `json:"port"                env:"PICOCLAW_GATEWAY_PORT"`
	HotReload bool   `json:"hot_reload"          env:"PICOCLAW_GATEWAY_HOT_RELOAD"`
	LogLevel  string `json:"log_level,omitempty" env:"PICOCLAW_LOG_LEVEL"`
}

func canonicalGatewayLogLevel(level logger.LogLevel) string {
	switch level {
	case logger.DEBUG:
		return "debug"
	case logger.INFO:
		return "info"
	case logger.WARN:
		return "warn"
	case logger.ERROR:
		return "error"
	case logger.FATAL:
		return "fatal"
	default:
		return DefaultGatewayLogLevel
	}
}

func normalizeGatewayLogLevel(logLevel string) string {
	if level, ok := logger.ParseLevel(logLevel); ok {
		return canonicalGatewayLogLevel(level)
	}
	return DefaultGatewayLogLevel
}

// EffectiveGatewayLogLevel returns the normalized runtime log level from a loaded config.
// Invalid or empty values fall back to the package default.
func EffectiveGatewayLogLevel(cfg *Config) string {
	if cfg == nil {
		return DefaultGatewayLogLevel
	}
	return normalizeGatewayLogLevel(cfg.Gateway.LogLevel)
}

var (
	gatewayIPFamiliesOnce sync.Once
	gatewayHasIPv4        bool
	gatewayHasIPv6        bool
)

func detectGatewayIPFamilies() (bool, bool) {
	gatewayIPFamiliesOnce.Do(func() {
		if ips, err := net.LookupIP("localhost"); err == nil {
			for _, ip := range ips {
				if ip == nil {
					continue
				}
				if ip.To4() != nil {
					gatewayHasIPv4 = true
					continue
				}
				gatewayHasIPv6 = true
			}
		}

		if gatewayHasIPv4 && gatewayHasIPv6 {
			return
		}

		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, addr := range addrs {
				ipnet, ok := addr.(*net.IPNet)
				if !ok || ipnet.IP == nil {
					continue
				}
				if ipnet.IP.To4() != nil {
					gatewayHasIPv4 = true
					continue
				}
				gatewayHasIPv6 = true
			}
		}
	})

	return gatewayHasIPv4, gatewayHasIPv6
}

func selectAdaptiveGatewayLoopbackHost(hasIPv4, hasIPv6 bool) string {
	switch {
	case hasIPv4 && hasIPv6:
		return "localhost"
	case hasIPv6:
		return "::1"
	case hasIPv4:
		return "127.0.0.1"
	default:
		return "localhost"
	}
}

func selectAdaptiveGatewayAnyHost(hasIPv4, hasIPv6 bool) string {
	switch {
	case hasIPv4 && hasIPv6:
		return "::"
	case hasIPv6:
		return "::"
	case hasIPv4:
		return "0.0.0.0"
	default:
		return "::"
	}
}

func resolveAdaptiveGatewayLoopbackHost() string {
	hasIPv4, hasIPv6 := detectGatewayIPFamilies()
	return selectAdaptiveGatewayLoopbackHost(hasIPv4, hasIPv6)
}

func resolveAdaptiveGatewayAnyHost() string {
	hasIPv4, hasIPv6 := detectGatewayIPFamilies()
	return selectAdaptiveGatewayAnyHost(hasIPv4, hasIPv6)
}

func normalizeGatewayHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = strings.TrimSpace(DefaultConfig().Gateway.Host)
	}

	if host == "" {
		host = "localhost"
	}

	if strings.EqualFold(host, "localhost") {
		return resolveAdaptiveGatewayLoopbackHost()
	}

	trimmed := strings.Trim(host, "[]")
	if ip := net.ParseIP(trimmed); ip != nil && ip.IsUnspecified() {
		return resolveAdaptiveGatewayAnyHost()
	}

	return host
}

func resolveGatewayHostFromEnv(baseHost string) string {
	envHost, ok := os.LookupEnv(EnvGatewayHost)
	if !ok {
		return normalizeGatewayHost(baseHost)
	}

	envHost = strings.TrimSpace(envHost)
	if envHost == "" {
		return normalizeGatewayHost(baseHost)
	}

	return normalizeGatewayHost(envHost)
}

// ResolveGatewayLogLevel reads the configured gateway log level without triggering
// the full config loader, so startup code can apply logging before config load logs run.
// The PICOCLAW_LOG_LEVEL environment variable overrides the file value.
func ResolveGatewayLogLevel(path string) string {
	cfg := struct {
		Gateway GatewayConfig `json:"gateway"`
	}{
		Gateway: GatewayConfig{LogLevel: DefaultGatewayLogLevel},
	}

	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	if envLevel := os.Getenv("PICOCLAW_LOG_LEVEL"); envLevel != "" {
		cfg.Gateway.LogLevel = envLevel
	}

	return normalizeGatewayLogLevel(cfg.Gateway.LogLevel)
}
