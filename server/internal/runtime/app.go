package runtimeapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/relayhub/relayhub/server/internal/config"
	"github.com/relayhub/relayhub/server/internal/models"
	"github.com/relayhub/relayhub/server/internal/provider"
	"github.com/relayhub/relayhub/server/internal/router"
	"github.com/relayhub/relayhub/server/internal/session"
	"github.com/relayhub/relayhub/server/internal/storage"
)

type App struct {
	Config    *config.Store
	Store     *storage.SQLiteStore
	Router    *router.Engine
	Providers *provider.Registry
	Sessions  *session.Manager
	Events    *EventHub
}

func New(configPath string) (*App, error) {
	cfgStore, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	cfg := cfgStore.Snapshot()

	dbPath := cfg.DatabasePath
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(filepath.Dir(configPath), "..", dbPath)
	}
	sqliteStore, err := storage.Open(filepath.Clean(dbPath))
	if err != nil {
		return nil, err
	}

	app := &App{
		Config:    cfgStore,
		Store:     sqliteStore,
		Router:    router.New(),
		Providers: provider.NewRegistry(),
		Events:    NewEventHub(),
	}
	app.Sessions = session.NewManager(sqliteStore)
	return app, nil
}

func (a *App) Close() error {
	return a.Store.Close()
}

func (a *App) AuthenticateProxy(token string) (models.APIKey, models.Project, bool) {
	return a.Config.FindAPIKey(token)
}

func (a *App) AdminToken() string {
	return a.Config.Snapshot().AdminToken
}

func (a *App) Snapshot() models.AppConfig {
	return a.Config.Snapshot()
}

func (a *App) SimulateRoute(ctx context.Context, req models.NormalizedRequest) (models.RouteDecision, error) {
	binding, ok, err := a.Sessions.Get(ctx, req.ProjectID, req.SessionKey)
	if err != nil {
		return models.RouteDecision{}, err
	}
	var bindingPtr *models.SessionBinding
	if ok {
		bindingPtr = &binding
	}
	return a.Router.Decide(a.Config.Snapshot(), req, bindingPtr)
}

func (a *App) Replay(ctx context.Context, requestID string) (models.NormalizedResponse, models.RequestRecord, []models.AttemptRecord, models.RouteDecision, error) {
	record, _, err := a.Store.GetRequest(ctx, requestID)
	if err != nil {
		return models.NormalizedResponse{}, models.RequestRecord{}, nil, models.RouteDecision{}, err
	}
	normBytes, err := json.Marshal(record.NormalizedReq)
	if err != nil {
		return models.NormalizedResponse{}, models.RequestRecord{}, nil, models.RouteDecision{}, err
	}
	var req models.NormalizedRequest
	if err := json.Unmarshal(normBytes, &req); err != nil {
		return models.NormalizedResponse{}, models.RequestRecord{}, nil, models.RouteDecision{}, err
	}
	req.RequestID = ""
	return a.Proxy(ctx, req)
}

func (a *App) Proxy(ctx context.Context, req models.NormalizedRequest) (models.NormalizedResponse, models.RequestRecord, []models.AttemptRecord, models.RouteDecision, error) {
	if req.RequestID == "" {
		req.RequestID = "req_" + uuid.NewString()
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now().UTC()
	}

	startedAt := time.Now().UTC()
	binding, ok, err := a.Sessions.Get(ctx, req.ProjectID, req.SessionKey)
	if err != nil {
		return a.failRecord(req, startedAt, models.RouteDecision{}, nil, err)
	}
	var bindingPtr *models.SessionBinding
	if ok {
		bindingPtr = &binding
	}

	decision, err := a.Router.Decide(a.Config.Snapshot(), req, bindingPtr)
	if err != nil {
		return a.failRecord(req, startedAt, decision, nil, err)
	}
	a.Events.Publish("request.routed", map[string]any{
		"request_id": req.RequestID,
		"policy":     decision.Policy.Strategy,
		"candidates": len(decision.Candidates),
	})

	response, attempts, err := a.executePolicy(ctx, req, decision)
	if err != nil {
		return a.failRecord(req, startedAt, decision, attempts, err)
	}
	response.LatencyMS = time.Since(startedAt).Milliseconds()

	record := buildRecord(req, decision, response, attempts, startedAt, time.Now().UTC(), "")
	if saveErr := a.Store.SaveRequest(ctx, record, attempts); saveErr != nil {
		return models.NormalizedResponse{}, models.RequestRecord{}, nil, decision, saveErr
	}
	if bindErr := a.Sessions.Bind(ctx, req.ProjectID, req.SessionKey, response.ProviderID, response.Model); bindErr != nil {
		return models.NormalizedResponse{}, models.RequestRecord{}, nil, decision, bindErr
	}
	a.Events.Publish("request.completed", map[string]any{
		"request_id":    req.RequestID,
		"provider_id":   response.ProviderID,
		"logical_model": req.LogicalModel,
		"physical_cost": record.PhysicalCost,
	})
	return response, record, attempts, decision, nil
}

