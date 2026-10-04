package service

import (
	"net/http"
	"net/http/httptest"
	"sync"
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

func setupImageStudioFunding(t *testing.T, wallet int, preference string) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ImageStudioCredit{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.SubscriptionPreConsumeRecord{}))
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		require.NoError(t, sqlDB.Close())
	})
	seedUser(t, 861001, wallet)
	credit, granted, err := model.ClaimImageStudioCredit(861001)
	require.NoError(t, err)
	require.True(t, granted)
	require.Equal(t, 500000, credit.Quota)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", nil)
	c.Set("image_studio_authenticated", true)
	info := &relaycommon.RelayInfo{
		UserId: 861001, IsPlayground: true, TokenUnlimited: true,
		UserSetting: dto.UserSetting{BillingPreference: preference},
		RequestId:   t.Name(), OriginModelName: "fixture-chat",
	}
	return c, info
}

func TestImageStudioCreditOneTimeConcurrentAndIsolated(t *testing.T) {
	_, _ = setupImageStudioFunding(t, 100, "wallet_only")
	var wg sync.WaitGroup
	amounts := make(chan int, 2)
	grants := make(chan bool, 2)
	failures := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, granted, err := model.ClaimImageStudioCredit(861002)
			if err != nil {
				failures <- err
				return
			}
			grants <- granted
			amount, err := model.ReserveImageStudioCredit(861002, 400000)
			if err != nil {
				failures <- err
				return
			}
			amounts <- amount
		}()
	}
	wg.Wait()
	close(failures)
	close(grants)
	close(amounts)
	for err := range failures {
		require.NoError(t, err)
	}
	total, grantCount := 0, 0
	for amount := range amounts {
		total += amount
	}
	for granted := range grants {
		if granted {
			grantCount++
		}
	}
	assert.Equal(t, 500000, total)
	assert.Equal(t, 1, grantCount)
	credit, granted, err := model.ClaimImageStudioCredit(861002)
	require.NoError(t, err)
	assert.False(t, granted)
	assert.Zero(t, credit.Quota)
	other, err := model.GetImageStudioCredit(861001)
	require.NoError(t, err)
	assert.Equal(t, 500000, other.Quota)
	assert.Equal(t, 100, getUserQuota(t, 861001))
	require.Error(t, model.RefundImageStudioCredit(861001, 1))
}

func TestImageStudioGiftPriorityAndSettlement(t *testing.T) {
	for _, tc := range []struct {
		name                                  string
		wallet, pre, bump, actual, gift, paid int
	}{
		{"gift_only_no_wallet", 0, 100000, 0, 50000, 50000, 0},
		{"exact_precharge", 200000, 100000, 0, 100000, 100000, 0},
		{"mixed", 200000, 600000, 0, 650000, 500000, 150000},
		{"refund_paid_first", 200000, 600000, 0, 200000, 200000, 0},
		{"reserve_more", 200000, 100000, 650000, 600000, 500000, 100000},
		{"late_wallet_debt", 0, 100000, 0, 550000, 500000, 50000},
		{"free_request", 0, 0, 0, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, info := setupImageStudioFunding(t, tc.wallet, "wallet_only")
			require.Nil(t, PreConsumeBilling(c, tc.pre, info))
			if tc.bump > 0 {
				require.NoError(t, info.Billing.Reserve(tc.bump))
			}
			require.NoError(t, info.Billing.Settle(tc.actual))
			require.NoError(t, info.Billing.Settle(tc.actual))
			assert.False(t, info.Billing.NeedsRefund())
			credit, err := model.GetImageStudioCredit(info.UserId)
			require.NoError(t, err)
			assert.Equal(t, 500000-tc.gift, credit.Quota)
			assert.Equal(t, tc.wallet-tc.paid, getUserQuota(t, info.UserId))
			other := map[string]interface{}{}
			appendBillingInfo(info, other)
			assert.Equal(t, tc.gift, other["studio_gift_consumed"])
			assert.Equal(t, tc.paid, other["wallet_quota_deducted"])
		})
	}
}

