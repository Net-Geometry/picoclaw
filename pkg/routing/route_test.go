package routing

import (
	"testing"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
)

func testConfig(agents []config.AgentConfig) *config.Config {
	return &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace: "/tmp/picoclaw-test",
				ModelName: "gpt-4",
			},
			List: agents,
		},
		Session: config.SessionConfig{
			Dimensions: []string{"sender"},
		},
	}
}

func TestResolveRoute_DefaultAgent_NoBindings(t *testing.T) {
	cfg := testConfig(nil)
	r := NewRouteResolver(cfg)

	route := r.ResolveRoute(bus.InboundContext{
		Channel:  "telegram",
		ChatType: "direct",
		SenderID: "user1",
	})

	if route.AgentID != DefaultAgentID {
		t.Errorf("AgentID = %q, want %q", route.AgentID, DefaultAgentID)
	}
	if route.MatchedBy != "default" {
		t.Errorf("MatchedBy = %q, want 'default'", route.MatchedBy)
	}
	if len(route.SessionPolicy.Dimensions) != 1 || route.SessionPolicy.Dimensions[0] != "sender" {
		t.Errorf("SessionPolicy.Dimensions = %v, want [sender]", route.SessionPolicy.Dimensions)
	}
	if route.SessionPolicy.IdentityLinks != nil {
		t.Errorf("SessionPolicy.IdentityLinks = %v, want nil", route.SessionPolicy.IdentityLinks)
	}
}

func TestResolveRoute_UsesNormalizedInboundContextFields(t *testing.T) {
	cfg := testConfig([]config.AgentConfig{{ID: "sales", Default: true}})
	r := NewRouteResolver(cfg)

	route := r.ResolveRoute(bus.InboundContext{
		Channel:  "Telegram",
		Account:  "Bot2",
		ChatType: "direct",
		SenderID: "user123",
	})

	if route.AgentID != "sales" {
		t.Errorf("AgentID = %q, want 'sales'", route.AgentID)
	}
	if route.Channel != "telegram" {
		t.Errorf("Channel = %q, want 'telegram'", route.Channel)
	}
	if route.AccountID != "bot2" {
		t.Errorf("AccountID = %q, want 'bot2'", route.AccountID)
	}
	if route.MatchedBy != "default" {
		t.Errorf("MatchedBy = %q, want 'default'", route.MatchedBy)
	}
}

func TestResolveRoute_InvalidAgentFallsToDefault(t *testing.T) {
	agents := []config.AgentConfig{
		{ID: "main", Default: true},
	}
	cfg := testConfig(agents)
	r := NewRouteResolver(cfg)

	route := r.ResolveRoute(bus.InboundContext{Channel: "telegram"})

	if route.AgentID != "main" {
		t.Errorf("AgentID = %q, want 'main' (invalid agent should fall to default)", route.AgentID)
	}
}

func TestResolveRoute_DefaultAgentSelection(t *testing.T) {
	agents := []config.AgentConfig{
		{ID: "alpha"},
		{ID: "beta", Default: true},
		{ID: "gamma"},
	}
	cfg := testConfig(agents)
	r := NewRouteResolver(cfg)

	route := r.ResolveRoute(bus.InboundContext{Channel: "cli"})

	if route.AgentID != "beta" {
		t.Errorf("AgentID = %q, want 'beta' (marked as default)", route.AgentID)
	}
}

func TestResolveRoute_NoDefaultUsesFirst(t *testing.T) {
	agents := []config.AgentConfig{
		{ID: "alpha"},
		{ID: "beta"},
	}
	cfg := testConfig(agents)
	r := NewRouteResolver(cfg)

	route := r.ResolveRoute(bus.InboundContext{Channel: "cli"})

	if route.AgentID != "alpha" {
		t.Errorf("AgentID = %q, want 'alpha' (first in list)", route.AgentID)
	}
}
