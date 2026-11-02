package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func freeDailyPoolConfig(t *testing.T) model.DailyTokenPoolConfig {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.Option{}, &model.DailyTokenPoolDay{}, &model.DailyTokenPoolReservation{}))
	t.Cleanup(func() {
		require.NoError(t, model.DB.Where(&model.Option{Key: model.DailyTokenPoolOptionKey}).Delete(&model.Option{}).Error)
		require.NoError(t, model.DB.Where("1 = 1").Delete(&model.DailyTokenPoolReservation{}).Error)
		require.NoError(t, model.DB.Where("1 = 1").Delete(&model.DailyTokenPoolDay{}).Error)
	})
	config := model.DefaultDailyTokenPoolConfig()
	config.Enabled, config.FreeUsage = true, true
	config.Groups, config.ChannelIDs = []string{"shared"}, []int{86}
	require.NoError(t, model.SaveDailyTokenPoolConfig(config))
	return config
}

func TestFreeDailyTokenPoolLeavesWalletSubscriptionAndTokenQuotaUntouched(t *testing.T) {
	for _, balance := range []int{0, 10000} {
		t.Run(map[int]string{0: "zero wallet", 10000: "funded wallet"}[balance], func(t *testing.T) {
			truncate(t)
			config := freeDailyPoolConfig(t)
			seedUser(t, 840, balance)
			seedToken(t, 841, 840, "free-shared-test-key", 1000)
			seedSubscription(t, 842, 840, 10000, 700)
			seedChannel(t, 86)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "shared")
			common.SetContextKey(ctx, constant.ContextKeyChannelId, 86)
			limit := uint(10)
			info := &relaycommon.RelayInfo{
				UserId: 840, TokenId: 841, TokenKey: "free-shared-test-key", TokenGroup: "shared", UsingGroup: "shared",
				RelayFormat: types.RelayFormatOpenAI, OriginModelName: "gpt-4o-mini",
				Request:   &dto.GeneralOpenAIRequest{Model: "gpt-4o-mini", MaxTokens: &limit},
				StartTime: time.Now(), FirstResponseTime: time.Now(), ForcePreConsume: true,
				ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 86},
				UserSetting: dto.UserSetting{BillingPreference: "subscription_first"},
				PriceData: hosttypes.PriceData{ModelRatio: 1, CompletionRatio: 1,
					GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1}},
			}
			require.Nil(t, PrepareDailyTokenPoolBilling(ctx, info))
			require.True(t, info.DailyTokenPoolFree)
			require.Nil(t, PreConsumeBilling(ctx, 500, info))
			require.Nil(t, info.Billing)
			require.NoError(t, PreConsumeTokenQuota(info, 500))
			require.Nil(t, ReserveDailyTokenPool(ctx, info, &model.Channel{Id: 86}))
			PostTextConsumeQuota(ctx, info, &dto.Usage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10}, nil)
			// Secondary/legacy charging paths must not debit a free request either.
			require.NoError(t, PostConsumeQuota(info, 20, 0, true))

			var user model.User
			require.NoError(t, model.DB.First(&user, 840).Error)
			assert.Equal(t, balance, user.Quota)
			assert.Zero(t, user.UsedQuota)
			assert.Equal(t, 1, user.RequestCount)
			var token model.Token
			require.NoError(t, model.DB.First(&token, 841).Error)
			assert.Equal(t, 1000, token.RemainQuota)
			assert.Zero(t, token.UsedQuota)
			var sub model.UserSubscription
			require.NoError(t, model.DB.First(&sub, 842).Error)
			assert.Equal(t, int64(700), sub.AmountUsed)
			var channel model.Channel
			require.NoError(t, model.DB.First(&channel, 86).Error)
			assert.Zero(t, channel.UsedQuota)
			status, err := model.GetDailyTokenPoolStatus(config, time.Now())
			require.NoError(t, err)
			assert.Equal(t, int64(10), status.UsedTokens)
			assert.Zero(t, status.ReservedTokens)
			var log model.Log
			require.NoError(t, model.LOG_DB.Where("user_id = ?", 840).Last(&log).Error)
			assert.Zero(t, log.Quota)
			assert.Equal(t, 7, log.PromptTokens)
			assert.Equal(t, 3, log.CompletionTokens)
			var other map[string]interface{}
			require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
			assert.Equal(t, BillingSourceDailyTokenPool, other["billing_source"])
			assert.Equal(t, true, other["daily_token_pool_free"])
		})
	}
}

func TestFreeDailyTokenPoolFailsClosedOnDisabledScopeChangesAndAutoRouting(t *testing.T) {
	for _, scenario := range []string{"disabled", "channel mismatch", "retry leaves group", "free mode removed", "auto group"} {
		t.Run(scenario, func(t *testing.T) {
			config := freeDailyPoolConfig(t)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "shared")
			common.SetContextKey(ctx, constant.ContextKeyChannelId, 86)
			info := &relaycommon.RelayInfo{TokenGroup: "shared"}
			require.Nil(t, PrepareDailyTokenPoolBilling(ctx, info))
			status := http.StatusForbidden
			switch scenario {
			case "disabled":
				config.Enabled = false
				status = http.StatusServiceUnavailable
			case "channel mismatch":
				common.SetContextKey(ctx, constant.ContextKeyChannelId, 87)
			case "retry leaves group":
				common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "paid")
			case "free mode removed":
				config.FreeUsage = false
			case "auto group":
				info = &relaycommon.RelayInfo{TokenGroup: "auto"}
			}
			require.NoError(t, model.SaveDailyTokenPoolConfig(config))
			apiErr := PrepareDailyTokenPoolBilling(ctx, info)
			require.NotNil(t, apiErr)
			assert.Equal(t, status, apiErr.StatusCode)
			assert.Nil(t, info.Billing)
		})
	}
}
