package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoPricingUsesLiveResolutionPrices(t *testing.T) {
	before := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(before)) })
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"grok-imagine-video":0.25,"grok-imagine-video-480p":0.25,"grok-imagine-video-720p":0.35}`))
	meta := &Model{Parameters: `{"video":{"currency":"CNY","billing_unit":"second","durations":[8,6,10],"resolutions":["480p","720p"]}}`}
	pricing := buildVideoPricing("grok-imagine-video", meta)
	require.NotNil(t, pricing)
	assert.Equal(t, "CNY", pricing.Currency)
	assert.Equal(t, []int{8, 6, 10}, pricing.DurationSeconds)
	assert.Equal(t, []VideoResolutionPrice{
		{Resolution: "480p", PricePerSecond: 0.25},
		{Resolution: "720p", PricePerSecond: 0.35},
	}, pricing.Prices)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"grok-imagine-video":0.25,"grok-imagine-video-480p":0.25,"grok-imagine-video-720p":0.4}`))
	assert.Equal(t, 0.4, buildVideoPricing("grok-imagine-video", meta).Prices[1].PricePerSecond)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"grok-imagine-video":0.25,"grok-imagine-video-480p":0.25}`))
	assert.Nil(t, buildVideoPricing("grok-imagine-video", meta), "never reuse the 480p rate for missing 720p pricing")
	assert.Nil(t, buildVideoPricing("fixed-image", &Model{}))
}

func TestVideoPricingExposesPerRequestResolutionRates(t *testing.T) {
	before := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(before)) })
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"seedance-fast":0.8,"seedance-fast-720p":0.8,"seedance-fast-1080p":1.2,"seedance-fast-2k":1.5}`))
	meta := &Model{Parameters: `{"video":{"currency":"CNY","billing_unit":"request","durations":[4,8,12],"resolutions":["720p","1080p","2k"]}}`}
	pricing := buildVideoPricing("seedance-fast", meta)
	require.NotNil(t, pricing, "fixed-request videos still need explicit currency and resolution prices")
	assert.Equal(t, "request", pricing.Unit)
	assert.Equal(t, "CNY", pricing.Currency)
	raw, err := common.Marshal(pricing)
	require.NoError(t, err)
	assert.JSONEq(t, `{"currency":"CNY","unit":"request","duration_seconds":[4,8,12],"default_resolution":"720p","prices":[{"resolution":"720p","price_per_request":0.8},{"resolution":"1080p","price_per_request":1.2},{"resolution":"2k","price_per_request":1.5}]}`, string(raw))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"seedance-fast":0.8,"seedance-fast-720p":0.8}`))
	assert.Nil(t, buildVideoPricing("seedance-fast", meta), "a missing resolution must not appear free or reuse the base price")
}

func TestPricingCatalogIncludesVideoResolutionRates(t *testing.T) {
	resetPricingEndpointTestTables(t)
	before := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(before)) })
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"grok-imagine-video":0.25,"grok-imagine-video-480p":0.25,"grok-imagine-video-720p":0.35}`))
	insertPricingEndpointChannel(t, 910, constant.ChannelTypeOpenAI, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 910, "grok-imagine-video")
	require.NoError(t, DB.Create(&Model{
		ModelName: "grok-imagine-video", Status: 1,
		Parameters: `{"video":{"currency":"CNY","billing_unit":"second","durations":[8,6,10],"resolutions":["480p","720p"]}}`,
	}).Error)
	prices := GetPricing()
	require.Len(t, prices, 1)
	require.NotNil(t, prices[0].VideoPricing)
	assert.Equal(t, 0.35, prices[0].VideoPricing.Prices[1].PricePerSecond)
}

func TestPricingCatalogIncludesRequestVideoRates(t *testing.T) {
	resetPricingEndpointTestTables(t)
	before := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(before)) })
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"seedance-mini":0.6}`))
	insertPricingEndpointChannel(t, 910, constant.ChannelTypeOpenAI, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 910, "seedance-mini")
	require.NoError(t, DB.Create(&Model{
		ModelName: "seedance-mini", Status: 1,
		Parameters: `{"video":{"currency":"CNY","billing_unit":"request","durations":[5,10],"resolutions":["720p"]}}`,
	}).Error)
	prices := GetPricing()
	require.Len(t, prices, 1)
	require.NotNil(t, prices[0].VideoPricing)
	assert.Equal(t, "request", prices[0].VideoPricing.Unit)
	assert.Equal(t, 0.6, prices[0].VideoPricing.Prices[0].PricePerRequest)
}
