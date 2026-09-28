package controller

import (
	"math"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelRatioValidation(t *testing.T) {
	for _, value := range []float64{-1, math.NaN(), math.Inf(1)} {
		require.Error(t, validateChannel(&model.Channel{ChannelRatio: &value}, false))
	}
	for _, value := range []*float64{nil, common.GetPointer(0.0), common.GetPointer(0.15)} {
		require.NoError(t, validateChannel(&model.Channel{ChannelRatio: value}, false))
	}
}

func TestChannelRatioUpdateAPIHandlesValueOmissionAndNull(t *testing.T) {
	oldDB, oldLogDB := model.DB, model.LOG_DB
	t.Cleanup(func() { model.DB, model.LOG_DB = oldDB, oldLogDB })
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	ch := &model.Channel{Name: "ratio-api", Key: "test", Type: 1, Models: "gpt-4o", Group: "default"}
	require.NoError(t, ch.Insert())
	engine := gin.New()
	engine.PUT("/api/channel", func(c *gin.Context) {
		c.Set("role", common.RoleRootUser)
		c.Set("id", 1)
		UpdateChannel(c)
	})
	for _, body := range []string{
		`{"id":1,"channel_ratio":0.15}`,
		`{"id":1,"name":"renamed"}`,
		`{"id":1,"channel_ratio":0}`,
		`{"id":1,"channel_ratio":null}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/api/channel", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(rec, req)
		var result struct {
			Success bool          `json:"success"`
			Message string        `json:"message"`
			Data    model.Channel `json:"data"`
		}
		require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &result))
		require.True(t, result.Success, result.Message)
		stored, err := model.GetChannelById(ch.Id, false)
		require.NoError(t, err)
		require.Equal(t, stored.ChannelRatio, result.Data.ChannelRatio)
		switch {
		case strings.Contains(body, "null"):
			require.Nil(t, stored.ChannelRatio)
		case strings.Contains(body, `"channel_ratio":0}`):
			require.NotNil(t, stored.ChannelRatio)
			require.Zero(t, *stored.ChannelRatio)
		default:
			require.NotNil(t, stored.ChannelRatio)
			require.Equal(t, 0.15, *stored.ChannelRatio)
		}
	}
}

func TestChannelRatioGroupAPIKeepsNamesAndAddsRange(t *testing.T) {
	oldDB, oldLogDB := model.DB, model.LOG_DB
	t.Cleanup(func() { model.DB, model.LOG_DB = oldDB, oldLogDB })
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&[]model.Channel{
		{Name: "A", Group: "default", ChannelRatio: common.GetPointer(0.15)},
		{Name: "B", Group: "default", ChannelRatio: common.GetPointer(0.18)},
	}).Error)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	GetGroups(ctx)
	var result struct {
		Success bool                                 `json:"success"`
		Data    []string                             `json:"data"`
		Ranges  map[string]service.ChannelRatioRange `json:"ratio_ranges"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &result))
	require.True(t, result.Success)
	require.Contains(t, result.Data, "default")
	require.Equal(t, service.ChannelRatioRange{Min: 0.15, Max: 0.18}, result.Ranges["default"])
}

func TestChannelRatioRetryReservationAndWalletSettlement(t *testing.T) {
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldPrice := ratio_setting.ModelPrice2JSONString()
	oldRedis, oldBatch := common.RedisEnabled, common.BatchUpdateEnabled
	oldQuotaPerUnit := common.QuotaPerUnit
	common.RedisEnabled, common.BatchUpdateEnabled = false, false
	common.QuotaPerUnit = 500000
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.RedisEnabled, common.BatchUpdateEnabled = oldRedis, oldBatch
		common.QuotaPerUnit = oldQuotaPerUnit
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(oldPrice))
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.User{}))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"channel-retry-test":0.01}`))

	for i, scenario := range []struct {
		name         string
		initial      float64
		retry        float64
		wantReserved int
		wantFinal    int
	}{
		{"paid_to_higher", 0.15, 0.18, 900, 900},
		{"paid_to_lower", 0.18, 0.15, 900, 750},
		{"free_to_paid", 0, 0.18, 900, 900},
		{"paid_to_free", 0.18, 0, 900, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			id := i + 900
			require.NoError(t, db.Create(&model.User{Id: id, Username: scenario.name, AffCode: scenario.name, Quota: 10000}).Error)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := &relaycommon.RelayInfo{
				UserId: id, UserGroup: "regular", UsingGroup: "default",
				OriginModelName: "channel-retry-test", IsPlayground: true, ForcePreConsume: true,
				UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
			}
			common.SetContextKey(ctx, constant.ContextKeyChannelRatio, common.GetPointer(scenario.initial))
			price, err := helper.ModelPriceHelper(ctx, info, 0, &types.TokenCountMeta{})
			require.NoError(t, err)
			if !price.FreeModel {
				require.Nil(t, service.PreConsumeBilling(ctx, price.QuotaToPreConsume, info))
			}
			common.SetContextKey(ctx, constant.ContextKeyChannelRatio, common.GetPointer(scenario.retry))
			info.RetryIndex = 1
			require.Nil(t, prepareSelectedChannelBilling(ctx, info, 0, &types.TokenCountMeta{}))
			require.Equal(t, scenario.retry, info.PriceData.GroupRatioInfo.GroupRatio)
			require.NotNil(t, info.Billing)
			require.Equal(t, scenario.wantReserved, info.Billing.GetPreConsumedQuota())
			require.Nil(t, service.SettleBilling(ctx, info, scenario.wantFinal))
			var user model.User
			require.NoError(t, db.First(&user, id).Error)
			require.Equal(t, 10000-scenario.wantFinal, user.Quota)
			other := service.GenerateMjOtherInfo(info, info.PriceData)
			require.Equal(t, scenario.retry, other["channel_ratio"])
			require.Equal(t, scenario.retry, other["effective_ratio"])
		})
	}
}

func TestChannelRatioIsSensitiveOnlyWhenChanged(t *testing.T) {
	origin := &model.Channel{ChannelRatio: common.GetPointer(0.15)}
	patch := &PatchChannel{Channel: model.Channel{ChannelRatio: common.GetPointer(0.15)}}
	require.False(t, channelHasSensitiveChanges(patch, origin, map[string]any{"channel_ratio": 0.15}))
	patch.ChannelRatio = nil
	require.True(t, channelHasSensitiveChanges(patch, origin, map[string]any{"channel_ratio": nil}))
}

func TestFirstChannelSelectionPreservesInitiallyPricedChannel(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 123)
	common.SetContextKey(ctx, constant.ContextKeyChannelRatio, common.GetPointer(0.15))
	info := relaycommon.GenRelayInfoOpenAI(ctx, nil)
	channel, apiErr := getChannel(ctx, info, nil)
	require.Nil(t, apiErr)
	require.Equal(t, 123, channel.Id)
	ratio, ok := common.GetContextKeyType[*float64](ctx, constant.ContextKeyChannelRatio)
	require.True(t, ok)
	require.Equal(t, 0.15, *ratio)
}
