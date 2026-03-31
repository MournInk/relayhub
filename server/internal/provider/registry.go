package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/relayhub/relayhub/server/internal/models"
)

type Registry struct {
	client *http.Client
	mu     sync.RWMutex
	health map[string]int
}

type AttemptResult struct {
	Response models.NormalizedResponse
	Usage    models.Usage
}

func NewRegistry() *Registry {
	return &Registry{
		client: &http.Client{Timeout: 90 * time.Second},
		health: map[string]int{},
	}
}

func (r *Registry) Execute(ctx context.Context, provider models.Provider, target models.Target, req models.NormalizedRequest) (AttemptResult, error) {
	switch provider.Type {
	case "mock":
		result, err := r.executeMock(ctx, provider, target, req)
		r.record(provider.ID, err == nil)
		return result, err
	case "openai":
		result, err := r.executeOpenAI(ctx, provider, target, req)
		r.record(provider.ID, err == nil)
		return result, err
	default:
		err := fmt.Errorf("provider type %q not supported", provider.Type)
		r.record(provider.ID, false)
		return AttemptResult{}, err
	}
}

func (r *Registry) Health(providerID string, fallback int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if value, ok := r.health[providerID]; ok {
		return value
	}
	return fallback
}

func (r *Registry) record(providerID string, success bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.health[providerID]
	if !ok {
		current = 100
	}
	if success {
		current += 2
	} else {
		current -= 10
	}
	if current > 100 {
		current = 100
	}
	if current < 0 {
		current = 0
	}
	r.health[providerID] = current
}

func (r *Registry) executeMock(ctx context.Context, provider models.Provider, target models.Target, req models.NormalizedRequest) (AttemptResult, error) {
	jitter := 0
	if provider.JitterMS > 0 {
		jitter = rand.Intn(provider.JitterMS + 1)
	}
	delay := time.Duration(provider.LatencyMS+jitter) * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return AttemptResult{}, ctx.Err()
	case <-timer.C:
	}

	prompt := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			prompt = req.Messages[i].Content
			break
		}
	}
	if prompt == "" && len(req.Messages) > 0 {
		prompt = req.Messages[len(req.Messages)-1].Content
	}

	responseText := provider.ResponseTemplate
	responseText = strings.ReplaceAll(responseText, "{{provider}}", provider.Name)
	responseText = strings.ReplaceAll(responseText, "{{prompt}}", prompt)
	if responseText == "" {
		responseText = "RelayHub mock provider response"
	}

	inputTokens := models.EstimateTokens(req.Messages)
	outputTokens := len(strings.Fields(responseText)) + 8
	usage := models.Usage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		Cost:         calcCost(provider, inputTokens, outputTokens),
	}
	return AttemptResult{
		Response: models.NormalizedResponse{
			ID:           "resp_" + uuid.NewString(),
			ProviderID:   provider.ID,
			ProviderType: provider.Type,
			Model:        target.Model,
			OutputText:   responseText,
			FinishReason: "stop",
			Usage:        usage,
			Raw: map[string]any{
				"provider": provider.ID,
				"model":    target.Model,
				"mock":     true,
			},
		},
		Usage: usage,
	}, nil
}

func (r *Registry) executeOpenAI(ctx context.Context, provider models.Provider, target models.Target, req models.NormalizedRequest) (AttemptResult, error) {
	payload := map[string]any{
		"model":       target.Model,
		"messages":    toOpenAIMessages(req.Messages),
		"stream":      false,
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return AttemptResult{}, err
	}

	url := strings.TrimSuffix(provider.BaseURL, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return AttemptResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if provider.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	httpResp, err := r.client.Do(httpReq)
	if err != nil {
		return AttemptResult{}, err
	}
	defer httpResp.Body.Close()
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return AttemptResult{}, err
	}
	if httpResp.StatusCode >= 400 {
		return AttemptResult{}, fmt.Errorf("provider returned %s: %s", httpResp.Status, string(respBody))
	}

	var parsed struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return AttemptResult{}, err
	}
	outputText := ""
	finishReason := "stop"
	if len(parsed.Choices) > 0 {
		outputText = parsed.Choices[0].Message.Content
		if parsed.Choices[0].FinishReason != "" {
			finishReason = parsed.Choices[0].FinishReason
		}
	}
	usage := models.Usage{
		InputTokens:  parsed.Usage.PromptTokens,
		OutputTokens: parsed.Usage.CompletionTokens,
		TotalTokens:  parsed.Usage.TotalTokens,
		Cost:         calcCost(provider, parsed.Usage.PromptTokens, parsed.Usage.CompletionTokens),
	}
	return AttemptResult{
		Response: models.NormalizedResponse{
			ID:           parsed.ID,
			ProviderID:   provider.ID,
			ProviderType: provider.Type,
			Model:        parsed.Model,
			OutputText:   outputText,
			FinishReason: finishReason,
			Usage:        usage,
			Raw:          map[string]any{"raw": string(respBody)},
		},
		Usage: usage,
	}, nil
}

func toOpenAIMessages(messages []models.Message) []map[string]string {
	out := make([]map[string]string, 0, len(messages))
	for _, message := range messages {
		out = append(out, map[string]string{
			"role":    message.Role,
			"content": message.Content,
		})
	}
	return out
}

func calcCost(provider models.Provider, inputTokens, outputTokens int) float64 {
	return (float64(inputTokens)/1000.0)*provider.CostPer1KInput + (float64(outputTokens)/1000.0)*provider.CostPer1KOutput
}
