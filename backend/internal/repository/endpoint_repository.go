package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/endpoint"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type endpointRepository struct {
	client *dbent.Client
}

// NewEndpointRepository 创建 ent 实现。
func NewEndpointRepository(client *dbent.Client) service.EndpointRepository {
	return &endpointRepository{client: client}
}

func (r *endpointRepository) Create(ctx context.Context, m *service.DBEndpoint) error {
	client := clientFromContext(ctx, r.client)
	now := time.Now().UTC()

	builder := client.Endpoint.Create().
		SetAccountID(m.AccountID).
		SetStableID(m.StableID).
		SetOutboundProtocol(m.OutboundProtocol).
		SetBaseURL(m.BaseURL).
		SetCreatedAt(now).
		SetUpdatedAt(now)

	if m.AuthHeader != "" {
		builder.SetAuthHeader(m.AuthHeader)
	}
	if m.AuthScheme != "" {
		builder.SetAuthScheme(m.AuthScheme)
	}
	if m.ModelsSource != "" {
		builder.SetModelsSource(m.ModelsSource)
	}
	if m.Priority != 0 {
		builder.SetPriority(m.Priority)
	}
	if m.Health != "" {
		builder.SetHealth(m.Health)
	}
	if len(m.Capabilities) > 0 {
		builder.SetCapabilities(m.Capabilities)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, errEndpointStableIDConflict)
	}
	m.ID = created.ID
	m.AuthHeader = created.AuthHeader
	m.AuthScheme = created.AuthScheme
	m.ModelsSource = created.ModelsSource
	m.Priority = created.Priority
	m.Health = created.Health
	m.CreatedAt = created.CreatedAt
	m.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *endpointRepository) Update(ctx context.Context, m *service.DBEndpoint) error {
	client := clientFromContext(ctx, r.client)

	builder := client.Endpoint.UpdateOneID(m.ID).
		SetAccountID(m.AccountID).
		SetStableID(m.StableID).
		SetOutboundProtocol(m.OutboundProtocol).
		SetBaseURL(m.BaseURL).
		SetAuthHeader(m.AuthHeader).
		SetAuthScheme(m.AuthScheme).
		SetModelsSource(m.ModelsSource).
		SetPriority(m.Priority).
		SetHealth(m.Health).
		SetUpdatedAt(time.Now().UTC())

	if len(m.Capabilities) > 0 {
		builder.SetCapabilities(m.Capabilities)
	} else {
		builder.ClearCapabilities()
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, errEndpointNotFound, errEndpointStableIDConflict)
	}
	m.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *endpointRepository) Delete(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	return translatePersistenceError(
		client.Endpoint.DeleteOneID(id).Exec(ctx),
		errEndpointNotFound,
		nil,
	)
}

func (r *endpointRepository) GetByID(ctx context.Context, id int64) (*service.DBEndpoint, error) {
	m, err := clientFromContext(ctx, r.client).Endpoint.Get(ctx, id)
	if err != nil {
		return nil, translatePersistenceError(err, errEndpointNotFound, nil)
	}
	return endpointEntityToService(m), nil
}

func (r *endpointRepository) ListByAccountID(ctx context.Context, accountID int64) ([]*service.DBEndpoint, error) {
	items, err := r.client.Endpoint.Query().
		Where(endpoint.AccountIDEQ(accountID)).
		Order(dbent.Asc(endpoint.FieldPriority), dbent.Asc(endpoint.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("endpoint list by account_id: %w", err)
	}
	out := make([]*service.DBEndpoint, 0, len(items))
	for _, item := range items {
		out = append(out, endpointEntityToService(item))
	}
	return out, nil
}

func (r *endpointRepository) FindByStableID(ctx context.Context, accountID int64, stableID string) (*service.DBEndpoint, error) {
	rows, err := r.client.Endpoint.Query().
		Where(
			endpoint.AccountIDEQ(accountID),
			endpoint.StableIDEQ(stableID),
		).
		Limit(1).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("endpoint find by stable_id: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return endpointEntityToService(rows[0]), nil
}

// --- helpers ---

var (
	errEndpointNotFound      = infraerrors.New(404, "ENDPOINT_NOT_FOUND", "endpoint not found")
	errEndpointStableIDConflict = infraerrors.New(409, "ENDPOINT_STABLE_ID_CONFLICT", "endpoint stable_id already exists for this account")
)

func endpointEntityToService(m *dbent.Endpoint) *service.DBEndpoint {
	if m == nil {
		return nil
	}
	return &service.DBEndpoint{
		ID:               m.ID,
		AccountID:        m.AccountID,
		StableID:         m.StableID,
		OutboundProtocol: m.OutboundProtocol,
		BaseURL:          m.BaseURL,
		AuthHeader:       m.AuthHeader,
		AuthScheme:       m.AuthScheme,
		ModelsSource:     m.ModelsSource,
		Priority:         m.Priority,
		Health:           m.Health,
		Capabilities:     m.Capabilities,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}
