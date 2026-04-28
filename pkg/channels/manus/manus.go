package manus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
)

const (
	defaultAPIBase      = "https://api.manus.ai"
	defaultPollInterval = 2000 // 2 seconds
	defaultTaskTimeout  = 300  // 5 minutes
	maxRetries          = 3
	requestTimeout      = 30 * time.Second
)

// ManusChannel implements the Channel interface for Manus AI agent integration.
// Manus is a comprehensive AI agent that can plan, execute multi-step workflows, use tools,
// browse the web, and deliver results. This channel sends messages to Manus and polls for results.
type ManusChannel struct {
	*channels.BaseChannel
	config       *config.ManusSettings
	apiClient    *http.Client
	apiBase      string
	apiKey       string
	pollInterval time.Duration
	taskTimeout  time.Duration
	taskStates   map[string]string // taskID -> last known status
}

// NewManusChannel creates a new Manus channel instance.
func NewManusChannel(
	bc *config.Channel,
	cfg *config.ManusSettings,
	messageBus *bus.MessageBus,
) (*ManusChannel, error) {
	if cfg.APIKey.String() == "" {
		return nil, fmt.Errorf("manus api_key is required")
	}

	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = defaultAPIBase
	}

	pollInterval := time.Duration(cfg.PollInterval) * time.Millisecond
	if pollInterval == 0 {
		pollInterval = time.Duration(defaultPollInterval) * time.Millisecond
	}

	taskTimeout := time.Duration(cfg.TaskTimeout) * time.Second
	if taskTimeout == 0 {
		taskTimeout = time.Duration(defaultTaskTimeout) * time.Second
	}

	base := channels.NewBaseChannel("manus", cfg, messageBus, bc.AllowFrom,
		channels.WithReasoningChannelID(bc.ReasoningChannelID),
	)

	return &ManusChannel{
		BaseChannel:  base,
		config:       cfg,
		apiClient:    &http.Client{Timeout: requestTimeout},
		apiBase:      apiBase,
		apiKey:       cfg.APIKey.String(),
		pollInterval: pollInterval,
		taskTimeout:  taskTimeout,
		taskStates:   make(map[string]string),
	}, nil
}

// Start initializes the Manus channel.
func (c *ManusChannel) Start(ctx context.Context) error {
	logger.InfoC("manus", "Starting Manus channel")
	c.SetRunning(true)
	logger.InfoC("manus", "Manus channel started")
	return nil
}

// Stop gracefully stops the Manus channel.
func (c *ManusChannel) Stop(ctx context.Context) error {
	logger.InfoC("manus", "Stopping Manus channel")
	c.SetRunning(false)
	return nil
}

