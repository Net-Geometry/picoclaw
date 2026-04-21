package agent

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/routing"
	"github.com/sipeed/picoclaw/pkg/session"
)

func manualSelectionScopeKey(inbound *bus.InboundContext, sessionKey string) string {
	sessionKey = strings.TrimSpace(sessionKey)
	if session.IsExplicitSessionKey(sessionKey) {
		return "session:" + sessionKey
	}
	if inbound == nil {
		return ""
	}
	parts := []string{
		strings.ToLower(strings.TrimSpace(inbound.Channel)),
		strings.ToLower(strings.TrimSpace(inbound.Account)),
		strings.ToLower(strings.TrimSpace(inbound.SpaceID)),
		strings.ToLower(strings.TrimSpace(inbound.ChatID)),
		strings.ToLower(strings.TrimSpace(inbound.TopicID)),
		strings.ToLower(strings.TrimSpace(inbound.SenderID)),
	}
	joined := strings.Join(parts, "|")
	if strings.Trim(joined, "|") == "" {
		return ""
	}
	return "inbound:" + joined
}

func (al *AgentLoop) setManualAgent(scopeKey, agentID string) {
	scopeKey = strings.TrimSpace(scopeKey)
	agentID = routing.NormalizeAgentID(agentID)
	if scopeKey == "" {
		return
	}
	if agentID == "" {
		al.manualAgents.Delete(scopeKey)
		return
	}
	al.manualAgents.Store(scopeKey, agentID)
}

func (al *AgentLoop) getManualAgent(scopeKey string) (string, bool) {
	scopeKey = strings.TrimSpace(scopeKey)
	if scopeKey == "" {
		return "", false
	}
	v, ok := al.manualAgents.Load(scopeKey)
	if !ok {
		return "", false
	}
	agentID, ok := v.(string)
	if !ok || strings.TrimSpace(agentID) == "" {
		return "", false
	}
	return routing.NormalizeAgentID(agentID), true
}

func (al *AgentLoop) setManualProject(scopeKey, project string) {
	scopeKey = strings.TrimSpace(scopeKey)
	project = strings.TrimSpace(project)
	if scopeKey == "" {
		return
	}
	if project == "" {
		al.manualProjects.Delete(scopeKey)
		return
	}
	al.manualProjects.Store(scopeKey, project)
}

func (al *AgentLoop) getManualProject(scopeKey string) (string, bool) {
	scopeKey = strings.TrimSpace(scopeKey)
	if scopeKey == "" {
		return "", false
	}
	v, ok := al.manualProjects.Load(scopeKey)
	if !ok {
		return "", false
	}
	project, ok := v.(string)
	if !ok || strings.TrimSpace(project) == "" {
		return "", false
	}
	return strings.TrimSpace(project), true
}

func normalizeProjectName(value string) (string, bool) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", false
	}
	lower := strings.ToLower(name)
	if lower == "none" || lower == "clear" || lower == "off" {
		return "", true
	}
	if strings.ContainsAny(name, `/\\`) || strings.Contains(name, "..") {
		return "", false
	}
	cleaned := filepath.Clean(name)
	if cleaned == "." || cleaned == ".." {
		return "", false
	}
	return cleaned, true
}

func applyProjectContext(userMessage, workspace, project string) string {
	project = strings.TrimSpace(project)
	if project == "" {
		return userMessage
	}
	projectRoot := filepath.Join(workspace, "projects", project)
	return fmt.Sprintf("[PROJECT CONTEXT]\nActive project: %s\nProject root: %s\nUse this project as the default scope unless the user asks otherwise.\n\n%s", project, projectRoot, userMessage)
}
