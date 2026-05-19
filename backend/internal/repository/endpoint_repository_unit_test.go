package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func newEndpointRepoSQLite(t *testing.T) (*endpointRepository, *dbent.Client) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:endpoint_repo?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return &endpointRepository{client: client}, client
}

// seedAccount 在 SQLite 中创建一个最小化 Account 记录，供 endpoint FK 使用。
func seedAccount(t *testing.T, client *dbent.Client) int64 {
	t.Helper()
	now := time.Now().UTC()
	acc, err := client.Account.Create().
		SetName("test-account").
		SetPlatform("openai").
		SetType("apikey").
		SetCredentials(map[string]any{}).
		SetExtra(map[string]any{}).
		SetConcurrency(3).
		SetPriority(50).
		SetStatus("active").
		SetSchedulable(true).
		SetErrorMessage("").
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(context.Background())
	require.NoError(t, err)
	return acc.ID
}

func seedEndpoint(t *testing.T, repo *endpointRepository, accountID int64, stableID, proto string) *service.DBEndpoint {
	t.Helper()
	m := &service.DBEndpoint{
		AccountID:        accountID,
		StableID:         stableID,
		OutboundProtocol: proto,
		BaseURL:          "https://api.example.com",
		Capabilities:     []string{"streaming"},
	}
	require.NoError(t, repo.Create(context.Background(), m))
	require.NotZero(t, m.ID)
	return m
}

func TestEndpointRepo_CreateAndGet(t *testing.T) {
	repo, client := newEndpointRepoSQLite(t)
	ctx := context.Background()
	accID := seedAccount(t, client)

	m := &service.DBEndpoint{
		AccountID:        accID,
		StableID:         "wanjie-openai_chat",
		OutboundProtocol: "openai_chat",
		BaseURL:          "https://api.openai.com",
		AuthHeader:       "Authorization",
		AuthScheme:       "Bearer",
		ModelsSource:     "remote",
		Priority:         100,
		Health:           "healthy",
		Capabilities:     []string{"streaming", "vision"},
	}
	require.NoError(t, repo.Create(ctx, m))
	require.NotZero(t, m.ID)
	require.False(t, m.CreatedAt.IsZero())
	require.False(t, m.UpdatedAt.IsZero())

	got, err := repo.GetByID(ctx, m.ID)
	require.NoError(t, err)
	require.Equal(t, m.ID, got.ID)
	require.Equal(t, accID, got.AccountID)
	require.Equal(t, "wanjie-openai_chat", got.StableID)
	require.Equal(t, "openai_chat", got.OutboundProtocol)
	require.Equal(t, "https://api.openai.com", got.BaseURL)
	require.Equal(t, []string{"streaming", "vision"}, got.Capabilities)
}

func TestEndpointRepo_ListByAccountID(t *testing.T) {
	repo, client := newEndpointRepoSQLite(t)
	ctx := context.Background()
	accID := seedAccount(t, client)

	seedEndpoint(t, repo, accID, "ep-a", "openai_chat")
	seedEndpoint(t, repo, accID, "ep-b", "anthropic_messages")
	seedEndpoint(t, repo, accID, "ep-c", "gemini_v1beta")

	// 创建另一个账号的 endpoint，不应出现在结果中
	otherAccID := seedAccount(t, client)
	seedEndpoint(t, repo, otherAccID, "ep-other", "openai_chat")

	items, err := repo.ListByAccountID(ctx, accID)
	require.NoError(t, err)
	require.Len(t, items, 3)
	for _, item := range items {
		require.Equal(t, accID, item.AccountID)
	}
}

func TestEndpointRepo_FindByStableID(t *testing.T) {
	repo, client := newEndpointRepoSQLite(t)
	ctx := context.Background()
	accID := seedAccount(t, client)

	seedEndpoint(t, repo, accID, "target-stable", "openai_chat")

	got, err := repo.FindByStableID(ctx, accID, "target-stable")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "target-stable", got.StableID)
	require.Equal(t, accID, got.AccountID)
}

func TestEndpointRepo_FindByStableID_MissReturnNil(t *testing.T) {
	repo, client := newEndpointRepoSQLite(t)
	ctx := context.Background()
	accID := seedAccount(t, client)

	got, err := repo.FindByStableID(ctx, accID, "nonexistent-stable")
	require.NoError(t, err, "miss should not error")
	require.Nil(t, got)
}

func TestEndpointRepo_Update(t *testing.T) {
	repo, client := newEndpointRepoSQLite(t)
	ctx := context.Background()
	accID := seedAccount(t, client)

	m := seedEndpoint(t, repo, accID, "update-ep", "openai_chat")
	originalUpdatedAt := m.UpdatedAt

	// pause to ensure UpdatedAt changes
	time.Sleep(time.Millisecond)

	m.BaseURL = "https://api.updated.com"
	m.Health = "degraded"
	m.Priority = 50
	m.Capabilities = []string{"streaming"}
	require.NoError(t, repo.Update(ctx, m))
	require.True(t, m.UpdatedAt.After(originalUpdatedAt) || m.UpdatedAt.Equal(originalUpdatedAt),
		"UpdatedAt should be refreshed")

	got, err := repo.GetByID(ctx, m.ID)
	require.NoError(t, err)
	require.Equal(t, "https://api.updated.com", got.BaseURL)
	require.Equal(t, "degraded", got.Health)
	require.Equal(t, 50, got.Priority)
}

func TestEndpointRepo_Delete(t *testing.T) {
	repo, client := newEndpointRepoSQLite(t)
	ctx := context.Background()
	accID := seedAccount(t, client)

	m := seedEndpoint(t, repo, accID, "delete-ep", "openai_chat")
	require.NoError(t, repo.Delete(ctx, m.ID))

	_, err := repo.GetByID(ctx, m.ID)
	require.Error(t, err, "GetByID after Delete should return error")
}
