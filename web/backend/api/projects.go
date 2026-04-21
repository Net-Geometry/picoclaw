package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/sipeed/picoclaw/pkg/config"
)

type projectItem struct {
	Name string `json:"name"`
}

type projectListResponse struct {
	Workspace string        `json:"workspace"`
	Root      string        `json:"root"`
	Projects  []projectItem `json:"projects"`
}

type projectEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Type       string `json:"type"`
	Size       int64  `json:"size,omitempty"`
	ModifiedMs int64  `json:"modified_ms"`
}

type projectEntriesResponse struct {
	Project string         `json:"project"`
	Path    string         `json:"path"`
	Entries []projectEntry `json:"entries"`
}

type projectFileResponse struct {
	Project  string `json:"project"`
	Path     string `json:"path"`
	Content  string `json:"content"`
	Size     int64  `json:"size"`
	Editable bool   `json:"editable"`
}

type updateProjectFileRequest struct {
	Content string `json:"content"`
}

const maxEditableProjectFileSize = int64(2 * 1024 * 1024) // 2MB

func (h *Handler) registerProjectRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/projects", h.handleListProjects)
	mux.HandleFunc("GET /api/projects/{name}/entries", h.handleListProjectEntries)
	mux.HandleFunc("GET /api/projects/{name}/file", h.handleGetProjectFile)
	mux.HandleFunc("PUT /api/projects/{name}/file", h.handleUpdateProjectFile)
}

func (h *Handler) projectsRoot() (string, string, error) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		return "", "", err
	}
	workspace := cfg.WorkspacePath()
	if workspace == "" {
		return "", "", os.ErrNotExist
	}
	return workspace, filepath.Join(workspace, "projects"), nil
}

func (h *Handler) handleListProjects(w http.ResponseWriter, r *http.Request) {
	workspace, root, err := h.projectsRoot()
	if err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(projectListResponse{
				Workspace: workspace,
				Root:      root,
				Projects:  []projectItem{},
			})
			return
		}
		http.Error(w, "failed to read projects directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	projects := make([]projectItem, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		projects = append(projects, projectItem{Name: name})
	}

	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(projectListResponse{
		Workspace: workspace,
		Root:      root,
		Projects:  projects,
	})
}

func safeRelPath(raw string) (string, error) {
	rel := strings.TrimSpace(raw)
	if rel == "" || rel == "." {
		return ".", nil
	}
	clean := filepath.Clean(rel)
	if clean == "." {
		return ".", nil
	}
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", os.ErrPermission
	}
	return clean, nil
}

func (h *Handler) resolveProjectFile(r *http.Request) (string, string, string, error) {
	_, root, err := h.projectsRoot()
	if err != nil {
		return "", "", "", err
	}

	projectName := strings.TrimSpace(r.PathValue("name"))
	if projectName == "" || strings.Contains(projectName, "/") || strings.Contains(projectName, "\\") {
		return "", "", "", os.ErrInvalid
	}

	relPath, err := safeRelPath(r.URL.Query().Get("path"))
	if err != nil || relPath == "." {
		return "", "", "", os.ErrPermission
	}

	projectRoot := filepath.Join(root, projectName)
	target := filepath.Join(projectRoot, relPath)
	return projectName, relPath, target, nil
}

func isTextContent(content []byte) bool {
	if len(content) == 0 {
		return true
	}
	if !utf8.Valid(content) {
		return false
	}
	if bytes.Contains(content, []byte{0}) {
		return false
	}
	return true
}

func (h *Handler) handleListProjectEntries(w http.ResponseWriter, r *http.Request) {
	_, root, err := h.projectsRoot()
	if err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	projectName := strings.TrimSpace(r.PathValue("name"))
	if projectName == "" || strings.Contains(projectName, "/") || strings.Contains(projectName, "\\") {
		http.Error(w, "invalid project name", http.StatusBadRequest)
		return
	}

	relPath, err := safeRelPath(r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	projectRoot := filepath.Join(root, projectName)
	target := filepath.Join(projectRoot, relPath)

	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "path not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to stat path: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !info.IsDir() {
		http.Error(w, "path is not a directory", http.StatusBadRequest)
		return
	}

	items, err := os.ReadDir(target)
	if err != nil {
		http.Error(w, "failed to read directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]projectEntry, 0, len(items))
	for _, item := range items {
		itemInfo, itemErr := item.Info()
		if itemErr != nil {
			continue
		}

		childRel := item.Name()
		if relPath != "." {
			childRel = filepath.Join(relPath, item.Name())
		}
		childRel = filepath.ToSlash(childRel)

		entryType := "file"
		size := itemInfo.Size()
		if item.IsDir() {
			entryType = "dir"
			size = 0
		}
		out = append(out, projectEntry{
			Name:       item.Name(),
			Path:       childRel,
			Type:       entryType,
			Size:       size,
			ModifiedMs: itemInfo.ModTime().UnixMilli(),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type == "dir"
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})

	currentPath := relPath
	if currentPath == "." {
		currentPath = ""
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(projectEntriesResponse{
		Project: projectName,
		Path:    currentPath,
		Entries: out,
	})
}

func (h *Handler) handleGetProjectFile(w http.ResponseWriter, r *http.Request) {
	projectName, relPath, target, err := h.resolveProjectFile(r)
	if err != nil {
		http.Error(w, "invalid project or file path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to stat file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		http.Error(w, "path is a directory", http.StatusBadRequest)
		return
	}
	if info.Size() > maxEditableProjectFileSize {
		http.Error(w, "file too large to edit in web UI", http.StatusBadRequest)
		return
	}

	content, err := os.ReadFile(target)
	if err != nil {
		http.Error(w, "failed to read file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !isTextContent(content) {
		http.Error(w, "file is not UTF-8 text", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(projectFileResponse{
		Project:  projectName,
		Path:     filepath.ToSlash(relPath),
		Content:  string(content),
		Size:     int64(len(content)),
		Editable: true,
	})
}

func (h *Handler) handleUpdateProjectFile(w http.ResponseWriter, r *http.Request) {
	projectName, relPath, target, err := h.resolveProjectFile(r)
	if err != nil {
		http.Error(w, "invalid project or file path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to stat file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		http.Error(w, "path is a directory", http.StatusBadRequest)
		return
	}

	var req updateProjectFileRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxEditableProjectFileSize+1024)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	content := []byte(req.Content)
	if int64(len(content)) > maxEditableProjectFileSize {
		http.Error(w, "file too large to edit in web UI", http.StatusBadRequest)
		return
	}
	if !isTextContent(content) {
		http.Error(w, "content must be UTF-8 text", http.StatusBadRequest)
		return
	}

	if err := os.WriteFile(target, content, info.Mode()); err != nil {
		http.Error(w, "failed to save file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(projectFileResponse{
		Project:  projectName,
		Path:     filepath.ToSlash(relPath),
		Content:  req.Content,
		Size:     int64(len(content)),
		Editable: true,
	})
}
