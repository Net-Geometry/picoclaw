package routing

import (
	"strings"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
)

// SessionPolicy describes how a routed message should be mapped to a session.
type SessionPolicy struct {
	Dimensions    []string
	IdentityLinks map[string][]string
}

// ResolvedRoute is the result of agent routing.
type ResolvedRoute struct {
	AgentID       string
	Channel       string
	AccountID     string
	SessionPolicy SessionPolicy
	MatchedBy     string // currently always "default" until the new binding system lands
}

// RouteResolver determines which agent handles a message.
type RouteResolver struct {
	cfg *config.Config
}

// NewRouteResolver creates a new route resolver.
func NewRouteResolver(cfg *config.Config) *RouteResolver {
	return &RouteResolver{cfg: cfg}
}

// ResolveRoute determines which agent handles the message from a normalized
// inbound context and returns the session policy that should be used to
// allocate session state.
func (r *RouteResolver) ResolveRoute(inbound bus.InboundContext) ResolvedRoute {
	channel := strings.ToLower(strings.TrimSpace(inbound.Channel))
	accountID := NormalizeAccountID(inbound.Account)

	return ResolvedRoute{
		AgentID:       r.pickAgentID(r.resolveDefaultAgentID()),
		Channel:       channel,
		AccountID:     accountID,
		SessionPolicy: r.sessionPolicy(),
		MatchedBy:     "default",
	}
}

func (r *RouteResolver) pickAgentID(agentID string) string {
	trimmed := strings.TrimSpace(agentID)
	if trimmed == "" {
		return NormalizeAgentID(r.resolveDefaultAgentID())
	}
	normalized := NormalizeAgentID(trimmed)
	agents := r.cfg.Agents.List
	if len(agents) == 0 {
		return normalized
	}
	for _, a := range agents {
		if NormalizeAgentID(a.ID) == normalized {
			return normalized
		}
	}
	return NormalizeAgentID(r.resolveDefaultAgentID())
}

func (r *RouteResolver) resolveDefaultAgentID() string {
	agents := r.cfg.Agents.List
	if len(agents) == 0 {
		return DefaultAgentID
	}
	for _, a := range agents {
		if a.Default {
			id := strings.TrimSpace(a.ID)
			if id != "" {
				return NormalizeAgentID(id)
			}
		}
	}
	if id := strings.TrimSpace(agents[0].ID); id != "" {
		return NormalizeAgentID(id)
	}
	return DefaultAgentID
}

func (r *RouteResolver) sessionPolicy() SessionPolicy {
	return SessionPolicy{
		Dimensions:    normalizeSessionDimensions(r.cfg.Session.Dimensions),
		IdentityLinks: cloneIdentityLinks(r.cfg.Session.IdentityLinks),
	}
}

func normalizeSessionDimensions(dimensions []string) []string {
	if len(dimensions) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(dimensions))
	seen := make(map[string]struct{}, len(dimensions))
	for _, dimension := range dimensions {
		dimension = strings.ToLower(strings.TrimSpace(dimension))
		switch dimension {
		case "space", "chat", "topic", "sender":
		default:
			continue
		}
		if _, ok := seen[dimension]; ok {
			continue
		}
		seen[dimension] = struct{}{}
		normalized = append(normalized, dimension)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func cloneIdentityLinks(src map[string][]string) map[string][]string {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string][]string, len(src))
	for canonical, ids := range src {
		dup := make([]string, len(ids))
		copy(dup, ids)
		cloned[canonical] = dup
	}
	return cloned
}
