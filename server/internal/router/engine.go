package router

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/relayhub/relayhub/server/internal/models"
)

type Engine struct{}

func New() *Engine {
	return &Engine{}
}

func (e *Engine) Decide(cfg models.AppConfig, req models.NormalizedRequest, binding *models.SessionBinding) (models.RouteDecision, error) {
	logicalModelIdx := slices.IndexFunc(cfg.LogicalModels, func(item models.LogicalModel) bool {
		return item.ID == req.LogicalModel
	})
	if logicalModelIdx < 0 {
		return models.RouteDecision{}, fmt.Errorf("logical model %q not found", req.LogicalModel)
	}
	logicalModel := cfg.LogicalModels[logicalModelIdx]

	policy := models.RoutePolicy{
		Strategy:           "single",
		Winner:             "first_complete",
		MaxCandidates:      1,
		PreferSessionBound: true,
	}
	matchedRules := []string{}
	for _, rule := range cfg.RouteRules {
		if !rule.Enabled {
			continue
		}
		if !matchesRule(rule.Match, req) {
			continue
		}
		matchedRules = append(matchedRules, rule.ID)
		if rule.Priority >= 0 {
			policy = rule.Policy
			if policy.MaxCandidates <= 0 {
				policy.MaxCandidates = 1
			}
		}
	}

	providerMap := map[string]models.Provider{}
	for _, provider := range cfg.Providers {
		if provider.Enabled {
			providerMap[provider.ID] = provider
		}
	}

	candidates := make([]models.Candidate, 0, len(logicalModel.Targets))
	for _, target := range logicalModel.Targets {
		provider, ok := providerMap[target.ProviderID]
		if !ok {
			continue
		}
		if len(policy.ProviderIDs) > 0 && !slices.Contains(policy.ProviderIDs, provider.ID) {
			continue
		}
		candidate := models.Candidate{Provider: provider, Target: target, Score: provider.Priority + target.Priority}
		candidates = append(candidates, candidate)
	}

	if len(candidates) == 0 {
		return models.RouteDecision{}, fmt.Errorf("no providers available for logical model %q", logicalModel.ID)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	if binding != nil && binding.ProviderID != "" && policy.PreferSessionBound {
		boundIdx := slices.IndexFunc(candidates, func(item models.Candidate) bool {
			return item.Provider.ID == binding.ProviderID && item.Target.Model == binding.ProviderModel
		})
		if boundIdx >= 0 {
			boundCandidate := candidates[boundIdx]
			candidates = append([]models.Candidate{boundCandidate}, append(candidates[:boundIdx], candidates[boundIdx+1:]...)...)
			matchedRules = append(matchedRules, "session-binding")
		}
	}

	maxCandidates := policy.MaxCandidates
	if maxCandidates > len(candidates) {
		maxCandidates = len(candidates)
	}
	if maxCandidates <= 0 {
		maxCandidates = 1
	}

	reason := "default policy"
	if len(matchedRules) > 0 {
		reason = "matched route rules: " + strings.Join(matchedRules, ", ")
	}

	return models.RouteDecision{
		MatchedRuleIDs: matchedRules,
		Reason:         reason,
		Policy:         policy,
		Candidates:     candidates[:maxCandidates],
	}, nil
}

func matchesRule(match models.RouteMatch, req models.NormalizedRequest) bool {
	if match.LogicalModel != "" && match.LogicalModel != req.LogicalModel {
		return false
	}
	if match.ProjectID != "" && match.ProjectID != req.ProjectID {
		return false
	}
	if match.MinChars > 0 {
		total := 0
		for _, message := range req.Messages {
			total += len(message.Content)
		}
		if total < match.MinChars {
			return false
		}
	}
	return true
}