// Send submits a message to Manus and polls for the result.
func (c *ManusChannel) Send(ctx context.Context, msg bus.OutboundMessage) ([]string, error) {
	if !c.IsRunning() {
		return nil, fmt.Errorf("manus channel is not running")
	}

	// Extract message content
	content := msg.Content
	if content == "" {
		return nil, fmt.Errorf("empty message content")
	}

	logger.DebugCF("manus", "Sending message to Manus", map[string]any{
		"chat_id": msg.ChatID,
		"content": content[:min(100, len(content))] + "...",
	})

	// Create task
	taskID, err := c.createTask(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	logger.InfoCF("manus", "Task created", map[string]any{"task_id": taskID})

	// Poll for results
	result, err := c.pollTaskResult(ctx, taskID)
	if err != nil {
		// Return task ID even on timeout/error so caller can track it
		logger.WarnCF("manus", "Poll error", map[string]any{
			"task_id": taskID,
			"error":   err.Error(),
		})
		return []string{taskID}, err
	}

	// Log the result
	if result != "" {
		logger.DebugCF("manus", "Task completed with result", map[string]any{
			"task_id": taskID,
			"result":  result[:min(200, len(result))] + "...",
		})
	}

	return []string{taskID}, nil
}

// createTask creates a new task in Manus.
// POST /v2/task.create
func (c *ManusChannel) createTask(ctx context.Context, message string) (string, error) {
	endpoint := c.apiBase + "/v2/task.create"

	body := map[string]any{
		"message": map[string]any{
			"content": message,
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-manus-api-key", c.apiKey)

	resp, err := c.apiClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create task failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var respData struct {
		OK      bool   `json:"ok"`
		TaskID  string `json:"task_id"`
		Error   any    `json:"error"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if !respData.OK {
		return "", fmt.Errorf("API error: %v", respData.Error)
	}

	if respData.TaskID == "" {
		return "", fmt.Errorf("no task_id in response")
	}

	return respData.TaskID, nil
}

// pollTaskResult polls for the task result until completion or timeout.
// GET /v2/task.listMessages?task_id=...
func (c *ManusChannel) pollTaskResult(ctx context.Context, taskID string) (string, error) {
	deadline := time.Now().Add(c.taskTimeout)
	var resultBuilder strings.Builder

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(c.pollInterval):
		}

		if time.Now().After(deadline) {
			return resultBuilder.String(), fmt.Errorf("task timeout after %v", c.taskTimeout)
		}

		// Poll for messages
		endpoint := fmt.Sprintf("%s/v2/task.listMessages?task_id=%s&order=desc&limit=100", c.apiBase, taskID)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}

		req.Header.Set("x-manus-api-key", c.apiKey)

		resp, err := c.apiClient.Do(req)
		if err != nil {
			logger.WarnCF("manus", "Poll request failed", map[string]any{
				"task_id": taskID,
				"error":   err.Error(),
			})
			continue
		}

		respData := struct {
			OK       bool              `json:"ok"`
			Messages []json.RawMessage `json:"messages"`
			Error    any               `json:"error"`
		}{}

		if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
			resp.Body.Close()
			logger.WarnCF("manus", "Failed to decode poll response", map[string]any{
				"task_id": taskID,
				"error":   err.Error(),
			})
			continue
		}
		resp.Body.Close()

		if !respData.OK {
			logger.WarnCF("manus", "Poll API error", map[string]any{
				"task_id": taskID,
				"error":   respData.Error,
			})
			continue
		}

		// Process messages
		for _, msgRaw := range respData.Messages {
			var msg map[string]any
			if err := json.Unmarshal(msgRaw, &msg); err != nil {
				continue
			}

			msgType, ok := msg["type"].(string)
			if !ok {
				continue
			}

			switch msgType {
			case "status_update":
				if statusData, ok := msg["status_update"].(map[string]any); ok {
					agentStatus, _ := statusData["agent_status"].(string)

					if agentStatus == "stopped" {
						// Task completed - extract results
						return resultBuilder.String(), nil
					} else if agentStatus == "error" {
						if errMsg, ok := statusData["error_message"].(string); ok {
							return resultBuilder.String(), fmt.Errorf("task error: %s", errMsg)
						}
						return resultBuilder.String(), fmt.Errorf("task error")
					} else if agentStatus == "waiting" {
						logger.DebugCF("manus", "Task waiting for input", map[string]any{
							"task_id": taskID,
							"detail":  statusData["status_detail"],
						})
					}
				}

			case "assistant_message":
				if assistantMsg, ok := msg["assistant_message"].(map[string]any); ok {
					if content, ok := assistantMsg["content"].(string); ok && content != "" {
						resultBuilder.WriteString(content)
						resultBuilder.WriteString("\n")
					}
				}

			case "error_message":
				if errMsg, ok := msg["error_message"].(map[string]any); ok {
					if content, ok := errMsg["content"].(string); ok && content != "" {
						logger.WarnCF("manus", "Task error message", map[string]any{
							"task_id": taskID,
							"error":   content,
						})
						resultBuilder.WriteString("[ERROR] ")
						resultBuilder.WriteString(content)
						resultBuilder.WriteString("\n")
					}
				}
			}
		}
	}
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
