package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/cron"
)

func (h *Handler) registerCronRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/cron/jobs", h.handleListCronJobs)
	mux.HandleFunc("POST /api/cron/jobs", h.handleCreateCronJob)
	mux.HandleFunc("PUT /api/cron/jobs/{id}", h.handleUpdateCronJob)
	mux.HandleFunc("DELETE /api/cron/jobs/{id}", h.handleDeleteCronJob)
	mux.HandleFunc("POST /api/cron/jobs/{id}/enable", h.handleEnableCronJob)
	mux.HandleFunc("POST /api/cron/jobs/{id}/disable", h.handleDisableCronJob)
}

func (h *Handler) cronStorePath() (string, error) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg.WorkspacePath(), "cron", "jobs.json"), nil
}

func (h *Handler) handleListCronJobs(w http.ResponseWriter, r *http.Request) {
	storePath, err := h.cronStorePath()
	if err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	cs := cron.NewCronService(storePath, nil)
	jobs := cs.ListJobs(true)
	if jobs == nil {
		jobs = []cron.CronJob{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func (h *Handler) handleCreateCronJob(w http.ResponseWriter, r *http.Request) {
	storePath, err := h.cronStorePath()
	if err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var req struct {
		Name     string            `json:"name"`
		Schedule cron.CronSchedule `json:"schedule"`
		Message  string            `json:"message"`
		Channel  string            `json:"channel"`
		To       string            `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	cs := cron.NewCronService(storePath, nil)
	job, err := cs.AddJob(req.Name, req.Schedule, req.Message, req.Channel, req.To)
	if err != nil {
		http.Error(w, "failed to create job: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

func (h *Handler) handleUpdateCronJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	storePath, err := h.cronStorePath()
	if err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var job cron.CronJob
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	job.ID = id

	cs := cron.NewCronService(storePath, nil)
	if err := cs.UpdateJob(&job); err != nil {
		http.Error(w, "failed to update job: "+err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (h *Handler) handleDeleteCronJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	storePath, err := h.cronStorePath()
	if err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	cs := cron.NewCronService(storePath, nil)
	if !cs.RemoveJob(id) {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleEnableCronJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	storePath, err := h.cronStorePath()
	if err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	cs := cron.NewCronService(storePath, nil)
	job := cs.EnableJob(id, true)
	if job == nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (h *Handler) handleDisableCronJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	storePath, err := h.cronStorePath()
	if err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	cs := cron.NewCronService(storePath, nil)
	job := cs.EnableJob(id, false)
	if job == nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}
