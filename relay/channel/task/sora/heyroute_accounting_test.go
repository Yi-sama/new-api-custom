package sora

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHeyRouteResolutionWalletChargeAndFailureRefund(t *testing.T) {
	configureHeyRouteTestPrices(t)
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldRedis, oldBatch, oldLog := common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = false, false, true
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = oldRedis, oldBatch, oldLog
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Task{}, &model.Log{}))

	for index, tc := range []struct {
		resolution string
		charge     int
	}{
		{"480p", 1000000}, // 8 seconds * 0.25 CNY * 500000 quota/CNY.
		{"720p", 1400000},
	} {
		t.Run(tc.resolution, func(t *testing.T) {
			id := index + 810
			const initial = 10000000
			require.NoError(t, db.Create(&model.User{Id: id, Username: tc.resolution, AffCode: tc.resolution, Quota: initial}).Error)
			token := &model.Token{Id: id, UserId: id, Key: tc.resolution, Name: "video-test", RemainQuota: initial}
			require.NoError(t, db.Create(token).Error)
			require.NoError(t, db.Create(&model.Channel{Id: id, Name: tc.resolution, Key: "test"}).Error)
			payload, err := common.Marshal(map[string]any{
				"model": "grok-imagine-video", "prompt": "test", "seconds": 8, "resolution": tc.resolution,
			})
			require.NoError(t, err)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(payload))
			c.Request.Header.Set("Content-Type", "application/json")
			defer common.CleanupBodyStorage(c)
			info := &relaycommon.RelayInfo{
				UserId: id, TokenId: id, TokenKey: token.Key,
				OriginModelName: "grok-imagine-video", UsingGroup: "default", ForcePreConsume: true,
				UserSetting:   dto.UserSetting{BillingPreference: "wallet_only"},
				ChannelMeta:   &relaycommon.ChannelMeta{ChannelId: id, ChannelBaseUrl: "https://cn.heyroute.ai"},
				TaskRelayInfo: &relaycommon.TaskRelayInfo{},
			}
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
			info.PriceData, err = helper.ModelPriceHelperPerCall(c, info)
			require.NoError(t, err)
			for name, ratio := range adaptor.EstimateBilling(c, info) {
				info.PriceData.AddOtherRatio(name, ratio)
			}
			info.PriceData.Quota = common.QuotaFromFloat(info.PriceData.ApplyOtherRatiosToFloat(float64(info.PriceData.Quota)))
			assert.InDelta(t, tc.charge, info.PriceData.Quota, 1)
			require.Nil(t, service.PreConsumeBilling(c, info.PriceData.Quota, info))
			require.NoError(t, service.SettleBilling(c, info, info.PriceData.Quota))
			service.LogTaskConsumption(c, info)
			var user model.User
			require.NoError(t, db.First(&user, id).Error)
			assert.InDelta(t, tc.charge, initial-user.Quota, 1)
			require.NoError(t, db.First(token, id).Error)
			assert.Equal(t, initial-info.PriceData.Quota, token.RemainQuota)

			task := &model.Task{
				TaskID: "video-" + tc.resolution, UserId: id, ChannelId: id, Quota: info.PriceData.Quota,
				Group: "default", PrivateData: model.TaskPrivateData{
					TokenId: id, BillingSource: service.BillingSourceWallet,
				},
			}
			require.NoError(t, db.Create(task).Error)
			require.True(t, service.RefundTaskQuota(context.Background(), task, "mock upstream failure"))
			require.NoError(t, db.First(&user, id).Error)
			require.NoError(t, db.First(token, id).Error)
			assert.Equal(t, initial, user.Quota)
			assert.Equal(t, initial, token.RemainQuota)
			assert.Zero(t, user.UsedQuota)
			assert.Zero(t, token.UsedQuota)
		})
	}
}
