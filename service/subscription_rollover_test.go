package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSubscriptionRollover(t *testing.T, nextSubscription bool, preference string, overflow bool) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	now := time.Now().Unix()
	c, info := setupAccrualBilling(t, now-86400, now+30*86400, "expire", "active", 80)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("id = ?", 620201).
		Updates(map[string]interface{}{
			"quota_grant_mode": "full", "amount_total": 100, "allow_wallet_overflow": overflow,
		}).Error)
	require.NoError(t, model.DB.AutoMigrate(&model.Token{}))
	seedToken(t, 620301, info.UserId, "rollover-token", 1000)
	info.IsPlayground = false
	info.TokenId = 620301
	info.TokenKey = "rollover-token"
	info.TokenUnlimited = false
	info.UserSetting.BillingPreference = preference
	if nextSubscription {
		require.NoError(t, model.DB.Create(&model.UserSubscription{
			Id: 620202, UserId: info.UserId, PlanId: 620101,
			AmountTotal: 200, Status: "active", StartTime: now - 86400, EndTime: now + 31*86400,
			AllowWalletOverflow: overflow, QuotaGrantMode: "full",
		}).Error)
	}
	return c, info
}

func TestSubscriptionRolloverReserveAndSettle(t *testing.T) {
	for _, next := range []bool{true, false} {
		for _, reserve := range []bool{true, false} {
			name := "wallet"
			if next {
				name = "next-subscription"
			}
			if reserve {
				name += "/reserve"
			} else {
				name += "/settle"
			}
			t.Run(name, func(t *testing.T) {
				c, info := setupSubscriptionRollover(t, next, "subscription_first", true)
				session, apiErr := newRegularBillingSession(c, info, 10, false)
				require.Nil(t, apiErr)
				require.EqualValues(t, 90, getSubscriptionUsed(t, 620201))
				if reserve {
					require.NoError(t, session.Reserve(30))
					assert.Equal(t, 30, session.GetPreConsumedQuota())
				}
				require.NoError(t, session.Settle(35))
				require.NoError(t, session.Settle(35), "settlement must be idempotent")
				assert.False(t, session.NeedsRefund())
				assert.EqualValues(t, 80, getSubscriptionUsed(t, 620201), "release the original hold, not its unused entitlement")
				if next {
					assert.EqualValues(t, 35, getSubscriptionUsed(t, 620202))
					assert.Equal(t, 5000, getUserQuota(t, info.UserId))
					assert.Equal(t, BillingSourceSubscription, info.BillingSource)
					assert.Equal(t, 620202, info.SubscriptionId)
					assert.EqualValues(t, 35, info.SubscriptionPreConsumed+info.SubscriptionPostDelta)
					assert.EqualValues(t, 35, info.SubscriptionAmountUsedAfterPreConsume+info.SubscriptionPostDelta)
				} else {
					assert.Equal(t, 4965, getUserQuota(t, info.UserId))
					assert.Equal(t, BillingSourceWallet, info.BillingSource)
					assert.Zero(t, info.SubscriptionId)
				}
				assert.Equal(t, 965, getTokenRemainQuota(t, info.TokenId))
			})
		}
	}
}

func TestSubscriptionRolloverRefundsEntireMovedReservation(t *testing.T) {
	for _, next := range []bool{true, false} {
		t.Run(map[bool]string{true: "subscription", false: "wallet"}[next], func(t *testing.T) {
			c, info := setupSubscriptionRollover(t, next, "subscription_first", true)
			session, apiErr := newRegularBillingSession(c, info, 10, false)
			require.Nil(t, apiErr)
			require.NoError(t, session.Reserve(30))
			require.NoError(t, session.Reserve(40))
			require.NoError(t, session.funding.Refund())
			assert.EqualValues(t, 80, getSubscriptionUsed(t, 620201))
			if next {
				require.NoError(t, session.funding.Refund())
				assert.Zero(t, getSubscriptionUsed(t, 620202))
			}
			assert.Equal(t, 5000, getUserQuota(t, info.UserId))
		})
	}
}

func TestSubscriptionRolloverFailurePreservesOriginalReservation(t *testing.T) {
	for _, test := range []struct {
		name       string
		preference string
		overflow   bool
		wallet     int
		token      int
	}{
		{"subscription only", "subscription_only", true, 5000, 990},
		{"wallet overflow disabled", "subscription_first", false, 5000, 990},
		{"wallet insufficient", "subscription_first", true, 5, 990},
		{"token insufficient", "subscription_first", true, 5000, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, info := setupSubscriptionRollover(t, false, test.preference, test.overflow)
			session, apiErr := newRegularBillingSession(c, info, 10, false)
			require.Nil(t, apiErr)
			require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", info.UserId).Update("quota", test.wallet).Error)
			require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", info.TokenId).Update("remain_quota", test.token).Error)
			require.Error(t, session.Reserve(30))
			assert.Equal(t, 10, session.GetPreConsumedQuota())
			assert.EqualValues(t, 90, getSubscriptionUsed(t, 620201))
			assert.Equal(t, test.wallet, getUserQuota(t, info.UserId))
			assert.Equal(t, test.token, getTokenRemainQuota(t, info.TokenId))
			require.NoError(t, session.Settle(5))
			assert.EqualValues(t, 85, getSubscriptionUsed(t, 620201))
		})
	}
}
