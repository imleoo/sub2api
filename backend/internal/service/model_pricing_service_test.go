//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type modelPricingServiceRepoStub struct {
	ModelPricingRepository
	filter ModelPricingListFilter
}

func (s *modelPricingServiceRepoStub) List(ctx context.Context, filter ModelPricingListFilter) ([]*DBModelPricing, int, error) {
	s.filter = filter
	return []*DBModelPricing{{ModelID: "gpt-5.2"}}, 1, nil
}

func TestModelPricingService_ListPassesVisibleOnlyFilter(t *testing.T) {
	repo := &modelPricingServiceRepoStub{}
	svc := NewModelPricingService(repo)

	items, total, err := svc.List(context.Background(), ModelPricingListFilter{VisibleOnly: true})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, 1, total)
	require.True(t, repo.filter.VisibleOnly)
}
