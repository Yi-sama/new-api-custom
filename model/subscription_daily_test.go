package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const subscriptionDaySeconds = int64(24 * time.Hour / time.Second)

func prepareDailySubscriptionTest(t *testing.T) {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
	t.Cleanup(func() {
		DB.Exec("DELETE FROM subscription_pre_consume_records")
	})
}

func TestDailySubscriptionGrantScheduleCapsAtLifetimeTotal(t *testing.T) {
	start := int64(1_000_000)
	sub := &UserSubscription{
		AmountTotal:    60,
		QuotaGrantMode: SubscriptionQuotaGrantDaily,
		DailyAmount:    2,
		StartTime:      start,
		EndTime:        start + 30*subscriptionDaySeconds,
	}

	assert.EqualValues(t, 2, subscriptionGrantedAmount(sub, start))
	assert.EqualValues(t, 20, subscriptionGrantedAmount(sub, start+9*subscriptionDaySeconds))
	assert.EqualValues(t, 60, subscriptionGrantedAmount(sub, sub.EndTime-1))
	assert.EqualValues(t, 60, subscriptionGrantedAmount(sub, sub.EndTime))
	assert.Zero(t, nextSubscriptionGrantTime(sub, sub.EndTime-1))
}

func TestDailySubscriptionPreConsumeRetainAndRefund(t *testing.T) {
	prepareDailySubscriptionTest(t)
	now := GetDBTimestamp()

	active := &UserSubscription{
		UserId:            701,
		AmountTotal:       1_000_000,
		QuotaGrantMode:    SubscriptionQuotaGrantDaily,
		DailyAmount:       1_000_000,
		QuotaExpiryPolicy: SubscriptionQuotaExpiryExpire,
		StartTime:         now - 30,
		EndTime:           now + 30*subscriptionDaySeconds,
		Status:            "active",
	}
	require.NoError(t, DB.Create(active).Error)

	_, err := PreConsumeUserSubscription("daily-over-earned", active.UserId, "", 0, 1_000_001)
	require.Error(t, err)
	preConsumed, err := PreConsumeUserSubscription("daily-partial", active.UserId, "", 0, 500_000)
	require.NoError(t, err)
	assert.EqualValues(t, 1_000_000, preConsumed.AmountTotal)
	require.NoError(t, PostConsumeUserSubscriptionDelta(active.Id, 500_000))
	require.Error(t, PostConsumeUserSubscriptionDelta(active.Id, 1))

	refundable := &UserSubscription{
		UserId:            702,
		AmountTotal:       1_000_000,
		QuotaGrantMode:    SubscriptionQuotaGrantDaily,
		DailyAmount:       1_000_000,
		QuotaExpiryPolicy: SubscriptionQuotaExpiryExpire,
		StartTime:         now - 30,
		EndTime:           now + 30*subscriptionDaySeconds,
		Status:            "active",
	}
	require.NoError(t, DB.Create(refundable).Error)
	_, err = PreConsumeUserSubscription("daily-refund", refundable.UserId, "", 0, 400_000)
	require.NoError(t, err)
	require.NoError(t, RefundSubscriptionPreConsume("daily-refund"))
	require.NoError(t, RefundSubscriptionPreConsume("daily-refund"))
	assert.Zero(t, getSubscriptionResetSub(t, refundable.Id).AmountUsed)

	retained := &UserSubscription{
		UserId:            703,
		AmountTotal:       3_000_000,
		QuotaGrantMode:    SubscriptionQuotaGrantDaily,
		DailyAmount:       1_000_000,
		QuotaExpiryPolicy: SubscriptionQuotaExpiryRetain,
		StartTime:         now - 2*subscriptionDaySeconds,
		EndTime:           now - 1,
		Status:            "expired",
	}
	require.NoError(t, DB.Create(retained).Error)
	retainedPreConsume, err := PreConsumeUserSubscription("daily-retain", retained.UserId, "", 0, 1_000_000)
	require.NoError(t, err)
	assert.EqualValues(t, 2_000_000, retainedPreConsume.AmountTotal)

	expired := &UserSubscription{
		UserId:            704,
		AmountTotal:       3_000_000,
		QuotaGrantMode:    SubscriptionQuotaGrantDaily,
		DailyAmount:       1_000_000,
		QuotaExpiryPolicy: SubscriptionQuotaExpiryExpire,
		StartTime:         now - 2*subscriptionDaySeconds,
		EndTime:           now - 1,
		Status:            "expired",
	}
	cancelled := *retained
	cancelled.Id = 0
	cancelled.UserId = 705
	cancelled.Status = "cancelled"
	require.NoError(t, DB.Create(expired).Error)
	require.NoError(t, DB.Create(&cancelled).Error)
	_, err = PreConsumeUserSubscription("daily-expired", expired.UserId, "", 0, 1)
	require.Error(t, err)
	_, err = PreConsumeUserSubscription("daily-cancelled", cancelled.UserId, "", 0, 1)
	require.Error(t, err)

	assert.True(t, subscriptionIsTimeActiveOrRetained(active, now))
	active.AmountUsed = 1_000_000
	assert.True(t, subscriptionIsTimeActiveOrRetained(active, now))
	assert.False(t, subscriptionCanPreConsume(active, now))
	assert.True(t, subscriptionCanPreConsume(retained, now))
	assert.False(t, subscriptionCanPreConsume(&cancelled, now))
}

func TestDailySubscriptionSnapshotAndManualReset(t *testing.T) {
	prepareDailySubscriptionTest(t)
	user := &User{Username: "daily-snapshot", Password: "unused", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, DB.Create(user).Error)
	plan := &SubscriptionPlan{
		Title:                   "daily snapshot",
		DurationUnit:            SubscriptionDurationDay,
		DurationValue:           30,
		TotalAmount:             3_000_000,
		QuotaGrantMode:          SubscriptionQuotaGrantDaily,
		DailyAmount:             1_000_000,
		QuotaExpiryPolicy:       SubscriptionQuotaExpiryRetain,
		QuotaResetPeriod:        SubscriptionResetDaily,
		QuotaResetCustomSeconds: 60,
	}
	require.NoError(t, DB.Create(plan).Error)
	sub, err := CreateUserSubscriptionFromPlanTx(DB, user.Id, plan, "test")
	require.NoError(t, err)
	assert.Equal(t, SubscriptionQuotaGrantDaily, sub.QuotaGrantMode)
	assert.EqualValues(t, 1_000_000, sub.DailyAmount)
	assert.Equal(t, SubscriptionQuotaExpiryRetain, sub.QuotaExpiryPolicy)
	assert.Zero(t, sub.NextResetTime)

	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"quota_grant_mode":   SubscriptionQuotaGrantFull,
		"daily_amount":       int64(0),
		"quota_reset_period": SubscriptionResetDaily,
	}).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(123)).Error)
	_, err = AdminResetUserSubscriptionsByPlan(user.Id, plan.Id, true)
	require.Error(t, err)
	assert.EqualValues(t, 123, getSubscriptionResetSub(t, sub.Id).AmountUsed)
}
