package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenFastModeSettlementCacheAndFallback(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	usage := &dto.Usage{PromptTokens: 1000, CompletionTokens: 200,
		PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 200}}
	info := &relaycommon.RelayInfo{
		FastMode: true, FastModeRequested: true, OriginModelName: "gpt-6.1-sol", StartTime: time.Now(),
		ChannelMeta: &relaycommon.ChannelMeta{},
		PriceData: types.PriceData{ModelRatio: 1, CompletionRatio: 2, CacheRatio: 0.1,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}},
	}
	info.PriceData.AddOtherRatio("fast_mode", 1.5)
	assert.Equal(t, 1830, calculateTextQuotaSummary(ctx, info, usage).Quota)
	info.ObserveFastModeTier([]byte(`{"service_tier":"default"}`))
	assert.Equal(t, 1220, calculateTextQuotaSummary(ctx, info, usage).Quota)
	other := GenerateTextOtherInfo(ctx, info, 1, 1, 2, 200, 0.1, 0, 0)
	assert.Equal(t, 1.0, other["fast_mode_ratio"])
	assert.Equal(t, "default", other["service_tier"])
	info.ChannelSetting.TrustSub2APIFastProtocol = true
	header := make(http.Header)
	header.Set("X-Sub2API-Fast-Protocol", "codex-oauth-priority-v1")
	info.ObserveFastModeProtocol(header)
	info.ObserveFastModeTier([]byte(`{"service_tier":"default"}`))
	assert.Equal(t, 1830, calculateTextQuotaSummary(ctx, info, usage).Quota)
	other = GenerateTextOtherInfo(ctx, info, 1, 1, 2, 200, 0.1, 0, 0)
	assert.Equal(t, 1.5, other["fast_mode_ratio"])
	assert.Equal(t, "default", other["service_tier"], "retain the raw upstream tier")
	assert.Equal(t, "codex-oauth-priority-v1", other["fast_mode_billing_protocol"])
}

func TestTokenFastModeTieredSettlementDoesNotStack(t *testing.T) {
	info := makeRelayInfo(`tier("standard", p * 2 + c * 10)`, 1, 1000, 100)
	info.FastMode = true
	ok, quota, result := TryTieredSettle(info, BuildTieredTokenParams(&dto.Usage{PromptTokens: 1000, CompletionTokens: 100}, false, nil))
	assert.True(t, ok)
	assert.Equal(t, 2250, quota)
	assert.Equal(t, 2250.0, result.ActualQuotaBeforeGroup)
	_, again, _ := TryTieredSettle(info, BuildTieredTokenParams(&dto.Usage{PromptTokens: 1000, CompletionTokens: 100}, false, nil))
	assert.Equal(t, quota, again)
	info.FastModeTier = "default"
	_, fallback, _ := TryTieredSettle(info, BuildTieredTokenParams(&dto.Usage{PromptTokens: 1000, CompletionTokens: 100}, false, nil))
	assert.Equal(t, 1500, fallback)
}

func TestTokenFastModeWalletReserveSettleAndRefund(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "settle", true: "upstream failure refund"}[fail], func(t *testing.T) {
			truncate(t)
			const userID = 730
			seedUser(t, userID, 10000)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := &relaycommon.RelayInfo{UserId: userID, IsPlayground: true, ForcePreConsume: true,
				UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
				PriceData:   types.PriceData{GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}}}
			require.Nil(t, PreConsumeBilling(ctx, 1500, info))
			balance, err := model.GetUserQuota(userID, false)
			require.NoError(t, err)
			assert.Equal(t, 8500, balance)
			if fail {
				info.Billing.Refund(ctx)
				require.Eventually(t, func() bool {
					balance, err := model.GetUserQuota(userID, false)
					return err == nil && balance == 10000
				}, 3*time.Second, 10*time.Millisecond)
			} else {
				require.NoError(t, SettleBilling(ctx, info, 1830))
				balance, err = model.GetUserQuota(userID, false)
				require.NoError(t, err)
				assert.Equal(t, 8170, balance)
			}
		})
	}
}
