package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	runtimeapp "github.com/relayhub/relayhub/server/internal/runtime"
	"github.com/relayhub/relayhub/server/internal/transport"
)

func TestChatProxyAndAdminSummary(t *testing.T) {
	t.Helper()

	app := newTestApp(t)
	defer app.Close()

	server := httptest.NewServer(transport.NewRouter(app))
	defer server.Close()

	payload := map[string]any{
		"model": "smart-fast",
		"messages": []map[string]string{
			{"role": "user", "content": "say hello from race mode"},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	response, err := http.NewRequest(http.MethodPost, server.URL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	response.Header.Set("Authorization", "Bearer relayhub-local-key")
	response.Header.Set("Content-Type", "application/json")
	response.Header.Set("X-Relay-Session-ID", "session-race-1")

	httpClient := &http.Client{Timeout: 5 * time.Second}
	httpResp, err := httpClient.Do(response)
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(httpResp.Body)
		t.Fatalf("unexpected status %d: %s", httpResp.StatusCode, string(raw))
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&completion); err != nil {
		t.Fatalf("decode completion: %v", err)
	}
	if len(completion.Choices) == 0 || completion.Choices[0].Message.Content == "" {
		t.Fatalf("expected non-empty completion")
	}

	adminReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/admin/usage/summary", nil)
	if err != nil {
		t.Fatalf("new admin request: %v", err)
	}
	adminReq.Header.Set("Authorization", "Bearer relayhub-admin")
	adminResp, err := httpClient.Do(adminReq)
	if err != nil {
		t.Fatalf("admin request failed: %v", err)
	}
	defer adminResp.Body.Close()
	if adminResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected admin status: %d", adminResp.StatusCode)
	}
	var summary struct {
		Requests int `json:"requests"`
	}
	if err := json.NewDecoder(adminResp.Body).Decode(&summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.Requests < 1 {
		t.Fatalf("expected at least one request, got %d", summary.Requests)
	}
}

func TestReplayAndSessionBinding(t *testing.T) {
	t.Helper()

	app := newTestApp(t)
	defer app.Close()

	req := map[string]any{
		"model": "smart-fast",
		"messages": []map[string]string{
			{"role": "user", "content": "bind this session"},
		},
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	server := httptest.NewServer(transport.NewRouter(app))
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second}

	proxyReq, err := http.NewRequest(http.MethodPost, server.URL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new proxy request: %v", err)
	}
	proxyReq.Header.Set("Authorization", "Bearer relayhub-local-key")
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("X-Relay-Session-ID", "sticky-session")

	resp, err := client.Do(proxyReq)
	if err != nil {
		t.Fatalf("proxy call failed: %v", err)
	}
	requestID := resp.Header.Get("X-Relay-Request-ID")
	resp.Body.Close()
	if requestID == "" {
		t.Fatalf("expected request id header")
	}

	replayReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/admin/requests/"+requestID+"/replay", nil)
	if err != nil {
		t.Fatalf("new replay request: %v", err)
	}
	replayReq.Header.Set("Authorization", "Bearer relayhub-admin")
	replayResp, err := client.Do(replayReq)
	if err != nil {
		t.Fatalf("replay call failed: %v", err)
	}
	defer replayResp.Body.Close()
	if replayResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(replayResp.Body)
		t.Fatalf("unexpected replay status %d: %s", replayResp.StatusCode, string(raw))
	}

	sessionReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/admin/sessions", nil)
	if err != nil {
		t.Fatalf("new sessions request: %v", err)
	}
	sessionReq.Header.Set("Authorization", "Bearer relayhub-admin")
	sessionResp, err := client.Do(sessionReq)
	if err != nil {
		t.Fatalf("session list failed: %v", err)
	}
	defer sessionResp.Body.Close()

	var sessions struct {
		Items []struct {
			SessionKey string `json:"session_key"`
			ProviderID string `json:"provider_id"`
		} `json:"items"`
	}
	if err := json.NewDecoder(sessionResp.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	found := false
	for _, item := range sessions.Items {
		if item.SessionKey == "sticky-session" && item.ProviderID != "" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected sticky-session binding in session list")
	}
}

func TestRaceCreatesMultipleAttempts(t *testing.T) {
	t.Helper()

	app := newTestApp(t)
	defer app.Close()

	response, record, attempts, _, err := app.Proxy(context.Background(), normalizedProxyRequest())
	if err != nil {
		t.Fatalf("proxy failed: %v", err)
	}
	if response.ProviderID == "" {
		t.Fatalf("expected winner provider")
	}
	if record.PhysicalCost < record.LogicalUsage.Cost {
		t.Fatalf("physical cost should be >= logical cost")
	}
	if len(attempts) < 2 {
		t.Fatalf("expected multiple attempts for race, got %d", len(attempts))
	}
}

func newTestApp(t *testing.T) *runtimeapp.App {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config", "relayhub.json")
	app, err := runtimeapp.New(configPath)
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	return app
}
