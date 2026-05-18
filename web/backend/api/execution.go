package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
)

const (
	defaultExecutionTimeout = 30 * time.Second
	maxExecutionTimeout     = 5 * time.Minute
)

// Container execution types
type containerCreateRequest struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	// Environment variables for the container
	Env []string `json:"env,omitempty"`
	// Working directory inside container
	WorkDir string `json:"work_dir,omitempty"`
	// Command to run
	Command []string `json:"command,omitempty"`
}

type containerCreateResponse struct {
	ContainerID string `json:"container_id"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
}

type containerRunRequest struct {
	ProjectName string   `json:"project_name"`
	ProjectPath string   `json:"project_path"`
	Image       string   `json:"image"`
	Command     []string `json:"command"`
	Timeout     int      `json:"timeout,omitempty"` // in seconds
	Env         []string `json:"env,omitempty"`
	WorkDir     string   `json:"work_dir,omitempty"`
}

type containerRunResponse struct {
	ContainerID string `json:"container_id,omitempty"`
	ExitCode    int    `json:"exit_code"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	Duration    int64  `json:"duration_ms"`
	Status      string `json:"status"`
}

// Test execution types
type testRunRequest struct {
	ProjectName string `json:"project_name"`
	ProjectPath string `json:"project_path"`
	TestPath    string `json:"test_path,omitempty"` // specific test file or pattern
	Verbose     bool   `json:"verbose,omitempty"`
	Timeout     int    `json:"timeout,omitempty"` // in seconds
}

