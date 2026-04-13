package api

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/sipeed/picoclaw/pkg/config"
)

var (
	adaptiveLoopbackHostOnce sync.Once
	adaptiveLoopbackHost     string
)

func selectAdaptiveLoopbackHost(hasIPv4, hasIPv6 bool) string {
	switch {
	case hasIPv4 && hasIPv6:
		return "localhost"
	case hasIPv6:
		return "::1"
	case hasIPv4:
		return "127.0.0.1"
	default:
		return "127.0.0.1"
	}
}

func isLoopbackEquivalentHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	trimmed := strings.Trim(host, "[]")
	ip := net.ParseIP(trimmed)
	return ip != nil && ip.IsLoopback()
}

func resolveAdaptiveLoopbackHost() string {
	adaptiveLoopbackHostOnce.Do(func() {
		ips, err := net.LookupIP("localhost")
		if err != nil {
			adaptiveLoopbackHost = selectAdaptiveLoopbackHost(false, false)
			return
		}

		hasIPv4 := false
		hasIPv6 := false
		for _, ip := range ips {
			if ip == nil {
				continue
			}
			if ip.To4() != nil {
				hasIPv4 = true
				continue
			}
			hasIPv6 = true
		}

		adaptiveLoopbackHost = selectAdaptiveLoopbackHost(hasIPv4, hasIPv6)
	})
	return adaptiveLoopbackHost
}

func resolveDefaultLoopbackHost() string {
	return resolveAdaptiveLoopbackHost()
}

func resolveLocalhostLoopbackHost() string {
	return resolveAdaptiveLoopbackHost()
}

func (h *Handler) effectiveLauncherPublic() bool {
	if h.serverHostExplicit {
		// -host takes precedence over -public and launcher-config public setting.
		return false
	}

	if h.serverPublicExplicit {
		return h.serverPublic
	}

	cfg, err := h.loadLauncherConfig()
	if err == nil {
		return cfg.Public
	}

	return h.serverPublic
}

func canonicalLauncherBindHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return resolveDefaultLoopbackHost()
	}
	if strings.EqualFold(host, "localhost") {
		return resolveLocalhostLoopbackHost()
	}
	return host
}

func (h *Handler) launcherAndGatewayBindHostsAligned(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}

	// With -host specified, -public is ignored, so launcher's legacy bind host is loopback.
	launcherHost := canonicalLauncherBindHost("127.0.0.1")
	gatewayHost := canonicalLauncherBindHost(cfg.Gateway.Host)
	if isLoopbackEquivalentHost(launcherHost) && isLoopbackEquivalentHost(gatewayHost) {
		return true
	}

	return launcherHost == gatewayHost
}

func (h *Handler) gatewayHostOverrideForConfig(cfg *config.Config) string {
	if h.serverHostExplicit {
		if h.launcherAndGatewayBindHostsAligned(cfg) {
			return strings.TrimSpace(h.serverHost)
		}
		return ""
	}

	if h.effectiveLauncherPublic() {
		return "0.0.0.0"
	}
	return ""
}

func (h *Handler) gatewayHostOverride() string {
	if !h.serverHostExplicit {
		return h.gatewayHostOverrideForConfig(nil)
	}

	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		return ""
	}
	return h.gatewayHostOverrideForConfig(cfg)
}

func (h *Handler) effectiveGatewayBindHost(cfg *config.Config) string {
	if override := h.gatewayHostOverrideForConfig(cfg); override != "" {
		return override
	}
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Gateway.Host)
}

func gatewayProbeHost(bindHost string) string {
	bindHost = strings.TrimSpace(bindHost)
	if bindHost == "" {
		return resolveDefaultLoopbackHost()
	}
	if strings.EqualFold(bindHost, "localhost") {
		return resolveLocalhostLoopbackHost()
	}

	trimmed := strings.Trim(bindHost, "[]")
	if ip := net.ParseIP(trimmed); ip != nil && ip.IsUnspecified() {
		if ip.To4() == nil {
			return "::1"
		}
		return "127.0.0.1"
	}
	return bindHost
}

func (h *Handler) gatewayProxyURL() *url.URL {
	cfg, err := config.LoadConfig(h.configPath)
	port := 18790
	bindHost := ""
	if err == nil && cfg != nil {
		if cfg.Gateway.Port != 0 {
			port = cfg.Gateway.Port
		}
		bindHost = h.effectiveGatewayBindHost(cfg)
	}

	return &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(gatewayProbeHost(bindHost), strconv.Itoa(port)),
	}
}

func requestHostName(r *http.Request) string {
	reqHost, _, err := net.SplitHostPort(r.Host)
	if err == nil {
		return reqHost
	}
	if strings.TrimSpace(r.Host) != "" {
		return r.Host
	}
	return "127.0.0.1"
}

