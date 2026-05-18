package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
)

const (
	manusDefaultAPIBase      = "https://api.manus.ai"
	manusDefaultPollInterval = 2 * time.Second
	manusDefaultTaskTimeout  = 5 * time.Minute
	manusRequestTimeout      = 30 * time.Second
)

type chatManusSendRequest struct {
	Content string `json:"content"`
}

type chatManusSendResponse struct {
	TaskID  string `json:"task_id"`
	Content string `json:"content"`
	TraceID string `json:"trace_id,omitempty"`
}

type manusPollResult struct {
	Content             string
	PollCount           int
	AssistantMsgCount   int
	StatusUpdateCount   int
	ErrorMsgCount       int
	LastObservedStatus  string
	LastObservedMessage string
}

func newManusTraceID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("manus-%d", time.Now().UnixNano())
	}
	return "manus-" + hex.EncodeToString(buf)
}

func previewContent(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

func (h *Handler) registerChatRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/chat/manus/send", h.handleChatManusSend)
}

func (h *Handler) handleChatManusSend(w http.ResponseWriter, r *http.Request) {
	traceID := newManusTraceID()
	start := time.Now()
	w.Header().Set("X-Pico-Trace-ID", traceID)

	var req chatManusSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.WarnCF("web-manus", "Invalid manus send request JSON", map[string]any{
			"trace_id": traceID,
			"error":    err,
		})
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		logger.WarnCF("web-manus", "Rejected empty manus message", map[string]any{
			"trace_id": traceID,
		})
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	logger.InfoCF("web-manus", "Received manus chat request", map[string]any{
		"trace_id":        traceID,
		"content_len":     len(content),
		"content_preview": previewContent(content, 160),
		"remote_addr":     strings.TrimSpace(r.RemoteAddr),
		"user_agent":      strings.TrimSpace(r.UserAgent()),
	})

	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		logger.ErrorCF("web-manus", "Failed to load config for manus request", map[string]any{
			"trace_id": traceID,
			"error":    err,
		})
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	bc := cfg.Channels.GetByType(config.ChannelManus)
	if bc == nil {
		logger.WarnCF("web-manus", "Manus channel not configured", map[string]any{
			"trace_id": traceID,
		})
		http.Error(w, "manus channel is not configured", http.StatusBadRequest)
		return
	}
	if !bc.Enabled {
		logger.WarnCF("web-manus", "Manus channel disabled", map[string]any{
			"trace_id": traceID,
		})
		http.Error(w, "manus channel is disabled", http.StatusBadRequest)
		return
	}

	var manusCfg config.ManusSettings
	if err := bc.Decode(&manusCfg); err != nil {
		logger.ErrorCF("web-manus", "Failed to decode manus settings", map[string]any{
			"trace_id": traceID,
			"error":    err,
		})
		http.Error(w, "failed to decode manus settings: "+err.Error(), http.StatusBadRequest)
		return
	}

	apiKey := strings.TrimSpace(manusCfg.APIKey.String())
	if apiKey == "" {
		logger.WarnCF("web-manus", "Manus API key missing", map[string]any{
			"trace_id": traceID,
		})
		http.Error(w, "manus api_key is not configured", http.StatusBadRequest)
		return
	}

	apiBase := strings.TrimSpace(manusCfg.APIBase)
	if apiBase == "" {
		apiBase = manusDefaultAPIBase
	}

	pollInterval := manusDefaultPollInterval
	if manusCfg.PollInterval > 0 {
		pollInterval = time.Duration(manusCfg.PollInterval) * time.Millisecond
	}

	taskTimeout := manusDefaultTaskTimeout
	if manusCfg.TaskTimeout > 0 {
		taskTimeout = time.Duration(manusCfg.TaskTimeout) * time.Second
	}

	logger.InfoCF("web-manus", "Prepared manus model service request", map[string]any{
		"trace_id":         traceID,
		"api_base":         apiBase,
		"poll_interval_ms": pollInterval.Milliseconds(),
		"task_timeout_ms":  taskTimeout.Milliseconds(),
	})

	ctx, cancel := context.WithTimeout(r.Context(), taskTimeout)
	defer cancel()

	client := &http.Client{Timeout: manusRequestTimeout}

	taskCreateStart := time.Now()
	taskID, err := createManusTask(ctx, client, apiBase, apiKey, content)
	if err != nil {
		logger.ErrorCF("web-manus", "Failed to create manus task", map[string]any{
			"trace_id":         traceID,
			"error":            err,
			"create_duration_ms": time.Since(taskCreateStart).Milliseconds(),
		})
		http.Error(w, "failed to create manus task: "+err.Error(), http.StatusBadGateway)
		return
	}

	logger.InfoCF("web-manus", "Created manus task", map[string]any{
		"trace_id":           traceID,
		"task_id":            taskID,
		"create_duration_ms": time.Since(taskCreateStart).Milliseconds(),
	})

	pollStart := time.Now()
	pollResult, err := pollManusTaskResult(ctx, client, apiBase, apiKey, taskID, traceID, pollInterval)
	if err != nil {
		logger.ErrorCF("web-manus", "Failed while polling manus task", map[string]any{
			"trace_id":              traceID,
			"task_id":               taskID,
			"error":                 err,
			"poll_count":            pollResult.PollCount,
			"assistant_msg_count":   pollResult.AssistantMsgCount,
			"status_update_count":   pollResult.StatusUpdateCount,
			"error_msg_count":       pollResult.ErrorMsgCount,
			"last_observed_status":  pollResult.LastObservedStatus,
			"last_observed_message": previewContent(pollResult.LastObservedMessage, 160),
			"poll_duration_ms":      time.Since(pollStart).Milliseconds(),
		})
		http.Error(w, "failed to poll manus task: "+err.Error(), http.StatusBadGateway)
		return
	}

	logger.InfoCF("web-manus", "Completed manus task", map[string]any{
		"trace_id":             traceID,
		"task_id":              taskID,
		"poll_count":           pollResult.PollCount,
		"assistant_msg_count":  pollResult.AssistantMsgCount,
		"status_update_count":  pollResult.StatusUpdateCount,
		"poll_duration_ms":     time.Since(pollStart).Milliseconds(),
		"request_duration_ms":  time.Since(start).Milliseconds(),
		"response_len":         len(strings.TrimSpace(pollResult.Content)),
		"response_preview":     previewContent(pollResult.Content, 200),
		"last_observed_status": pollResult.LastObservedStatus,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(chatManusSendResponse{
		TaskID:  taskID,
		Content: strings.TrimSpace(pollResult.Content),
		TraceID: traceID,
	})
}

func createManusTask(ctx context.Context, client *http.Client, apiBase, apiKey, content string) (string, error) {
	body := map[string]any{
		"message": map[string]any{
			"content": content,
		},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	endpoint := strings.TrimRight(apiBase, "/") + "/v2/task.create"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-manus-api-key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}

	var parsed struct {
		OK     bool   `json:"ok"`
		TaskID string `json:"task_id"`
		Error  any    `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if !parsed.OK {
		return "", fmt.Errorf("api error: %v", parsed.Error)
	}
	if strings.TrimSpace(parsed.TaskID) == "" {
		return "", fmt.Errorf("empty task_id")
	}

	return parsed.TaskID, nil
}

func pollManusTaskResult(
	ctx context.Context,
	client *http.Client,
	apiBase, apiKey, taskID, traceID string,
	pollInterval time.Duration,
) (manusPollResult, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	result := manusPollResult{}
	var out strings.Builder

	for {
		select {
		case <-ctx.Done():
			result.Content = out.String()
			return result, ctx.Err()
		case <-ticker.C:
		}

		result.PollCount++

		endpoint := fmt.Sprintf("%s/v2/task.listMessages?task_id=%s&order=desc&limit=100", strings.TrimRight(apiBase, "/"), taskID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			logger.WarnCF("web-manus", "Failed to build manus poll request", map[string]any{
				"trace_id": traceID,
				"task_id": taskID,
				"error":   err,
			})
			continue
		}
		req.Header.Set("x-manus-api-key", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			logger.WarnCF("web-manus", "Failed to call manus poll endpoint", map[string]any{
				"trace_id": traceID,
				"task_id": taskID,
				"error":   err,
			})
			continue
		}

		var parsed struct {
			OK       bool              `json:"ok"`
			Messages []json.RawMessage `json:"messages"`
			Error    any               `json:"error"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
			resp.Body.Close()
			logger.WarnCF("web-manus", "Failed to decode manus poll response", map[string]any{
				"trace_id": traceID,
				"task_id": taskID,
				"error":   err,
			})
			continue
		}
		resp.Body.Close()

		if !parsed.OK {
			result.Content = out.String()
			return result, fmt.Errorf("api error: %v", parsed.Error)
		}

		for _, raw := range parsed.Messages {
			var msg map[string]any
			if err := json.Unmarshal(raw, &msg); err != nil {
				logger.WarnCF("web-manus", "Failed to parse manus message payload", map[string]any{
					"trace_id": traceID,
					"task_id": taskID,
					"error":   err,
				})
				continue
			}

			typ, _ := msg["type"].(string)
			switch typ {
			case "assistant_message":
				result.AssistantMsgCount++
				assistant, _ := msg["assistant_message"].(map[string]any)
				content, _ := assistant["content"].(string)
				content = strings.TrimSpace(content)
				if content != "" {
					result.LastObservedMessage = content
					out.WriteString(content)
					out.WriteString("\n")
				}
			case "error_message":
				result.ErrorMsgCount++
				errMsg, _ := msg["error_message"].(map[string]any)
				content, _ := errMsg["content"].(string)
				if strings.TrimSpace(content) != "" {
					result.Content = out.String()
					result.LastObservedMessage = content
					return result, fmt.Errorf("%s", strings.TrimSpace(content))
				}
				result.Content = out.String()
				return result, fmt.Errorf("manus returned error_message")
			case "status_update":
				result.StatusUpdateCount++
				status, _ := msg["status_update"].(map[string]any)
				agentStatus, _ := status["agent_status"].(string)
				result.LastObservedStatus = agentStatus
				if agentStatus == "stopped" {
					result.Content = out.String()
					return result, nil
				}
				if agentStatus == "error" {
					errMessage, _ := status["error_message"].(string)
					errMessage = strings.TrimSpace(errMessage)
					if errMessage == "" {
						errMessage = "manus task failed"
					}
					result.Content = out.String()
					result.LastObservedMessage = errMessage
					return result, fmt.Errorf("%s", errMessage)
				}
			}
		}
	}
}
