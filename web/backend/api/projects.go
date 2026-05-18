package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

const (
	maxEditableProjectFileSize = int64(2 * 1024 * 1024)   // 2MB
	maxUploadFileSize          = int64(100 * 1024 * 1024) // 100MB
)

type projectFileUploadResponse struct {
	Project string `json:"project"`
	Path    string `json:"path"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
}

func (h *Handler) registerProjectRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/projects", h.handleListProjects)
	mux.HandleFunc("GET /api/projects/{name}/entries", h.handleListProjectEntries)
	mux.HandleFunc("GET /api/projects/{name}/file", h.handleGetProjectFile)
	mux.HandleFunc("PUT /api/projects/{name}/file", h.handleUpdateProjectFile)
	mux.HandleFunc("POST /api/projects/{name}/upload", h.handleFileUpload)
	mux.HandleFunc("GET /api/projects/{name}/download", h.handleFileDownload)
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

// handleFileUpload handles multipart file uploads to a project directory
func (h *Handler) handleFileUpload(w http.ResponseWriter, r *http.Request) {
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
	if relPath == "." {
		relPath = ""
	}

	projectRoot := filepath.Join(root, projectName)
	targetDir := filepath.Join(projectRoot, relPath)

	// Validate target directory exists and is a directory
	info, err := os.Stat(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "target directory not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to stat directory: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !info.IsDir() {
		http.Error(w, "target path is not a directory", http.StatusBadRequest)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(maxUploadFileSize); err != nil {
		http.Error(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	// Process uploaded files
	uploadedFiles := make([]projectFileUploadResponse, 0)
	for _, fheaders := range r.MultipartForm.File {
		for _, fheader := range fheaders {
			// Validate filename
			fname := filepath.Base(fheader.Filename)
			if fname == "" || fname == "." || fname == ".." || strings.Contains(fname, "/") || strings.Contains(fname, "\\") {
				continue
			}

			file, err := fheader.Open()
			if err != nil {
				continue
			}
			defer file.Close()

			// Limit file size
			limitedFile := io.LimitReader(file, maxUploadFileSize)

			targetPath := filepath.Join(targetDir, fname)

			// Write file
			outFile, err := os.Create(targetPath)
			if err != nil {
				continue
			}
			defer outFile.Close()

			written, err := io.Copy(outFile, limitedFile)
			if err != nil {
				os.Remove(targetPath)
				continue
			}

			filePath := fname
			if relPath != "" {
				filePath = filepath.Join(relPath, fname)
			}

			uploadedFiles = append(uploadedFiles, projectFileUploadResponse{
				Project: projectName,
				Path:    filepath.ToSlash(filePath),
				Name:    fname,
				Size:    written,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"project": projectName,
		"files":   uploadedFiles,
	})
}

// handleFileDownload handles file downloads from a project
func (h *Handler) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	_, relPath, target, err := h.resolveProjectFile(r)
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

	// Open and serve the file
	file, err := os.Open(target)
	if err != nil {
		http.Error(w, "failed to open file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Set appropriate headers for download
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(relPath)+"\"")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	if _, err := io.Copy(w, file); err != nil {
		// Response already started, can't send error
		return
	}
}
