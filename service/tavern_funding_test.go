package service

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTavernFunding(t *testing.T, wallet int, preference string) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TavernCredit{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.SubscriptionPreConsumeRecord{}))
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		require.NoError(t, sqlDB.Close())
	})
	seedUser(t, 851001, wallet)
	credit, granted, err := model.ClaimTavernCredit(851001)
	require.NoError(t, err)
	require.True(t, granted)
	require.Equal(t, 500000, credit.Quota)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", nil)
	c.Set("tavern_authenticated", true)
	info := &relaycommon.RelayInfo{
		UserId: 851001, IsPlayground: true, TokenUnlimited: true,
		UserSetting: dto.UserSetting{BillingPreference: preference},
		RequestId:   t.Name(), OriginModelName: "fixture-chat",
	}
	return c, info
}

func TestTavernCreditClaimAndConcurrentReserve(t *testing.T) {
	_, _ = setupTavernFunding(t, 100, "wallet_only")
	var wg sync.WaitGroup
	results := make(chan int, 2)
	failures := make(chan error, 2)
	grants := make(chan bool, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, granted, err := model.ClaimTavernCredit(851002)
			if err != nil {
				failures <- err
				return
			}
			grants <- granted
			reserved, err := model.ReserveTavernCredit(851002, 400000)
			if err != nil {
				failures <- err
				return
			}
			results <- reserved
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	close(grants)
	for err := range failures {
		require.NoError(t, err)
	}
	total := 0
	for quota := range results {
		total += quota
	}
	assert.Equal(t, 500000, total)
	grantCount := 0
	for granted := range grants {
		if granted {
			grantCount++
		}
	}
	assert.Equal(t, 1, grantCount)
	credit, granted, err := model.ClaimTavernCredit(851002)
	require.NoError(t, err)
	assert.False(t, granted)
	assert.Zero(t, credit.Quota)
	assert.Equal(t, 100, getUserQuota(t, 851001))
}

func TestTavernExhaustionFallsBackToWallet(t *testing.T) {
	c, info := setupTavernFunding(t, 200000, "wallet_only")
	require.Nil(t, PreConsumeBilling(c, 600000, info))
	require.NoError(t, info.Billing.Settle(650000))
	next := &relaycommon.RelayInfo{
		UserId: info.UserId, IsPlayground: true, TokenUnlimited: true,
		UserSetting: info.UserSetting, RequestId: "next-request",
	}
	require.Nil(t, PreConsumeBilling(c, 40000, next))
	assert.Equal(t, BillingSourceWallet, next.BillingSource)
	require.NoError(t, next.Billing.Settle(40000))
	assert.Equal(t, 10000, getUserQuota(t, info.UserId))
	credit, granted, err := model.ClaimTavernCredit(info.UserId)
	require.NoError(t, err)
	assert.False(t, granted)
	assert.Zero(t, credit.Quota)
}

func TestTavernSettlementSplitsAndRefunds(t *testing.T) {
	for _, tc := range []struct {
		name       string
		pre, bump  int
		actual     int
		gift, paid int
	}{
		{"gift_only_zero_wallet", 100000, 0, 50000, 50000, 0},
		{"exact_precharge", 100000, 0, 100000, 100000, 0},
		{"mixed", 600000, 0, 650000, 500000, 150000},
		{"refund_paid_first", 600000, 0, 200000, 200000, 0},
		{"reserve_more", 100000, 650000, 600000, 500000, 100000},
		{"late_wallet_debt", 100000, 0, 550000, 500000, 50000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wallet := 200000
			if tc.name == "gift_only_zero_wallet" || tc.name == "late_wallet_debt" {
				wallet = 0
			}
			c, info := setupTavernFunding(t, wallet, "wallet_only")
			require.Nil(t, PreConsumeBilling(c, tc.pre, info))
			if tc.bump > 0 {
				require.NoError(t, info.Billing.Reserve(tc.bump))
			}
			require.NoError(t, info.Billing.Settle(tc.actual))
			require.NoError(t, info.Billing.Settle(tc.actual))
			assert.False(t, info.Billing.NeedsRefund())
			credit, err := model.GetTavernCredit(info.UserId)
			require.NoError(t, err)
			assert.Equal(t, 500000-tc.gift, credit.Quota)
			assert.Equal(t, wallet-tc.paid, getUserQuota(t, info.UserId))
			other := map[string]interface{}{}
			appendBillingInfo(info, other)
			assert.Equal(t, tc.gift, other["tavern_gift_consumed"])
			assert.Equal(t, tc.paid, other["wallet_quota_deducted"])
		})
	}
}