func requestWSScheme(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		proto := strings.ToLower(strings.TrimSpace(strings.Split(forwarded, ",")[0]))
		if proto == "https" || proto == "wss" {
			return "wss"
		}
		if proto == "http" || proto == "ws" {
			return "ws"
		}
	}

	if r.TLS != nil {
		return "wss"
	}

	return "ws"
}

// requestHTTPScheme returns http or https for URLs that are not WebSockets (e.g. SSE).
func requestHTTPScheme(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		proto := strings.ToLower(strings.TrimSpace(strings.Split(forwarded, ",")[0]))
		if proto == "https" || proto == "wss" {
			return "https"
		}
		if proto == "http" || proto == "ws" {
			return "http"
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// forwardedHostFirst returns the client-visible host from reverse-proxy / tunnel headers
// (e.g. VS Code port forwarding, nginx). Empty if unset.
func forwardedHostFirst(r *http.Request) string {
	raw := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if raw == "" {
		raw = forwardedRFC7239Host(r)
	}
	if raw == "" {
		return ""
	}
	if i := strings.IndexByte(raw, ','); i >= 0 {
		raw = strings.TrimSpace(raw[:i])
	}
	return raw
}

// forwardedRFC7239Host parses host= from the first Forwarded header element (RFC 7239).
func forwardedRFC7239Host(r *http.Request) string {
	v := strings.TrimSpace(r.Header.Get("Forwarded"))
	if v == "" {
		return ""
	}
	first := strings.TrimSpace(strings.Split(v, ",")[0])
	for _, part := range strings.Split(first, ";") {
		part = strings.TrimSpace(part)
		low := strings.ToLower(part)
		if !strings.HasPrefix(low, "host=") {
			continue
		}
		val := strings.TrimSpace(part[strings.IndexByte(part, '=')+1:])
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		return val
	}
	return ""
}

// forwardedPortFirst returns the first X-Forwarded-Port value, or empty.
func forwardedPortFirst(r *http.Request) string {
	raw := strings.TrimSpace(r.Header.Get("X-Forwarded-Port"))
	if raw == "" {
		return ""
	}
	if i := strings.IndexByte(raw, ','); i >= 0 {
		raw = strings.TrimSpace(raw[:i])
	}
	return raw
}

// clientVisiblePort picks the TCP port the browser uses to reach this app (after proxies).
// Used by picoWebUIAddr → buildWsURL / buildPicoEventsURL / buildPicoSendURL so WebSocket and
// HTTP URLs match the dashboard page origin (cookies / token flow behind tunnels and reverse proxies).
func clientVisiblePort(r *http.Request, serverListenPort int) string {
	if p := forwardedPortFirst(r); p != "" {
		return p
	}
	if _, port, err := net.SplitHostPort(r.Host); err == nil && port != "" {
		return port
	}
	if requestHTTPScheme(r) == "https" {
		return "443"
	}
	return strconv.Itoa(serverListenPort)
}

// joinClientVisibleHostPort builds host:port for absolute URLs returned to the browser.
func joinClientVisibleHostPort(r *http.Request, host string, serverListenPort int) string {
	if h, p, err := net.SplitHostPort(host); err == nil {
		return net.JoinHostPort(h, p)
	}
	return net.JoinHostPort(host, clientVisiblePort(r, serverListenPort))
}

// picoWebUIAddr is host:port for URLs returned to the browser (/pico/ws, /pico/events, /pico/send).
// It must match the HTTP Host the client used (or X-Forwarded-*), not cfg.Gateway.Host — otherwise
// e.g. page on localhost with ws_url 127.0.0.1 omits cookies and the dashboard auth handshake fails.
func (h *Handler) picoWebUIAddr(r *http.Request) string {
	wsPort := h.serverPort
	if wsPort == 0 {
		wsPort = 18800
	}
	if fwdHost := forwardedHostFirst(r); fwdHost != "" {
		return joinClientVisibleHostPort(r, fwdHost, wsPort)
	}
	host := requestHostName(r)
	// Use clientVisiblePort only when an explicit port is present in headers
	// or Host header — do not infer from TLS/scheme, as serverPort takes priority.
	if p := forwardedPortFirst(r); p != "" {
		return net.JoinHostPort(host, p)
	}
	if _, port, err := net.SplitHostPort(r.Host); err == nil && port != "" {
		return net.JoinHostPort(host, port)
	}
	return net.JoinHostPort(host, strconv.Itoa(wsPort))
}

func (h *Handler) buildWsURL(r *http.Request) string {
	return requestWSScheme(r) + "://" + h.picoWebUIAddr(r) + "/pico/ws"
}

func (h *Handler) buildPicoEventsURL(r *http.Request) string {
	return requestHTTPScheme(r) + "://" + h.picoWebUIAddr(r) + "/pico/events"
}

func (h *Handler) buildPicoSendURL(r *http.Request) string {
	return requestHTTPScheme(r) + "://" + h.picoWebUIAddr(r) + "/pico/send"
}
