package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/relayhub/relayhub/server/internal/models"
	runtimeapp "github.com/relayhub/relayhub/server/internal/runtime"
)

type Handler struct {
	app *runtimeapp.App
}

func Mount(r chi.Router, app *runtimeapp.App) {
	handler := &Handler{app: app}
	r.Get("/healthz", handler.health)
	r.Get("/v1/models", handler.models)
	r.Post("/v1/chat/completions", handler.chatCompletions)
	r.Post("/v1/responses", handler.responses)
	r.Post("/v1/messages", handler.messages)
	r.Post("/v1/embeddings", handler.embeddings)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

func (h *Handler) models(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.authenticate(w, r); !ok {
		return
	}
	cfg := h.app.Snapshot()
	data := make([]map[string]any, 0, len(cfg.LogicalModels))
	for _, model := range cfg.LogicalModels {
		data = append(data, map[string]any{
			"id":          model.ID,
			"object":      "model",
			"name":        model.Name,
			"description": model.Description,
			"owned_by":    "relayhub",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (h *Handler) chatCompletions(w http.ResponseWriter, r *http.Request) {
	h.handleProxy(w, r, "openai_chat")
}

func (h *Handler) responses(w http.ResponseWriter, r *http.Request) {
	h.handleProxy(w, r, "openai_responses")
}

func (h *Handler) messages(w http.ResponseWriter, r *http.Request) {
	h.handleProxy(w, r, "anthropic_messages")
}

func (h *Handler) embeddings(w http.ResponseWriter, r *http.Request) {
	h.handleProxy(w, r, "openai_embeddings")
}

func (h *Handler) handleProxy(w http.ResponseWriter, r *http.Request, entryProtocol string) {
	apiKey, project, ok := h.authenticate(w, r)
	if !ok {
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	req, err := normalizeRequest(entryProtocol, body, apiKey, project, r.Header.Get("X-Relay-Session-ID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	response, record, _, _, err := h.app.Proxy(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	w.Header().Set("X-Relay-Request-ID", record.RequestID)
	if req.SessionKey != "" {
		w.Header().Set("X-Relay-Session-ID", req.SessionKey)
	}
	w.Header().Set("X-Relay-Provider", response.ProviderID)
	w.Header().Set("X-Relay-Model", response.Model)

	if req.Stream {
		streamResponse(w, req, response)
		return
	}
	writeJSON(w, http.StatusOK, formatResponse(entryProtocol, req.LogicalModel, response))
}

func (h *Handler) authenticate(w http.ResponseWriter, r *http.Request) (models.APIKey, models.Project, bool) {
	token := r.Header.Get("x-api-key")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		token = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
	}
	apiKey, project, ok := h.app.AuthenticateProxy(token)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": map[string]any{
				"message": "invalid or missing relay api key",
				"type":    "authentication_error",
			},
		})
		return models.APIKey{}, models.Project{}, false
	}
	return apiKey, project, true
}

func normalizeRequest(entryProtocol string, body map[string]any, apiKey models.APIKey, project models.Project, headerSession string) (models.NormalizedRequest, error) {
	modelName, _ := body["model"].(string)
	if modelName == "" {
		return models.NormalizedRequest{}, fmt.Errorf("model is required")
	}

	req := models.NormalizedRequest{
		RequestID:     "",
		EntryProtocol: entryProtocol,
		TaskType:      "chat",
		ProjectID:     project.ID,
		APIKeyID:      apiKey.ID,
		SessionKey:    headerSession,
		LogicalModel:  modelName,
		Metadata:      map[string]any{},
		RawBody:       body,
		CreatedAt:     time.Now().UTC(),
	}
	if value, ok := body["stream"].(bool); ok {
		req.Stream = value
	}
	if value, ok := body["temperature"].(float64); ok {
		req.Temperature = value
	}
	if value, ok := body["max_tokens"].(float64); ok {
		req.MaxTokens = int(value)
	}
	if value, ok := body["max_output_tokens"].(float64); ok && req.MaxTokens == 0 {
		req.MaxTokens = int(value)
	}
	if metadata, ok := body["metadata"].(map[string]any); ok {
		req.Metadata = metadata
	}
	if req.SessionKey == "" {
		if value, ok := req.Metadata["session_id"].(string); ok {
			req.SessionKey = value
		}
	}

	switch entryProtocol {
	case "openai_chat":
		req.Messages = parseOpenAIMessages(body["messages"])
	case "openai_responses":
		req.Messages = parseResponsesInput(body["input"])
	case "anthropic_messages":
		req.Messages = parseAnthropicMessages(body["messages"])
	case "openai_embeddings":
		req.TaskType = "embeddings"
		req.Messages = parseEmbeddingInput(body["input"])
	default:
		req.Messages = parseOpenAIMessages(body["messages"])
	}
	if req.SessionKey == "" {
		req.SessionKey = fmt.Sprintf("%s-%s", project.ID, modelName)
	}
	return req, nil
}

func parseOpenAIMessages(raw any) []models.Message {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]models.Message, 0, len(items))
	for _, item := range items {
		payload, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := payload["role"].(string)
		content := stringifyContent(payload["content"])
		out = append(out, models.Message{Role: role, Content: content})
	}
	return out
}

func parseResponsesInput(raw any) []models.Message {
	switch value := raw.(type) {
	case string:
		return []models.Message{{Role: "user", Content: value}}
	case []any:
		out := []models.Message{}
		for _, item := range value {
			payload, ok := item.(map[string]any)
			if !ok {
				continue
			}
			role, _ := payload["role"].(string)
			content := stringifyContent(payload["content"])
			if role == "" {
				role = "user"
			}
			out = append(out, models.Message{Role: role, Content: content})
		}
		return out
	default:
		return nil
	}
}

func parseAnthropicMessages(raw any) []models.Message {
	return parseOpenAIMessages(raw)
}

func parseEmbeddingInput(raw any) []models.Message {
	switch value := raw.(type) {
	case string:
		return []models.Message{{Role: "user", Content: value}}
	case []any:
		out := make([]models.Message, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if ok {
				out = append(out, models.Message{Role: "user", Content: text})
			}
		}
		return out
	default:
		return nil
	}
}

func stringifyContent(raw any) string {
	switch value := raw.(type) {
	case string:
		return value
	case []any:
		parts := []string{}
		for _, item := range value {
			payload, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := payload["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func formatResponse(entryProtocol, logicalModel string, response models.NormalizedResponse) map[string]any {
	switch entryProtocol {
	case "openai_responses":
		return map[string]any{
			"id":     response.ID,
			"object": "response",
			"status": "completed",
			"model":  logicalModel,
			"output": []map[string]any{
				{
					"id":   "msg_" + response.ID,
					"type": "message",
					"role": "assistant",
					"content": []map[string]any{
						{
							"type":        "output_text",
							"text":        response.OutputText,
							"annotations": []any{},
						},
					},
				},
			},
			"usage": map[string]any{
				"input_tokens":  response.Usage.InputTokens,
				"output_tokens": response.Usage.OutputTokens,
				"total_tokens":  response.Usage.TotalTokens,
			},
		}
	case "anthropic_messages":
		return map[string]any{
			"id":    "msg_" + response.ID,
			"type":  "message",
			"role":  "assistant",
			"model": logicalModel,
			"content": []map[string]any{
				{"type": "text", "text": response.OutputText},
			},
			"stop_reason": "end_turn",
			"usage": map[string]any{
				"input_tokens":  response.Usage.InputTokens,
				"output_tokens": response.Usage.OutputTokens,
			},
		}
	case "openai_embeddings":
		vector := []float64{0.13, 0.21, 0.34, 0.55, 0.89}
		return map[string]any{
			"object": "list",
			"data": []map[string]any{
				{
					"object":    "embedding",
					"index":     0,
					"embedding": vector,
				},
			},
			"model": logicalModel,
			"usage": map[string]any{
				"prompt_tokens": response.Usage.InputTokens,
				"total_tokens":  response.Usage.InputTokens,
			},
		}
	default:
		return map[string]any{
			"id":      response.ID,
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   logicalModel,
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": response.OutputText,
					},
					"finish_reason": response.FinishReason,
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     response.Usage.InputTokens,
				"completion_tokens": response.Usage.OutputTokens,
				"total_tokens":      response.Usage.TotalTokens,
			},
		}
	}
}

func streamResponse(w http.ResponseWriter, req models.NormalizedRequest, response models.NormalizedResponse) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusOK, formatResponse(req.EntryProtocol, req.LogicalModel, response))
		return
	}
	chunks := strings.Fields(response.OutputText)
	for _, chunk := range chunks {
		payload := map[string]any{
			"type":  "response.output_text.delta",
			"delta": chunk + " ",
		}
		if req.EntryProtocol == "openai_chat" {
			payload = map[string]any{
				"choices": []map[string]any{
					{"delta": map[string]any{"content": chunk + " "}, "index": 0},
				},
			}
		}
		data, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		time.Sleep(15 * time.Millisecond)
	}
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": err.Error(),
			"type":    "relay_error",
		},
	})
}

func callProxy(ctx context.Context, fn func(context.Context) (models.NormalizedResponse, error)) (models.NormalizedResponse, error) {
	return fn(ctx)
}
