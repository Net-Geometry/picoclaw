package api

import (
	"encoding/json"
	"net/http"

	"github.com/sipeed/picoclaw/pkg/config"
)

func (h *Handler) registerAgentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/agents", h.handleListAgents)
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
			a.Name = *req.Name
		}
		if req.Mode != nil {
			a.Mode = *req.Mode
		}
		if req.Model != nil {
			if req.Model.Primary == "" {
				a.Model = nil
			} else {
				a.Model = req.Model
			}
		}
		if req.Skills != nil {
			if len(*req.Skills) == 0 {
				a.Skills = nil
			} else {
				a.Skills = *req.Skills
			}
		}
		if req.AllowAgents != nil {
			if a.Subagents == nil {
				a.Subagents = &config.SubagentsConfig{}
			}
			a.Subagents.AllowAgents = *req.AllowAgents
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