func (a *App) failRecord(req models.NormalizedRequest, startedAt time.Time, decision models.RouteDecision, attempts []models.AttemptRecord, cause error) (models.NormalizedResponse, models.RequestRecord, []models.AttemptRecord, models.RouteDecision, error) {
	record := models.RequestRecord{
		RequestID:      req.RequestID,
		ProjectID:      req.ProjectID,
		APIKeyID:       req.APIKeyID,
		SessionKey:     req.SessionKey,
		LogicalModel:   req.LogicalModel,
		EntryProtocol:  req.EntryProtocol,
		RouteStrategy:  decision.Policy.Strategy,
		MatchedRuleIDs: decision.MatchedRuleIDs,
		Status:         "failed",
		Error:          cause.Error(),
		StartedAt:      startedAt,
		CompletedAt:    time.Now().UTC(),
		LogicalUsage:   models.Usage{},
		NormalizedReq:  models.ToMap(req),
		RouteDecision:  models.ToMap(decision),
	}
	if saveErr := a.Store.SaveRequest(context.Background(), record, attempts); saveErr != nil {
		return models.NormalizedResponse{}, models.RequestRecord{}, attempts, decision, errors.Join(cause, saveErr)
	}
	a.Events.Publish("request.failed", map[string]any{
		"request_id": req.RequestID,
		"error":      cause.Error(),
	})
	return models.NormalizedResponse{}, record, attempts, decision, cause
}

func (a *App) executePolicy(ctx context.Context, req models.NormalizedRequest, decision models.RouteDecision) (models.NormalizedResponse, []models.AttemptRecord, error) {
	switch decision.Policy.Strategy {
	case "race":
		return a.executeRace(ctx, req, decision.Candidates)
	case "hedged":
		return a.executeHedged(ctx, req, decision.Candidates, decision.Policy.HedgeDelayMS)
	case "failover":
		return a.executeFailover(ctx, req, decision.Candidates)
	default:
		return a.executeSingle(ctx, req, decision.Candidates)
	}
}

func (a *App) executeSingle(ctx context.Context, req models.NormalizedRequest, candidates []models.Candidate) (models.NormalizedResponse, []models.AttemptRecord, error) {
	if len(candidates) == 0 {
		return models.NormalizedResponse{}, nil, errors.New("no candidates")
	}
	result, attempt, err := a.invokeCandidate(ctx, req, candidates[0], 0, "single")
	if err != nil {
		return models.NormalizedResponse{}, []models.AttemptRecord{attempt}, err
	}
	attempt.IsWinner = true
	attempt.Status = "succeeded"
	return result, []models.AttemptRecord{attempt}, nil
}

