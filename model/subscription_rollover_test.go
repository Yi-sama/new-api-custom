package model

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSubscriptionRolloverWalletCacheAccounting(t *testing.T) {
	for _, test := range []struct {
		name    string
		cached  int
		failDB  bool
		wantErr bool
	}{
		{"atomic wallet debit", 5000, false, false},
		{"transaction rollback restores cache", 5000, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			prepareDailySubscriptionTest(t)
			server := useUserCacheMiniRedis(t)
			user := User{Username: "rollover-cache", Quota: 5000, AuthVersion: 1}
			require.NoError(t, DB.Create(&user).Error)
			require.NoError(t, populateUserCache(user))
			server.HSet(getUserCacheKey(user.Id), "Quota", strconv.Itoa(test.cached))
			plan := SubscriptionPlan{Title: "rollover", TotalAmount: 100, QuotaResetPeriod: "never"}
			require.NoError(t, DB.Create(&plan).Error)
			InvalidateSubscriptionPlanCache(plan.Id)
			now := GetDBTimestamp()
			sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: 100, AmountUsed: 80,
				Status: "active", StartTime: now - 10, EndTime: now + 86400, AllowWalletOverflow: true}
			require.NoError(t, DB.Create(&sub).Error)
			_, err := PreConsumeUserSubscription(t.Name(), user.Id, "", 0, 10)
			require.NoError(t, err)
			if test.failDB {
				const name = "rollover_test_failure"
				require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
					if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "user_subscriptions" {
						tx.AddError(errors.New("injected subscription update failure"))
					}
				}))
				defer DB.Callback().Update().Remove(name)
			}
			result, err := ResizeSubscriptionPreConsume(t.Name(), user.Id, 30, true, false)
			if test.wantErr {
				require.Error(t, err)
				assert.Equal(t, test.cached, mustRolloverCacheQuota(t, user.Id))
				var current UserSubscription
				require.NoError(t, DB.First(&current, sub.Id).Error)
				assert.EqualValues(t, 90, current.AmountUsed)
			} else {
				require.NoError(t, err)
				assert.Nil(t, result)
				assert.Equal(t, 4970, mustRolloverCacheQuota(t, user.Id))
			}
			var current User
			require.NoError(t, DB.First(&current, user.Id).Error)
			if test.wantErr {
				assert.Equal(t, 5000, current.Quota)
			} else {
				assert.Equal(t, 4970, current.Quota)
			}
		})
	}
}

func mustRolloverCacheQuota(t *testing.T, userID int) int {
	t.Helper()
	quota, err := common.RDB.HGet(context.Background(), getUserCacheKey(userID), "Quota").Int()
	require.NoError(t, err)
	return quota
}
