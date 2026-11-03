package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAccrualBilling(t *testing.T, start, end int64, policy, status string, used int64) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	oldDB, oldLogDB := model.DB, model.LOG_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.WalletPreConsumeRecord{}, &model.SubscriptionPlan{},
		&model.UserSubscription{}, &model.SubscriptionPreConsumeRecord{},
	))
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		require.NoError(t, sqlDB.Close())
	})
	seedUser(t, 620001, 5000)
	require.NoError(t, db.Create(&model.SubscriptionPlan{
		Id: 620101, Title: "Daily billing fixture",
		DurationUnit: "day", DurationValue: 30,
		TotalAmount: 3000, QuotaResetPeriod: "never",
		QuotaGrantMode: "daily", DailyAmount: 100, QuotaExpiryPolicy: policy,
	}).Error)
	model.InvalidateSubscriptionPlanCache(620101)
	require.NoError(t, db.Create(&model.UserSubscription{
		Id: 620201, UserId: 620001, PlanId: 620101,
		StartTime: start, EndTime: end, Status: status,
		AmountTotal: 3000, AmountUsed: used,
		QuotaGrantMode: "daily", DailyAmount: 100, QuotaExpiryPolicy: policy,
	}).Error)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/pg/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		UserId: 620001, RequestId: t.Name(), OriginModelName: "fixture-chat",
		IsPlayground: true, TokenUnlimited: true, ForcePreConsume: true,
		UserSetting: dto.UserSetting{BillingPreference: "subscription_only"},
	}
	return ctx, info
}

func TestDailyAccrualBillingReservesOnlyEarnedQuotaAndSettlesActualUsage(t *testing.T) {
	now := time.Now().Unix()
	start := now - 9*86400
	ctx, info := setupAccrualBilling(t, start, start+30*86400, "expire", "active", 750)

	session, apiErr := newRegularBillingSession(ctx, info, 200, false)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, BillingSourceSubscription, info.BillingSource)
	assert.EqualValues(t, 950, getSubscriptionUsed(t, 620201))
	assert.EqualValues(t, 1000, info.SubscriptionAmountTotal)
	assert.Equal(t, 5000, getUserQuota(t, 620001))

	require.Error(t, session.Reserve(251))
	assert.EqualValues(t, 950, getSubscriptionUsed(t, 620201))
	require.NoError(t, session.Settle(100))
	assert.EqualValues(t, 850, getSubscriptionUsed(t, 620201))
	assert.Equal(t, 5000, getUserQuota(t, 620001))
}

func TestDailyAccrualBillingExpiryPolicyControlsSubscriptionFunding(t *testing.T) {
	now := time.Now().Unix()
	start := now - 31*86400
	for _, test := range []struct {
		name, policy, status string
		spendable            bool
	}{
		{"retained balance", "retain", "expired", true},
		{"expiry clears access", "expire", "expired", false},
		{"cancellation clears access", "retain", "cancelled", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, info := setupAccrualBilling(t, start, start+30*86400, test.policy, test.status, 2800)
			session, apiErr := newRegularBillingSession(ctx, info, 100, false)
			if !test.spendable {
				require.NotNil(t, apiErr)
				assert.Nil(t, session)
				assert.EqualValues(t, 2800, getSubscriptionUsed(t, 620201))
				assert.Equal(t, 5000, getUserQuota(t, 620001))
				return
			}
			require.Nil(t, apiErr)
			require.NotNil(t, session)
			assert.EqualValues(t, 2900, getSubscriptionUsed(t, 620201))
			require.NoError(t, session.funding.Refund())
			require.NoError(t, session.funding.Refund())
			assert.EqualValues(t, 2800, getSubscriptionUsed(t, 620201))
			assert.Equal(t, 5000, getUserQuota(t, 620001))
		})
	}
}

func TestDailyAccrualBillingExhaustionDoesNotBypassWalletOverflowPolicy(t *testing.T) {
	now := time.Now().Unix()
	start := now - 9*86400
	ctx, info := setupAccrualBilling(t, start, start+30*86400, "expire", "active", 1000)
	info.UserSetting.BillingPreference = "subscription_first"

	session, apiErr := newRegularBillingSession(ctx, info, 200, false)

	require.NotNil(t, apiErr)
	assert.Nil(t, session)
	assert.EqualValues(t, 1000, getSubscriptionUsed(t, 620201))
	assert.Equal(t, 5000, getUserQuota(t, 620001))
}

func TestDailyAccrualBillingSettlesAuthorizedRequestAcrossExpiry(t *testing.T) {
	now := time.Now().Unix()
	start := now - 9*86400 - 3600
	ctx, info := setupAccrualBilling(t, start, now+86400, "expire", "active", 750)
	session, apiErr := newRegularBillingSession(ctx, info, 200, false)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).
		Where("id = ?", 620201).Update("end_time", now-1).Error)

	require.NoError(t, session.Settle(210))

	assert.EqualValues(t, 960, getSubscriptionUsed(t, 620201))
	assert.Equal(t, 5000, getUserQuota(t, 620001))
}
