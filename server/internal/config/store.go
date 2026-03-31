package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/relayhub/relayhub/server/internal/models"
)

type Store struct {
	path string
	mu   sync.RWMutex
	cfg  models.AppConfig
}

func Load(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		defaults := DefaultConfig()
		data, marshalErr := json.MarshalIndent(defaults, "", "  ")
		if marshalErr != nil {
			return nil, marshalErr
		}
		if writeErr := os.WriteFile(path, data, 0o644); writeErr != nil {
			return nil, writeErr
		}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg models.AppConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	applyEnvOverrides(&cfg)

	return &Store{path: path, cfg: cfg}, nil
}

func DefaultConfig() models.AppConfig {
	return models.AppConfig{
		InstanceName: "RelayHub Local",
		Listen:       ":8080",
		AdminToken:   "relayhub-admin",
		DatabasePath: "./data/relayhub.db",
		Projects: []models.Project{
			{ID: "local-dev", Name: "Local Dev", DailyBudget: 50},
		},
		APIKeys: []models.APIKey{
			{ID: "local-dev-key", Name: "Local Development", Key: "relayhub-local-key", ProjectID: "local-dev", Enabled: true},
		},
		Providers: []models.Provider{
			{
				ID:               "mock-fast",
				Name:             "Mock Fast",
				Type:             "mock",
				Enabled:          true,
				Priority:         100,
				Tags:             []string{"fast", "chat"},
				LatencyMS:        40,
				JitterMS:         15,
				CostPer1KInput:   0.40,
				CostPer1KOutput:  0.80,
				ResponseTemplate: "Fast route from {{provider}} handling {{prompt}}",
				Capabilities:     []string{"chat", "responses", "messages", "embeddings"},
				HealthScore:      100,
			},
			{
				ID:               "mock-balanced",
				Name:             "Mock Balanced",
				Type:             "mock",
				Enabled:          true,
				Priority:         80,
				Tags:             []string{"balanced", "chat"},
				LatencyMS:        90,
				JitterMS:         20,
				CostPer1KInput:   0.18,
				CostPer1KOutput:  0.36,
				ResponseTemplate: "Balanced route from {{provider}} handling {{prompt}}",
				Capabilities:     []string{"chat", "responses", "messages", "embeddings"},
				HealthScore:      100,
			},
			{
				ID:               "mock-precise",
				Name:             "Mock Precise",
				Type:             "mock",
				Enabled:          true,
				Priority:         90,
				Tags:             []string{"precise", "chat"},
				LatencyMS:        120,
				JitterMS:         25,
				CostPer1KInput:   0.28,
				CostPer1KOutput:  0.56,
				ResponseTemplate: "Precise route from {{provider}} handling {{prompt}}",
				Capabilities:     []string{"chat", "responses", "messages", "embeddings"},
				HealthScore:      100,
			},
		},
		LogicalModels: []models.LogicalModel{
			{
				ID:          "smart-fast",
				Name:        "Smart Fast",
				TaskType:    "chat",
				Description: "低延迟优先，默认竞速。",
				Tags:        []string{"default", "fast"},
				Targets: []models.Target{
					{ProviderID: "mock-fast", Model: "mock-fast", Priority: 100, Weight: 1, Tags: []string{"fast"}},
					{ProviderID: "mock-balanced", Model: "mock-balanced", Priority: 80, Weight: 1, Tags: []string{"fallback"}},
				},
			},
			{
				ID:          "smart-budget",
				Name:        "Smart Budget",
				TaskType:    "chat",
				Description: "低成本优先。",
				Tags:        []string{"budget"},
				Targets: []models.Target{
					{ProviderID: "mock-balanced", Model: "mock-balanced", Priority: 100, Weight: 1, Tags: []string{"cheap"}},
					{ProviderID: "mock-precise", Model: "mock-precise", Priority: 80, Weight: 1, Tags: []string{"backup"}},
				},
			},
			{
				ID:          "smart-precise",
				Name:        "Smart Precise",
				TaskType:    "chat",
				Description: "质量和稳定性优先。",
				Tags:        []string{"precise"},
				Targets: []models.Target{
					{ProviderID: "mock-precise", Model: "mock-precise", Priority: 100, Weight: 1, Tags: []string{"precise"}},
					{ProviderID: "mock-balanced", Model: "mock-balanced", Priority: 80, Weight: 1, Tags: []string{"fallback"}},
				},
			},
		},
		RouteRules: []models.RouteRule{
			{
				ID:       "smart-fast-race",
				Name:     "Fast Model Race",
				Enabled:  true,
				Priority: 100,
				Match:    models.RouteMatch{LogicalModel: "smart-fast"},
				Policy:   models.RoutePolicy{Strategy: "race", Winner: "first_complete", MaxCandidates: 2, PreferSessionBound: true},
			},
			{
				ID:       "smart-budget-single",
				Name:     "Budget Single Route",
				Enabled:  true,
				Priority: 90,
				Match:    models.RouteMatch{LogicalModel: "smart-budget"},
				Policy:   models.RoutePolicy{Strategy: "single", Winner: "first_complete", MaxCandidates: 1, PreferSessionBound: true},
			},
			{
				ID:       "smart-precise-failover",
				Name:     "Precise Failover",
				Enabled:  true,
				Priority: 80,
				Match:    models.RouteMatch{LogicalModel: "smart-precise"},
				Policy:   models.RoutePolicy{Strategy: "failover", Winner: "first_complete", MaxCandidates: 2, PreferSessionBound: true},
			},
		},
	}
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Snapshot() models.AppConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *Store) Save(cfg models.AppConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return err
	}
	s.cfg = cfg
	return nil
}