func (a *App) executeFailover(ctx context.Context, req models.NormalizedRequest, candidates []models.Candidate) (models.NormalizedResponse, []models.AttemptRecord, error) {
	attempts := []models.AttemptRecord{}
	var lastErr error
	for idx, candidate := range candidates {
		result, attempt, err := a.invokeCandidate(ctx, req, candidate, idx, "failover")
		attempts = append(attempts, attempt)
		if err == nil {
			attempts[len(attempts)-1].IsWinner = true
			attempts[len(attempts)-1].Status = "succeeded"
			return result, attempts, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("all candidates failed")
	}
	return models.NormalizedResponse{}, attempts, lastErr
}

func (a *App) executeHedged(ctx context.Context, req models.NormalizedRequest, candidates []models.Candidate, delayMS int) (models.NormalizedResponse, []models.AttemptRecord, error) {
	if len(candidates) <= 1 {
		return a.executeSingle(ctx, req, candidates)
	}
	if delayMS <= 0 {
		delayMS = 75
	}

	type resultEnvelope struct {
		resp    models.NormalizedResponse
		attempt models.AttemptRecord
		err     error
	}
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	resultCh := make(chan resultEnvelope, 2)
	var wg sync.WaitGroup
	run := func(idx int, mode string) {
		defer wg.Done()
		resp, attempt, err := a.invokeCandidate(childCtx, req, candidates[idx], idx, mode)
		if err != nil && errors.Is(err, context.Canceled) {
			attempt.Status = "cancelled"
			attempt.CancelledAt = time.Now().UTC()
		}
		resultCh <- resultEnvelope{resp: resp, attempt: attempt, err: err}
	}

	wg.Add(1)
	go run(0, "hedged-primary")
	timer := time.NewTimer(time.Duration(delayMS) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return models.NormalizedResponse{}, nil, ctx.Err()
	case <-timer.C:
		wg.Add(1)
		go run(1, "hedged-secondary")
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	attempts := []models.AttemptRecord{}
	var winner *models.NormalizedResponse
	var winnerAttemptIndex int
	var lastErr error
	for item := range resultCh {
		attempts = append(attempts, item.attempt)
		if item.err == nil && winner == nil {
			resp := item.resp
			winner = &resp
			winnerAttemptIndex = len(attempts) - 1
			cancel()
			continue
		}
		if item.err != nil && !errors.Is(item.err, context.Canceled) {
			lastErr = item.err
		}
	}
	if winner == nil {
		if lastErr == nil {
			lastErr = errors.New("hedged routing failed")
		}
		return models.NormalizedResponse{}, attempts, lastErr
	}
	attempts[winnerAttemptIndex].IsWinner = true
	attempts[winnerAttemptIndex].Status = "succeeded"
	return *winner, attempts, nil
}

func (a *App) executeRace(ctx context.Context, req models.NormalizedRequest, candidates []models.Candidate) (models.NormalizedResponse, []models.AttemptRecord, error) {
	type resultEnvelope struct {
		resp    models.NormalizedResponse
		attempt models.AttemptRecord
		err     error
	}
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	resultCh := make(chan resultEnvelope, len(candidates))
	var wg sync.WaitGroup
	for idx, candidate := range candidates {
		wg.Add(1)
		go func(index int, candidate models.Candidate) {
			defer wg.Done()
			resp, attempt, err := a.invokeCandidate(childCtx, req, candidate, index, "race")
			if err != nil && errors.Is(err, context.Canceled) {
				attempt.Status = "cancelled"
				attempt.CancelledAt = time.Now().UTC()
			}
			resultCh <- resultEnvelope{resp: resp, attempt: attempt, err: err}
		}(idx, candidate)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	attempts := make([]models.AttemptRecord, 0, len(candidates))
	var winner *models.NormalizedResponse
	var winnerIndex int
	var lastErr error
	for item := range resultCh {
		attempts = append(attempts, item.attempt)
		if item.err == nil && winner == nil {
			resp := item.resp
			winner = &resp
			winnerIndex = len(attempts) - 1
			cancel()
			continue
		}
		if item.err != nil && !errors.Is(item.err, context.Canceled) {
			lastErr = item.err
		}
	}
	if winner == nil {
		if lastErr == nil {
			lastErr = errors.New("race routing failed")
		}
		return models.NormalizedResponse{}, attempts, lastErr
	}
	attempts[winnerIndex].IsWinner = true
	attempts[winnerIndex].Status = "succeeded"
	return *winner, attempts, nil
}

func (a *App) invokeCandidate(ctx context.Context, req models.NormalizedRequest, candidate models.Candidate, index int, launchMode string) (models.NormalizedResponse, models.AttemptRecord, error) {
	startedAt := time.Now().UTC()
	a.Events.Publish("attempt.started", map[string]any{
		"request_id":     req.RequestID,
		"provider_id":    candidate.Provider.ID,
		"provider_model": candidate.Target.Model,
		"launch_mode":    launchMode,
	})
	result, err := a.Providers.Execute(ctx, candidate.Provider, candidate.Target, req)
	attempt := models.AttemptRecord{
		RequestID:     req.RequestID,
		AttemptIndex:  index,
		ProviderID:    candidate.Provider.ID,
		ProviderModel: candidate.Target.Model,
		Status:        "failed",
		LaunchMode:    launchMode,
		StartedAt:     startedAt,
		CompletedAt:   time.Now().UTC(),
		LatencyMS:     time.Since(startedAt).Milliseconds(),
	}
	if err != nil {
		attempt.Error = err.Error()
		a.Events.Publish("attempt.failed", map[string]any{
			"request_id":  req.RequestID,
			"provider_id": candidate.Provider.ID,
			"error":       err.Error(),
		})
		return models.NormalizedResponse{}, attempt, err
	}
	attempt.Usage = result.Usage
	a.Events.Publish("attempt.succeeded", map[string]any{
		"request_id":  req.RequestID,
		"provider_id": candidate.Provider.ID,
	})
	return result.Response, attempt, nil
}

func buildRecord(req models.NormalizedRequest, decision models.RouteDecision, response models.NormalizedResponse, attempts []models.AttemptRecord, startedAt, completedAt time.Time, failure string) models.RequestRecord {
	record := models.RequestRecord{
		RequestID:       req.RequestID,
		ProjectID:       req.ProjectID,
		APIKeyID:        req.APIKeyID,
		SessionKey:      req.SessionKey,
		LogicalModel:    req.LogicalModel,
		EntryProtocol:   req.EntryProtocol,
		RouteStrategy:   decision.Policy.Strategy,
		MatchedRuleIDs:  decision.MatchedRuleIDs,
		FinalProviderID: response.ProviderID,
		FinalModel:      response.Model,
		Status:          "succeeded",
		StartedAt:       startedAt,
		CompletedAt:     completedAt,
		LogicalUsage:    response.Usage,
		PhysicalCost:    sumPhysicalCost(attempts),
		NormalizedReq:   models.ToMap(req),
		RouteDecision:   models.ToMap(decision),
		ResponsePreview: response.OutputText,
		Error:           failure,
	}
	if failure != "" {
		record.Status = "failed"
	}
	return record
}

func sumPhysicalCost(attempts []models.AttemptRecord) float64 {
	total := 0.0
	for _, attempt := range attempts {
		total += attempt.Usage.Cost
	}
	return total
}

func (a *App) ValidateAdminToken(token string) bool {
	return token != "" && token == a.AdminToken()
}

func (a *App) Requests(ctx context.Context, limit int) ([]models.RequestRecord, error) {
	return a.Store.ListRequests(ctx, limit)
}

func (a *App) Request(ctx context.Context, requestID string) (models.RequestRecord, []models.AttemptRecord, error) {
	return a.Store.GetRequest(ctx, requestID)
}

func (a *App) SessionsList(ctx context.Context) ([]models.SessionBinding, error) {
	return a.Store.ListSessions(ctx)
}

func (a *App) UsageSummary(ctx context.Context) (models.UsageSummary, error) {
	return a.Store.UsageSummary(ctx)
}

func (a *App) UpsertProvider(item models.Provider) error {
	return a.Config.UpsertProvider(item)
}

func (a *App) UpsertLogicalModel(item models.LogicalModel) error {
	return a.Config.UpsertLogicalModel(item)
}

func (a *App) UpsertRouteRule(item models.RouteRule) error {
	return a.Config.UpsertRouteRule(item)
}

func (a *App) ExportConfig() models.AppConfig {
	return a.Config.Snapshot()
}

func (a *App) ImportConfig(cfg models.AppConfig) error {
	if cfg.Listen == "" {
		return fmt.Errorf("listen cannot be empty")
	}
	return a.Config.Save(cfg)
}
