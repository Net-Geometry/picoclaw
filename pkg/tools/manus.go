package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	manusToolDefaultPollInterval = 2 * time.Second
	manusToolDefaultTaskTimeout  = 5 * time.Minute
	manusToolRequestTimeout      = 30 * time.Second
)

// ManusTool delegates tasks to the Manus AI agent via the Manus API.
type ManusTool struct {
	apiKey       string
	apiBase      string
	pollInterval time.Duration
	taskTimeout  time.Duration
	httpClient   *http.Client
}

// NewManusTool creates a ManusTool with the provided credentials and options.
func NewManusTool(apiKey, apiBase string, pollIntervalMs, taskTimeoutSec int) *ManusTool {
	if apiBase == "" {
		apiBase = "https://api.manus.ai"
	}
	pollInterval := manusToolDefaultPollInterval
	if pollIntervalMs > 0 {
		pollInterval = time.Duration(pollIntervalMs) * time.Millisecond
	}
	taskTimeout := manusToolDefaultTaskTimeout
	if taskTimeoutSec > 0 {
		taskTimeout = time.Duration(taskTimeoutSec) * time.Second
	}
	return &ManusTool{
		apiKey:       apiKey,
		apiBase:      apiBase,
		pollInterval: pollInterval,
		taskTimeout:  taskTimeout,
		httpClient:   &http.Client{Timeout: manusToolRequestTimeout},
	}
}

func (t *ManusTool) Name() string {
	return "manus_task"
}

func (t *ManusTool) Description() string {
	return "Delegate a task to Manus, an autonomous AI agent with broad computer-use and web capabilities. " +
		"Manus will independently execute the task and return its results. " +
		"Use this for tasks that benefit from autonomous multi-step execution, browsing, or computer use."
}

func (t *ManusTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task": map[string]any{
				"type":        "string",
				"description": "The task or instruction to send to Manus. Be specific and self-contained.",
			},
		},
		"required": []string{"task"},
	}
}

func (t *ManusTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	task, ok := args["task"].(string)
	if !ok || strings.TrimSpace(task) == "" {
		return ErrorResult("task is required and must be a non-empty string")
	}

	taskID, err := t.createTask(ctx, task)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to create Manus task: %v", err))
	}

	result, err := t.pollResult(ctx, taskID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Manus task %s failed: %v", taskID, err))
	}

	return &ToolResult{
		ForLLM: fmt.Sprintf("[Manus task %s result]\n%s", taskID, result),
	}
}

func (t *ManusTool) createTask(ctx context.Context, content string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"message": map[string]any{"content": content},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.apiBase+"/v2/task.create", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-manus-api-key", t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}

	var result struct {
		OK      bool   `json:"ok"`
		TaskID  string `json:"task_id"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	if !result.OK || result.TaskID == "" {
		msg := result.Message
		if msg == "" {
			msg = "unknown error"
		}
		return "", fmt.Errorf("%s", msg)
	}
	return result.TaskID, nil
}

func (t *ManusTool) pollResult(ctx context.Context, taskID string) (string, error) {
	deadline := time.Now().Add(t.taskTimeout)
	var accumulated strings.Builder

	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timed out after %v", t.taskTimeout)
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(t.pollInterval):
		}

		url := fmt.Sprintf("%s/v2/task.listMessages?task_id=%s&order=desc&limit=100", t.apiBase, taskID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("x-manus-api-key", t.apiKey)

		resp, err := t.httpClient.Do(req)
		if err != nil {
			continue
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			continue
		}

		var poll struct {
			OK       bool              `json:"ok"`
			Messages []json.RawMessage `json:"messages"`
		}
		if err := json.Unmarshal(raw, &poll); err != nil || !poll.OK {
			continue
		}

		done := false
		accumulated.Reset()
		for _, msgRaw := range poll.Messages {
			var base struct {
				Type         string `json:"type"`
				StatusUpdate struct {
					AgentStatus  string `json:"agent_status"`
					ErrorMessage string `json:"error_message"`
				} `json:"status_update"`
				AssistantMessage struct {
					Content string `json:"content"`
				} `json:"assistant_message"`
				ErrorMessage struct {
					Content string `json:"content"`
				} `json:"error_message"`
			}
			if err := json.Unmarshal(msgRaw, &base); err != nil {
				continue
			}
			switch base.Type {
			case "status_update":
				switch base.StatusUpdate.AgentStatus {
				case "stopped":
					done = true
				case "error":
					errMsg := base.StatusUpdate.ErrorMessage
					if errMsg == "" {
						errMsg = "agent error"
					}
					return "", fmt.Errorf("%s", errMsg)
				}
			case "assistant_message":
				if c := strings.TrimSpace(base.AssistantMessage.Content); c != "" {
					accumulated.WriteString(c)
					accumulated.WriteString("\n")
				}
			case "error_message":
				if c := strings.TrimSpace(base.ErrorMessage.Content); c != "" {
					accumulated.WriteString("[ERROR] ")
					accumulated.WriteString(c)
					accumulated.WriteString("\n")
				}
			}
		}

		if done {
			return strings.TrimSpace(accumulated.String()), nil
		}
	}
}