func (s *Store) FindAPIKey(token string) (models.APIKey, models.Project, bool) {
	cfg := s.Snapshot()
	for _, apiKey := range cfg.APIKeys {
		if apiKey.Enabled && apiKey.Key == token {
			for _, project := range cfg.Projects {
				if project.ID == apiKey.ProjectID {
					return apiKey, project, true
				}
			}
		}
	}
	return models.APIKey{}, models.Project{}, false
}

func (s *Store) UpsertProvider(provider models.Provider) error {
	cfg := s.Snapshot()
	idx := slices.IndexFunc(cfg.Providers, func(item models.Provider) bool {
		return item.ID == provider.ID
	})
	if idx >= 0 {
		cfg.Providers[idx] = provider
	} else {
		cfg.Providers = append(cfg.Providers, provider)
	}
	return s.Save(cfg)
}

func (s *Store) UpsertLogicalModel(model models.LogicalModel) error {
	cfg := s.Snapshot()
	idx := slices.IndexFunc(cfg.LogicalModels, func(item models.LogicalModel) bool {
		return item.ID == model.ID
	})
	if idx >= 0 {
		cfg.LogicalModels[idx] = model
	} else {
		cfg.LogicalModels = append(cfg.LogicalModels, model)
	}
	return s.Save(cfg)
}

func (s *Store) UpsertRouteRule(rule models.RouteRule) error {
	cfg := s.Snapshot()
	idx := slices.IndexFunc(cfg.RouteRules, func(item models.RouteRule) bool {
		return item.ID == rule.ID
	})
	if idx >= 0 {
		cfg.RouteRules[idx] = rule
	} else {
		cfg.RouteRules = append(cfg.RouteRules, rule)
	}
	return s.Save(cfg)
}

func applyEnvOverrides(cfg *models.AppConfig) {
	if value := os.Getenv("RELAYHUB_INSTANCE_NAME"); value != "" {
		cfg.InstanceName = value
	}
	if value := os.Getenv("RELAYHUB_LISTEN"); value != "" {
		cfg.Listen = value
	}
	if value := os.Getenv("RELAYHUB_ADMIN_TOKEN"); value != "" {
		cfg.AdminToken = value
	}
	if value := os.Getenv("RELAYHUB_DATABASE_PATH"); value != "" {
		cfg.DatabasePath = value
	}
}