func TestTavernFailedPrechargeAndRefund(t *testing.T) {
	c, info := setupTavernFunding(t, 200000, "wallet_only")
	require.NotNil(t, PreConsumeBilling(c, 800000, info))
	credit, err := model.GetTavernCredit(info.UserId)
	require.NoError(t, err)
	assert.Equal(t, 500000, credit.Quota)
	assert.Equal(t, 200000, getUserQuota(t, info.UserId))
	require.Nil(t, PreConsumeBilling(c, 600000, info))
	require.NoError(t, info.Billing.Reserve(650000))
	funding := info.Billing.(*BillingSession).funding
	require.NoError(t, funding.Refund())
	require.NoError(t, funding.Refund())
	credit, err = model.GetTavernCredit(info.UserId)
	require.NoError(t, err)
	assert.Equal(t, 500000, credit.Quota)
	assert.Equal(t, 200000, getUserQuota(t, info.UserId))
}

func TestTavernCreditCannotPayOtherEntrypoints(t *testing.T) {
	for _, tc := range []struct {
		name, path                string
		authenticated, playground bool
	}{
		{"regular_playground", "/pg/chat/completions", false, true},
		{"api_with_context_flag", "/v1/chat/completions", true, false},
		{"image_with_context_flag", "/pg/images/generations", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, info := setupTavernFunding(t, 0, "wallet_only")
			c.Set("tavern_authenticated", tc.authenticated)
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, nil)
			info.IsPlayground = tc.playground
			require.NotNil(t, PreConsumeBilling(c, 100, info))
			credit, err := model.GetTavernCredit(info.UserId)
			require.NoError(t, err)
			assert.Equal(t, 500000, credit.Quota)
		})
	}
}

func TestTavernPreservesSubscriptionPreference(t *testing.T) {
	c, info := setupTavernFunding(t, 200000, "subscription_only")
	plan := &model.SubscriptionPlan{Id: 851001, Title: "Fixture", TotalAmount: 200000}
	require.NoError(t, model.DB.Create(plan).Error)
	seedSubscription(t, 851001, 851001, 200000, 0)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("id = ?", 851001).
		Update("plan_id", plan.Id).Error)
	require.Nil(t, PreConsumeBilling(c, 600000, info))
	require.NoError(t, info.Billing.Reserve(650000))
	require.NoError(t, info.Billing.Settle(620000))
	assert.Equal(t, int64(120000), getSubscriptionUsed(t, 851001))
	assert.Equal(t, 200000, getUserQuota(t, info.UserId))
	assert.Equal(t, BillingSourceSubscription, info.TavernFallbackSource)
	other := map[string]interface{}{}
	appendBillingInfo(info, other)
	assert.Equal(t, int64(120000), other["subscription_consumed"])
	assert.Equal(t, 0, other["wallet_quota_deducted"])
}

func TestTavernAsyncFailureRefund(t *testing.T) {
	c, info := setupTavernFunding(t, 0, "wallet_only")
	require.Nil(t, PreConsumeBilling(c, 100000, info))
	assert.True(t, info.Billing.NeedsRefund())
	info.Billing.Refund(c)
	info.Billing.Refund(c)
	require.Eventually(t, func() bool {
		credit, err := model.GetTavernCredit(info.UserId)
		return err == nil && credit.Quota == common.QuotaFromFloat(common.QuotaPerUnit)
	}, time.Second, time.Millisecond)
	assert.Equal(t, 0, getUserQuota(t, info.UserId))
}

func TestTavernSettlementRefundFailureDoesNotDoubleRefundWallet(t *testing.T) {
	c, info := setupTavernFunding(t, 200000, "wallet_only")
	require.Nil(t, PreConsumeBilling(c, 600000, info))
	require.NoError(t, model.DB.Exec(`CREATE TRIGGER fail_tavern_refund BEFORE UPDATE ON tavern_credits
		WHEN NEW.quota > OLD.quota BEGIN SELECT RAISE(ABORT, 'fixture refund failure'); END`).Error)
	t.Cleanup(func() { model.DB.Exec("DROP TRIGGER IF EXISTS fail_tavern_refund") })
	require.Error(t, info.Billing.Settle(200000))
	assert.Equal(t, 200000, getUserQuota(t, info.UserId))
	require.NoError(t, model.DB.Exec("DROP TRIGGER fail_tavern_refund").Error)
	require.NoError(t, info.Billing.Settle(200000))
	credit, err := model.GetTavernCredit(info.UserId)
	require.NoError(t, err)
	assert.Equal(t, 300000, credit.Quota)
	assert.Equal(t, 200000, getUserQuota(t, info.UserId))
}
