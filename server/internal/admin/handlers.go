package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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

	r.Route("/api/admin", func(r chi.Router) {
		r.Use(handler.auth)
		r.Get("/overview", handler.overview)
		r.Get("/providers", handler.providers)
		r.Post("/providers", handler.upsertProvider)
		r.Get("/models", handler.models)
		r.Post("/models", handler.upsertModel)
		r.Get("/router/rules", handler.routeRules)
		r.Post("/router/rules", handler.upsertRouteRule)
		r.Post("/router/simulate", handler.simulateRoute)
		r.Get("/sessions", handler.sessions)
		r.Get("/requests", handler.requests)
		r.Get("/requests/{requestID}", handler.requestByID)
		r.Post("/requests/{requestID}/replay", handler.replay)
		r.Get("/usage/summary", handler.usageSummary)
		r.Get("/settings", handler.settings)
		r.Post("/export", handler.exportConfig)
		r.Post("/import", handler.importConfig)
		r.Get("/events/stream", handler.events)
	})
}

func (h *Handler) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
		if !h.app.ValidateAdminToken(token) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid admin token"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	summary, err := h.app.UsageSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	cfg := h.app.Snapshot()
	providers := []map[string]any{}
	for _, provider := range cfg.Providers {
		providers = append(providers, map[string]any{
			"id":           provider.ID,
			"name":         provider.Name,
			"type":         provider.Type,
			"enabled":      provider.Enabled,
			"priority":     provider.Priority,
			"health_score": h.app.Providers.Health(provider.ID, provider.HealthScore),
			"tags":         provider.Tags,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"instance":  cfg.InstanceName,
		"listen":    cfg.Listen,
		"summary":   summary,
		"providers": providers,
		"time":      time.Now().UTC(),
	})
}

func (h *Handler) providers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": h.app.Snapshot().Providers})
}

func (h *Handler) upsertProvider(w http.ResponseWriter, r *http.Request) {
	var item models.Provider
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if item.ID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("provider id is required"))
		return
	}
	if err := h.app.UpsertProvider(item); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": true, "provider": item})
}

func (h *Handler) models(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": h.app.Snapshot().LogicalModels})
}

func (h *Handler) upsertModel(w http.ResponseWriter, r *http.Request) {
	var item models.LogicalModel
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if item.ID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("logical model id is required"))
		return
	}
	if err := h.app.UpsertLogicalModel(item); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": true, "model": item})
}

func (h *Handler) routeRules(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": h.app.Snapshot().RouteRules})
}

func (h *Handler) upsertRouteRule(w http.ResponseWriter, r *http.Request) {
	var item models.RouteRule
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if item.ID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("route rule id is required"))
		return
	}
	if err := h.app.UpsertRouteRule(item); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": true, "rule": item})
}

func (h *Handler) simulateRoute(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Model      string `json:"model"`
		ProjectID  string `json:"project_id"`
		SessionKey string `json:"session_key"`
		Prompt     string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req := models.NormalizedRequest{
		RequestID:     "simulated",
		EntryProtocol: "openai_chat",
		TaskType:      "chat",
		LogicalModel:  input.Model,
		ProjectID:     input.ProjectID,
		SessionKey:    input.SessionKey,
		Messages:      []models.Message{{Role: "user", Content: input.Prompt}},
	}
	decision, err := h.app.SimulateRoute(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, decision)
}

func (h *Handler) sessions(w http.ResponseWriter, r *http.Request) {
	items, err := h.app.SessionsList(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) requests(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			limit = value
		}
	}
	items, err := h.app.Requests(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) requestByID(w http.ResponseWriter, r *http.Request) {
	record, attempts, err := h.app.Request(r.Context(), chi.URLParam(r, "requestID"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"request":  record,
		"attempts": attempts,
	})
}

func (h *Handler) replay(w http.ResponseWriter, r *http.Request) {
	response, record, attempts, decision, err := h.app.Replay(r.Context(), chi.URLParam(r, "requestID"))
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"response": response,
		"request":  record,
		"attempts": attempts,
		"decision": decision,
	})
}

func (h *Handler) usageSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.app.UsageSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) settings(w http.ResponseWriter, _ *http.Request) {
	cfg := h.app.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"instance_name": cfg.InstanceName,
		"listen":        cfg.Listen,
		"database_path": cfg.DatabasePath,
	})
}

func (h *Handler) exportConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.app.ExportConfig())
}

func (h *Handler) importConfig(w http.ResponseWriter, r *http.Request) {
	var cfg models.AppConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.app.ImportConfig(cfg); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": true})
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("streaming unsupported"))
		return
	}

	ch := h.app.Events.Subscribe()
	defer h.app.Events.Unsubscribe(ch)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case msg := <-ch:
			_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}
