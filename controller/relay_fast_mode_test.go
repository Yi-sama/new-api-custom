package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fastModeBillingSettler struct {
	preConsumedQuota int
	reserveTargets   []int
}

func (*fastModeBillingSettler) Settle(int) error { return nil }

func (*fastModeBillingSettler) Refund(*gin.Context) {}

func (*fastModeBillingSettler) NeedsRefund() bool { return false }

func (s *fastModeBillingSettler) GetPreConsumedQuota() int { return s.preConsumedQuota }

func (s *fastModeBillingSettler) Reserve(targetQuota int) error {
	s.reserveTargets = append(s.reserveTargets, targetQuota)
	if targetQuota > s.preConsumedQuota {
		s.preConsumedQuota = targetQuota
	}
	return nil
}

func TestFastModeKeepsInitialChannelAndReservesSelectedChannelUplift(t *testing.T) {
	originalGroups := setting.FastModeAllowedGroups2JSONString()
	t.Cleanup(func() { require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(originalGroups)) })
	require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(`["default"]`))
	originalModelRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalModelRatios))
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"gpt-6.1-sol":1}`))

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 721)
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(ctx, constant.ContextKeyChannelRatio, common.GetPointer(1.0))
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "gpt-6.1-sol")

	info := &relaycommon.RelayInfo{
		TokenFastMode:   true,
		RelayMode:       relayconstant.RelayModeResponses,
		OriginModelName: "gpt-6.1-sol",
		UserGroup:       "default",
		UsingGroup:      "default",
	}
	meta := &types.TokenCountMeta{MaxTokens: 1}
	initialPrice, err := helper.ModelPriceHelper(ctx, info, 1000, meta)
	require.NoError(t, err)
	assert.Nil(t, info.ChannelMeta, "initial pricing must preserve the distributor channel sentinel")
	assert.False(t, info.FastMode)

	billing := &fastModeBillingSettler{preConsumedQuota: initialPrice.QuotaToPreConsume}
	info.Billing = billing
	info.FinalPreConsumedQuota = initialPrice.QuotaToPreConsume
	selected, selectErr := getChannel(ctx, info, nil)
	require.Nil(t, selectErr)
	require.Equal(t, 721, selected.Id, "first attempt must use the distributor-selected channel")

	require.Nil(t, prepareSelectedChannelBilling(ctx, info, 1000, meta))
	assert.True(t, info.FastMode)
	assert.Equal(t, constant.ChannelTypeOpenAI, info.ChannelType)
	assert.Equal(t, 1.5, info.PriceData.OtherRatios()["fast_mode"])
	assert.Equal(t, 1501, info.PriceData.QuotaToPreConsume)
	assert.Equal(t, []int{1501}, billing.reserveTargets)
	assert.Equal(t, 1501, info.FinalPreConsumedQuota)

	// The next selected channel maps the requested model outside Fast support and
	// has a higher channel ratio. Retry pricing must use this attempt's context.
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 722)
	common.SetContextKey(ctx, constant.ContextKeyChannelRatio, common.GetPointer(2.0))
	ctx.Set("model_mapping", `{"gpt-6.1-sol":"claude-opus-5"}`)
	info.RetryIndex = 1
	require.Nil(t, prepareSelectedChannelBilling(ctx, info, 1000, meta))
	assert.Equal(t, 722, info.ChannelId)
	assert.Equal(t, "claude-opus-5", info.UpstreamModelName)
	assert.False(t, info.FastMode)
	assert.False(t, info.PriceData.HasOtherRatio("fast_mode"))
	assert.Equal(t, 2002, info.PriceData.QuotaToPreConsume)
	assert.Equal(t, []int{1501, 2002}, billing.reserveTargets)
	assert.Equal(t, 2002, info.FinalPreConsumedQuota)
}
