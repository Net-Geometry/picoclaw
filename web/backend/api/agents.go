package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/routing"
)

func (h *Handler) registerAgentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/agents", h.handleListAgents)
	mux.HandleFunc("POST /api/agents", h.handleCreateAgent)
	mux.HandleFunc("GET /api/agents/{id}", h.handleGetAgent)
	mux.HandleFunc("PUT /api/agents/{id}", h.handleUpdateAgent)
}

// agentResponse is the shape returned / accepted by the frontend.
type agentResponse struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name,omitempty"`
	Mode        string                   `json:"mode,omitempty"`
	Model       *config.AgentModelConfig `json:"model,omitempty"`
	Skills      []string                 `json:"skills,omitempty"`
	AllowAgents []string                 `json:"allow_agents,omitempty"`
	IsDefault   bool                     `json:"is_default,omitempty"`
}

type agentListResponse struct {
	Agents           []agentResponse `json:"agents"`
	AvailableModels  []string        `json:"available_models"`
	AvailableSkills  []string        `json:"available_skills"`
	DefaultModelName string          `json:"default_model_name,omitempty"`
}

func (h *Handler) handleListAgents(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	agents := make([]agentResponse, 0, len(cfg.Agents.List))
	for _, a := range cfg.Agents.List {
		agents = append(agents, agentToResponse(a))
	}

	// Available model names
	modelNames := make([]string, 0, len(cfg.ModelList))
	for _, m := range cfg.ModelList {
		if m.Enabled {
			modelNames = append(modelNames, m.ModelName)
		}
	}

	// Available skills from workspace
	skillNames := listWorkspaceSkillNames(cfg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agentListResponse{
		Agents:           agents,
		AvailableModels:  modelNames,
		AvailableSkills:  skillNames,
		DefaultModelName: cfg.Agents.Defaults.ModelName,
	})
}

func (h *Handler) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for _, a := range cfg.Agents.List {
		if a.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(agentToResponse(a))
			return
		}
	}
	http.Error(w, "agent not found", http.StatusNotFound)
}

type updateAgentRequest struct {
	Name        *string                  `json:"name"`
	Mode        *string                  `json:"mode"`
	Model       *config.AgentModelConfig `json:"model"`
	Skills      *[]string                `json:"skills"`
	AllowAgents *[]string                `json:"allow_agents"`
}

type createAgentRequest struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Mode        string                   `json:"mode"`
	Model       *config.AgentModelConfig `json:"model"`
	Skills      []string                 `json:"skills"`
	AllowAgents []string                 `json:"allow_agents"`
}

func normalizeAgentMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "passive":
		return "passive"
	default:
		return "active"
	}
}

func normalizeStringList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func agentModelOrNil(m *config.AgentModelConfig) *config.AgentModelConfig {
	if m == nil {
		return nil
	}
	if strings.TrimSpace(m.Primary) == "" && len(m.Fallbacks) == 0 {
		return nil
	}
	m.Primary = strings.TrimSpace(m.Primary)
	m.Fallbacks = normalizeStringList(m.Fallbacks)
	return m
}

func (h *Handler) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var req createAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	idSeed := strings.TrimSpace(req.ID)
	if idSeed == "" {
		idSeed = strings.TrimSpace(req.Name)
	}
	if idSeed == "" {
		http.Error(w, "agent id is required", http.StatusBadRequest)
		return
	}
	id := routing.NormalizeAgentID(idSeed)

	for _, a := range cfg.Agents.List {
		if a.ID == id {
			http.Error(w, "agent already exists", http.StatusConflict)
			return
		}
	}

	agent := config.AgentConfig{
		ID:     id,
		Name:   strings.TrimSpace(req.Name),
		Mode:   normalizeAgentMode(req.Mode),
		Model:  agentModelOrNil(req.Model),
		Skills: normalizeStringList(req.Skills),
	}
	if allow := normalizeStringList(req.AllowAgents); len(allow) > 0 {
		agent.Subagents = &config.SubagentsConfig{AllowAgents: allow}
	}

	cfg.Agents.List = append(cfg.Agents.List, agent)
	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, "failed to save config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(agentToResponse(agent))
}

func (h *Handler) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var req updateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	found := false
	for i := range cfg.Agents.List {
		a := &cfg.Agents.List[i]
		if a.ID != id {
			continue
		}
		found = true
		if req.Name != nil {
			a.Name = strings.TrimSpace(*req.Name)
		}
		if req.Mode != nil {
			a.Mode = normalizeAgentMode(*req.Mode)
		}
		if req.Model != nil {
			if agentModelOrNil(req.Model) == nil {
				a.Model = nil
			} else {
				a.Model = req.Model
			}
		}
		if req.Skills != nil {
			normalized := normalizeStringList(*req.Skills)
			if len(normalized) == 0 {
				a.Skills = nil
			} else {
				a.Skills = normalized
			}
		}
		if req.AllowAgents != nil {
			if a.Subagents == nil {
				a.Subagents = &config.SubagentsConfig{}
			}
			a.Subagents.AllowAgents = normalizeStringList(*req.AllowAgents)
		}
		break
	}

	if !found {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, "failed to save config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the updated agent
	for _, a := range cfg.Agents.List {
		if a.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(agentToResponse(a))
			return
		}
	}
}

// agentToResponse converts a config.AgentConfig to the API response type.
func agentToResponse(a config.AgentConfig) agentResponse {
	r := agentResponse{
		ID:        a.ID,
		Name:      a.Name,
		Mode:      a.Mode,
		Model:     a.Model,
		Skills:    a.Skills,
		IsDefault: a.Default,
	}
	if a.Subagents != nil {
		r.AllowAgents = a.Subagents.AllowAgents
	}
	return r
}

// listWorkspaceSkillNames returns skill names from the workspace skills directory.
func listWorkspaceSkillNames(cfg *config.Config) []string {
	loader := newSkillsLoader(cfg.WorkspacePath())
	items := loader.ListSkills()
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}
