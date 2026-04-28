package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
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
}

func (h *Handler) registerChatRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/chat/manus/send", h.handleChatManusSend)
}

func (h *Handler) handleChatManusSend(w http.ResponseWriter, r *http.Request) {
	var req chatManusSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	bc := cfg.Channels.GetByType(config.ChannelManus)
	if bc == nil {
		http.Error(w, "manus channel is not configured", http.StatusBadRequest)
		return
	}
	if !bc.Enabled {
		http.Error(w, "manus channel is disabled", http.StatusBadRequest)
		return
	}

	var manusCfg config.ManusSettings
	if err := bc.Decode(&manusCfg); err != nil {
		http.Error(w, "failed to decode manus settings: "+err.Error(), http.StatusBadRequest)
		return
	}

	apiKey := strings.TrimSpace(manusCfg.APIKey.String())
	if apiKey == "" {
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

	ctx, cancel := context.WithTimeout(r.Context(), taskTimeout)
	defer cancel()

	client := &http.Client{Timeout: manusRequestTimeout}

	taskID, err := createManusTask(ctx, client, apiBase, apiKey, content)
	if err != nil {
		http.Error(w, "failed to create manus task: "+err.Error(), http.StatusBadGateway)
		return
	}

	result, err := pollManusTaskResult(ctx, client, apiBase, apiKey, taskID, pollInterval)
	if err != nil {
		http.Error(w, "failed to poll manus task: "+err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(chatManusSendResponse{
		TaskID:  taskID,
		Content: strings.TrimSpace(result),
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
	apiBase, apiKey, taskID string,
	pollInterval time.Duration,
) (string, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var out strings.Builder

	for {
		select {
		case <-ctx.Done():
			return out.String(), ctx.Err()
		case <-ticker.C:
		}

		endpoint := fmt.Sprintf("%s/v2/task.listMessages?task_id=%s&order=desc&limit=100", strings.TrimRight(apiBase, "/"), taskID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		req.Header.Set("x-manus-api-key", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		var parsed struct {
			OK       bool              `json:"ok"`
			Messages []json.RawMessage `json:"messages"`
			Error    any               `json:"error"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		if !parsed.OK {
			return out.String(), fmt.Errorf("api error: %v", parsed.Error)
		}

		for _, raw := range parsed.Messages {
			var msg map[string]any
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}

			typ, _ := msg["type"].(string)
			switch typ {
			case "assistant_message":
				assistant, _ := msg["assistant_message"].(map[string]any)
				content, _ := assistant["content"].(string)
				content = strings.TrimSpace(content)
				if content != "" {
					out.WriteString(content)
					out.WriteString("\n")
				}
			case "error_message":
				errMsg, _ := msg["error_message"].(map[string]any)
				content, _ := errMsg["content"].(string)
				if strings.TrimSpace(content) != "" {
					return out.String(), fmt.Errorf(strings.TrimSpace(content))
				}
				return out.String(), fmt.Errorf("manus returned error_message")
			case "status_update":
				status, _ := msg["status_update"].(map[string]any)
				agentStatus, _ := status["agent_status"].(string)
				if agentStatus == "stopped" {
					return out.String(), nil
				}
				if agentStatus == "error" {
					errMessage, _ := status["error_message"].(string)
					errMessage = strings.TrimSpace(errMessage)
					if errMessage == "" {
						errMessage = "manus task failed"
					}
					return out.String(), fmt.Errorf(errMessage)
				}
			}
		}
	}
}
