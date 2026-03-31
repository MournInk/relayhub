package session

import (
	"context"
	"time"

	"github.com/relayhub/relayhub/server/internal/models"
	"github.com/relayhub/relayhub/server/internal/storage"
)

type Manager struct {
	store *storage.SQLiteStore
}

func NewManager(store *storage.SQLiteStore) *Manager {
	return &Manager{store: store}
}

func (m *Manager) Get(ctx context.Context, projectID, sessionKey string) (models.SessionBinding, bool, error) {
	if sessionKey == "" {
		return models.SessionBinding{}, false, nil
	}
	return m.store.GetSessionBinding(ctx, projectID, sessionKey)
}

func (m *Manager) Bind(ctx context.Context, projectID, sessionKey, providerID, providerModel string) error {
	if sessionKey == "" {
		return nil
	}
	now := time.Now().UTC()
	return m.store.UpsertSessionBinding(ctx, models.SessionBinding{
		SessionKey:    sessionKey,
		ProjectID:     projectID,
		ProviderID:    providerID,
		ProviderModel: providerModel,
		BoundAt:       now,
		LastSeenAt:    now,
	})
}

func (m *Manager) Touch(ctx context.Context, projectID, sessionKey string) error {
	if sessionKey == "" {
		return nil
	}
	return m.store.TouchSession(ctx, projectID, sessionKey)
}
