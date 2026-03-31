package tests

import "github.com/relayhub/relayhub/server/internal/models"

func normalizedProxyRequest() models.NormalizedRequest {
	return models.NormalizedRequest{
		EntryProtocol: "openai_chat",
		TaskType:      "chat",
		ProjectID:     "local-dev",
		APIKeyID:      "local-dev-key",
		SessionKey:    "race-session",
		LogicalModel:  "smart-fast",
		Messages: []models.Message{
			{Role: "user", Content: "trigger a race route"},
		},
	}
}