func TestImageStudioRefundAndExhaustion(t *testing.T) {
	c, info := setupImageStudioFunding(t, 200000, "wallet_only")
	require.NotNil(t, PreConsumeBilling(c, 800000, info))
	credit, err := model.GetImageStudioCredit(info.UserId)
	require.NoError(t, err)
	assert.Equal(t, 500000, credit.Quota)
	require.Nil(t, PreConsumeBilling(c, 600000, info))
	require.NoError(t, info.Billing.Reserve(650000))
	info.Billing.Refund(c)
	info.Billing.Refund(c)
	require.Eventually(t, func() bool {
		credit, err := model.GetImageStudioCredit(info.UserId)
		return err == nil && credit.Quota == 500000 && getUserQuota(t, info.UserId) == 200000
	}, time.Second, time.Millisecond)
	info.Billing = nil
	require.Nil(t, PreConsumeBilling(c, 600000, info))
	require.NoError(t, info.Billing.Settle(650000))
	next := *info
	next.Billing = nil
	require.Nil(t, PreConsumeBilling(c, 40000, &next))
	assert.Equal(t, BillingSourceWallet, next.BillingSource)
	require.NoError(t, next.Billing.Settle(40000))
	assert.Equal(t, 10000, getUserQuota(t, info.UserId))
	credit, granted, err := model.ClaimImageStudioCredit(info.UserId)
	require.NoError(t, err)
	assert.False(t, granted)
	assert.Zero(t, credit.Quota)
}

func TestImageStudioGiftRequiresSignedBridgeAndSupportedRoute(t *testing.T) {
	for _, tc := range []struct {
		path                               string
		authenticated, playground, allowed bool
	}{
		{"/pg/chat/completions", false, true, false},
		{"/v1/chat/completions", true, false, false},
		{"/pg/other", true, true, false},
		{"/pg/chat/completions", true, true, true},
		{"/pg/responses", true, true, true},
		{"/pg/images/generations", true, true, true},
		{"/pg/images/edits", true, true, true},
		{"/pg/audio/speech", true, true, true},
		{"/pg/audio/transcriptions", true, true, true},
		{"/pg/audio/translations", true, true, true},
	} {
		t.Run(tc.path, func(t *testing.T) {
			c, info := setupImageStudioFunding(t, 0, "wallet_only")
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, nil)
			c.Request.Header.Set("X-Image-Studio-Authenticated", "true")
			c.Set("image_studio_authenticated", tc.authenticated)
			info.IsPlayground = tc.playground
			err := PreConsumeBilling(c, 100, info)
			if tc.allowed {
				require.Nil(t, err)
				require.NoError(t, info.Billing.Settle(100))
			} else {
				require.NotNil(t, err)
			}
			credit, creditErr := model.GetImageStudioCredit(info.UserId)
			require.NoError(t, creditErr)
			expected := 500000
			if tc.allowed {
				expected -= 100
			}
			assert.Equal(t, expected, credit.Quota)
		})
	}
}

func TestImageStudioGiftPreservesSubscriptionAndRefundRetry(t *testing.T) {
	c, info := setupImageStudioFunding(t, 200000, "subscription_only")
	plan := &model.SubscriptionPlan{Id: 861001, Title: "Fixture", TotalAmount: 200000}
	require.NoError(t, model.DB.Create(plan).Error)
	seedSubscription(t, 861001, 861001, 200000, 0)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("id = ?", 861001).Update("plan_id", plan.Id).Error)
	require.Nil(t, PreConsumeBilling(c, 600000, info))
	require.NoError(t, info.Billing.Reserve(650000))
	require.NoError(t, info.Billing.Settle(620000))
	assert.Equal(t, int64(120000), getSubscriptionUsed(t, 861001))
	assert.Equal(t, 200000, getUserQuota(t, info.UserId))
	assert.Equal(t, BillingSourceSubscription, info.ImageStudioFallbackSource)
	other := map[string]interface{}{}
	appendBillingInfo(info, other)
	assert.Equal(t, int64(120000), other["subscription_consumed"])
	assert.Equal(t, 0, other["wallet_quota_deducted"])
}

func TestImageStudioGiftSettlementRefundFailureIsIdempotent(t *testing.T) {
	c, info := setupImageStudioFunding(t, 200000, "wallet_only")
	require.Nil(t, PreConsumeBilling(c, 600000, info))
	require.NoError(t, model.DB.Exec(`CREATE TRIGGER fail_studio_refund BEFORE UPDATE ON image_studio_credits
		WHEN NEW.quota > OLD.quota BEGIN SELECT RAISE(ABORT, 'fixture refund failure'); END`).Error)
	t.Cleanup(func() { model.DB.Exec("DROP TRIGGER IF EXISTS fail_studio_refund") })
	require.Error(t, info.Billing.Settle(200000))
	assert.Equal(t, 200000, getUserQuota(t, info.UserId))
	require.NoError(t, model.DB.Exec("DROP TRIGGER fail_studio_refund").Error)
	require.NoError(t, info.Billing.Settle(200000))
	credit, err := model.GetImageStudioCredit(info.UserId)
	require.NoError(t, err)
	assert.Equal(t, 300000, credit.Quota)
	assert.Equal(t, 200000, getUserQuota(t, info.UserId))
}
