//go:build unit

package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/ent/modelpricing"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNormalizePricingUnit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected modelpricing.PricingUnit
	}{
		{"token", service.ModelPricingUnitToken, modelpricing.PricingUnitToken},
		{"second", service.ModelPricingUnitSecond, modelpricing.PricingUnitSecond},
		{"image_generation", service.ModelPricingUnitImage, modelpricing.PricingUnitImageGeneration},
		{"video_generation", service.ModelPricingUnitVideo, modelpricing.PricingUnitVideoGeneration},
		{"空字符串回退 token", "", modelpricing.PricingUnitToken},
		{"非法值回退 token", "per-request", modelpricing.PricingUnitToken},
		{"前后空白裁剪后合法", "  second  ", modelpricing.PricingUnitSecond},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, normalizePricingUnit(tc.input))
		})
	}
}

func TestMarshalUnmarshalTierPricing_RoundTrip(t *testing.T) {
	tiers := []service.VideoPriceTier{
		{Spec: "在线推理-无声视频", CNYPerMToken: 12.5},
		{Spec: "在线推理-1080p-输入包含视频", CNYPerMToken: 30},
	}

	marshaled := marshalTierPricing(tiers)
	require.NotNil(t, marshaled)

	got := unmarshalTierPricing(marshaled)
	require.Equal(t, tiers, got)
}

func TestMarshalTierPricing_EmptyReturnsNil(t *testing.T) {
	require.Nil(t, marshalTierPricing(nil))
	require.Nil(t, marshalTierPricing([]service.VideoPriceTier{}))
}

func TestUnmarshalTierPricing_NilOrBlankReturnsNil(t *testing.T) {
	require.Nil(t, unmarshalTierPricing(nil))

	blank := "   "
	require.Nil(t, unmarshalTierPricing(&blank))
}

func TestUnmarshalTierPricing_MalformedJSONReturnsNil(t *testing.T) {
	malformed := "{not-valid-json"
	require.Nil(t, unmarshalTierPricing(&malformed))
}