type testRunResponse struct {
	TestName string `json:"test_name"`
	Passed   int    `json:"passed"`
	Failed   int    `json:"failed"`
	Skipped  int    `json:"skipped"`
	Duration int64  `json:"duration_ms"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Status   string `json:"status"`
	ExitCode int    `json:"exit_code"`
}

// Script execution types
type scriptRunRequest struct {
	ProjectName string   `json:"project_name"`
	ProjectPath string   `json:"project_path"`
	ScriptPath  string   `json:"script_path"`
	Args        []string `json:"args,omitempty"`
	Env         []string `json:"env,omitempty"`
	Timeout     int      `json:"timeout,omitempty"` // in seconds
}

type scriptRunResponse struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Duration int64  `json:"duration_ms"`
	Status   string `json:"status"`
}

func (h *Handler) registerExecutionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/containers/run", h.handleContainerRun)
	mux.HandleFunc("POST /api/containers/create", h.handleContainerCreate)
	mux.HandleFunc("POST /api/tests/run", h.handleTestRun)
	mux.HandleFunc("POST /api/scripts/run", h.handleScriptRun)
}

// handleContainerCreate creates a new container
func (h *Handler) handleContainerCreate(w http.ResponseWriter, r *http.Request) {
	var req containerCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Image == "" {
		http.Error(w, "name and image are required", http.StatusBadRequest)
		return
	}

	// Check if Docker is available
	if !isDockerAvailable() {
		http.Error(w, "Docker is not available on this system", http.StatusServiceUnavailable)
		return
	}

	// Pull image if needed
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "pull", req.Image)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.WarnCF("docker", "Failed to pull image", map[string]any{
			"image":  req.Image,
			"error":  err,
			"output": string(output),
		})
		http.Error(w, "failed to pull image: "+string(output), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(containerCreateResponse{
		ContainerID: req.Name,
		Status:      "created",
		Message:     fmt.Sprintf("Container image %s is ready", req.Image),
	})
}

// handleContainerRun runs a command in a container
func (h *Handler) handleContainerRun(w http.ResponseWriter, r *http.Request) {
	var req containerRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Image == "" || len(req.Command) == 0 {
		http.Error(w, "image and command are required", http.StatusBadRequest)
		return
	}

	// Check if Docker is available
	if !isDockerAvailable() {
		http.Error(w, "Docker is not available on this system", http.StatusServiceUnavailable)
		return
	}

	// Resolve project path for volume mount
	projectRoot := ""
	if req.ProjectName != "" {
		_, root, err := h.projectsRoot()
		if err == nil {
			projectRoot = filepath.Join(root, req.ProjectName)
			if req.ProjectPath != "" {
				projectRoot = filepath.Join(projectRoot, req.ProjectPath)
			}
		}
	}

	timeout := defaultExecutionTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
		if timeout > maxExecutionTimeout {
			timeout = maxExecutionTimeout
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Build docker run command
	dockerCmd := []string{"docker", "run", "--rm"}

	// Add environment variables
	for _, env := range req.Env {
		dockerCmd = append(dockerCmd, "-e", env)
	}

	// Add volume mount if project path is available
	if projectRoot != "" {
		dockerCmd = append(dockerCmd, "-v", projectRoot+":/workspace")
	}

	// Set working directory
	workDir := req.WorkDir
	if workDir == "" && projectRoot != "" {
		workDir = "/workspace"
	}
	if workDir != "" {
		dockerCmd = append(dockerCmd, "-w", workDir)
	}

	// Add image and command
	dockerCmd = append(dockerCmd, req.Image)
	dockerCmd = append(dockerCmd, req.Command...)

	// Execute
	cmd := exec.CommandContext(ctx, dockerCmd[0], dockerCmd[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(containerRunResponse{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
		Status:   "completed",
	})
}

// handleTestRun runs tests in a project
func (h *Handler) handleTestRun(w http.ResponseWriter, r *http.Request) {
	var req testRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.ProjectName == "" {
		http.Error(w, "project_name is required", http.StatusBadRequest)
		return
	}

	// Resolve project path
	_, root, err := h.projectsRoot()
	if err != nil {
		http.Error(w, "failed to resolve project root: "+err.Error(), http.StatusInternalServerError)
		return
	}

	projectPath := filepath.Join(root, req.ProjectName)
	if req.ProjectPath != "" {
		projectPath = filepath.Join(projectPath, req.ProjectPath)
	}

	// Verify project exists
	if _, err := os.Stat(projectPath); err != nil {
		http.Error(w, "project path not found: "+err.Error(), http.StatusNotFound)
		return
	}

	timeout := defaultExecutionTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
		if timeout > maxExecutionTimeout {
			timeout = maxExecutionTimeout
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Try to detect and run tests
	testCmd := detectTestCommand(projectPath, req.TestPath, req.Verbose)
	if testCmd == nil {
		http.Error(w, "unable to detect test command for this project", http.StatusBadRequest)
		return
	}

	cmd := exec.CommandContext(ctx, testCmd[0], testCmd[1:]...)
	cmd.Dir = projectPath
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err = cmd.Run()
	duration := time.Since(startTime).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	// Parse test output for counts (simplified)
	passed, failed, skipped := parseTestOutput(stdout.String() + "\n" + stderr.String())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(testRunResponse{
		TestName: req.ProjectName,
		Passed:   passed,
		Failed:   failed,
		Skipped:  skipped,
		Duration: duration,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Status:   "completed",
		ExitCode: exitCode,
	})
}

// handleScriptRun runs a script in a project
func (h *Handler) handleScriptRun(w http.ResponseWriter, r *http.Request) {
	var req scriptRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.ProjectName == "" || req.ScriptPath == "" {
		http.Error(w, "project_name and script_path are required", http.StatusBadRequest)
		return
	}

	// Resolve project path
	_, root, err := h.projectsRoot()
	if err != nil {
		http.Error(w, "failed to resolve project root: "+err.Error(), http.StatusInternalServerError)
		return
	}

	projectPath := filepath.Join(root, req.ProjectName)
	if req.ProjectPath != "" {
		projectPath = filepath.Join(projectPath, req.ProjectPath)
	}

	// Build script path with validation
	scriptPath, err := safeRelPath(req.ScriptPath)
	if err != nil {
		http.Error(w, "invalid script path", http.StatusBadRequest)
		return
	}

	fullScriptPath := filepath.Join(projectPath, scriptPath)

	// Verify script exists and is executable
	info, err := os.Stat(fullScriptPath)
	if err != nil {
		http.Error(w, "script not found: "+err.Error(), http.StatusNotFound)
		return
	}
	if info.IsDir() {
		http.Error(w, "script path is a directory", http.StatusBadRequest)
		return
	}

	timeout := defaultExecutionTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
		if timeout > maxExecutionTimeout {
			timeout = maxExecutionTimeout
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Prepare command based on script type
	var cmd *exec.Cmd
	if strings.HasSuffix(fullScriptPath, ".sh") {
		cmd = exec.CommandContext(ctx, "sh", append([]string{fullScriptPath}, req.Args...)...)
	} else if strings.HasSuffix(fullScriptPath, ".py") {
		cmd = exec.CommandContext(ctx, "python3", append([]string{fullScriptPath}, req.Args...)...)
	} else if strings.HasSuffix(fullScriptPath, ".go") {
		cmd = exec.CommandContext(ctx, "go", append([]string{"run", fullScriptPath}, req.Args...)...)
	} else {
		// Try to execute directly
		cmd = exec.CommandContext(ctx, fullScriptPath, req.Args...)
	}

	cmd.Dir = projectPath
	for _, env := range req.Env {
		cmd.Env = append(cmd.Env, env)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err = cmd.Run()
	duration := time.Since(startTime).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scriptRunResponse{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
		Status:   "completed",
	})
}

// Helper functions

// isDockerAvailable checks if docker command is available
func isDockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "version")
	return cmd.Run() == nil
}

// detectTestCommand tries to detect what test runner to use
func detectTestCommand(projectPath, testPath string, verbose bool) []string {
	// Check for Go tests
	if _, err := os.Stat(filepath.Join(projectPath, "go.mod")); err == nil {
		args := []string{"test"}
		if verbose {
			args = append(args, "-v")
		}
		if testPath != "" {
			args = append(args, testPath)
		} else {
			args = append(args, "./...")
		}
		return append([]string{"go"}, args...)
	}

	// Check for Node/npm tests
	if _, err := os.Stat(filepath.Join(projectPath, "package.json")); err == nil {
		args := []string{"test"}
		if verbose {
			args = append(args, "--", "--verbose")
		}
		return append([]string{"npm"}, args...)
	}

	// Check for Python tests
	if _, err := os.Stat(filepath.Join(projectPath, "pytest.ini")); err == nil {
		args := []string{"pytest"}
		if verbose {
			args = append(args, "-v")
		}
		if testPath != "" {
			args = append(args, testPath)
		}
		return args
	}

	// Check for Makefile
	if _, err := os.Stat(filepath.Join(projectPath, "Makefile")); err == nil {
		return []string{"make", "test"}
	}

	return nil
}

// parseTestOutput extracts test counts from output (simplified)
func parseTestOutput(output string) (passed, failed, skipped int) {
	// This is a simplified parser - in production, you'd use more sophisticated parsing
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.ToLower(line)
		if strings.Contains(line, "passed") || strings.Contains(line, "✓") {
			passed++
		}
		if strings.Contains(line, "failed") || strings.Contains(line, "✗") {
			failed++
		}
		if strings.Contains(line, "skipped") || strings.Contains(line, "skip") {
			skipped++
		}
	}
	return
}
