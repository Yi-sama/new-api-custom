package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfitabilityPoolSeparatesAdminCallsFromPaidProfit(t *testing.T) {
	setupProfitabilityBreakdownTest(t, `
		{"request_id":"customer","account_id":11,"total_cost":80,"created_at":"1970-01-01T00:01:50Z"},
		{"request_id":"admin","account_id":12,"total_cost":20,"created_at":"1970-01-01T00:01:51Z"}`)
	require.NoError(t, DB.Create(&Channel{Id: 7, Name: "pool"}).Error)
	require.NoError(t, DB.Create(&[]ProfitabilityEntry{
		{EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 7, AccountCount: 2, AmountRmbCents: 200, OccurredAt: 100},
		{EntryType: ProfitabilityEntryAdminAdjustment, UserId: 2, Quota: 500_000, OccurredAt: 100},
	}).Error)
	require.NoError(t, DB.Create(&[]Log{
		{UserId: 2, Type: LogTypeConsume, ChannelId: 7, Group: "default", Quota: 2_000_000, UpstreamRequestId: "customer", CreatedAt: 110},
		{UserId: 1, Type: LogTypeConsume, ChannelId: 7, Group: "admin-only", Quota: 500_000, UpstreamRequestId: "admin", CreatedAt: 111},
	}).Error)

	report, err := GetProfitabilityReport(100, 250)
	require.NoError(t, err)
	assert.Equal(t, 5.0, report.Pool.AllCalls.ChargeUsd)
	require.NotNil(t, report.Pool.AllCalls.SubUsageUsd)
	assert.Equal(t, 100.0, *report.Pool.AllCalls.SubUsageUsd)
	require.NotNil(t, report.Pool.AllCalls.EffectiveRatio)
	assert.InDelta(t, 0.05, *report.Pool.AllCalls.EffectiveRatio, 1e-9)
	assert.Equal(t, 4.0, report.Pool.CustomerCalls.ChargeUsd)
	require.NotNil(t, report.Pool.CustomerCalls.SubUsageUsd)
	assert.Equal(t, 80.0, *report.Pool.CustomerCalls.SubUsageUsd)
	require.NotNil(t, report.Pool.CustomerCalls.EffectiveRatio)
	assert.InDelta(t, 0.05, *report.Pool.CustomerCalls.EffectiveRatio, 1e-9)
	assert.Equal(t, 1.0, report.Pool.AdminChargeUsd)
	assert.Equal(t, 20.0, report.Pool.MatchedAdminSubUsageUsd)
	assert.Equal(t, 3.0, report.CallChargeRmb)
	require.NotNil(t, report.Pool.ProfitRmb)
	assert.Equal(t, 1.0, *report.Pool.ProfitRmb)
}

func TestProfitabilityPoolKeepsFullUsageWithoutGuessingUnassignedRoles(t *testing.T) {
	setupProfitabilityBreakdownTest(t, `
		{"request_id":"customer","account_id":11,"total_cost":80,"created_at":"1970-01-01T00:01:50Z"},
		{"request_id":"","account_id":12,"total_cost":20,"created_at":"1970-01-01T00:01:51Z"}`)
	require.NoError(t, DB.Create(&Channel{Id: 7, Name: "pool"}).Error)
	require.NoError(t, DB.Create(&ProfitabilityEntry{
		EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 7, AccountCount: 1, AmountRmbCents: 200, OccurredAt: 100,
	}).Error)
	require.NoError(t, DB.Create(&Log{
		UserId: 2, Type: LogTypeConsume, ChannelId: 7, Quota: 2_500_000, UpstreamRequestId: "customer", CreatedAt: 110,
	}).Error)

	report, err := GetProfitabilityReportWithOptions(100, 250, ProfitabilityReportOptions{
		ChannelSelectionPresent: true, ChannelIDs: []int{7},
	})
	require.NoError(t, err)
	require.NotNil(t, report.Pool.AllCalls.SubUsageUsd)
	assert.Equal(t, 100.0, *report.Pool.AllCalls.SubUsageUsd)
	require.NotNil(t, report.Pool.AllCalls.EffectiveRatio)
	assert.InDelta(t, 0.05, *report.Pool.AllCalls.EffectiveRatio, 1e-9)
	assert.Equal(t, "unverified_scope", report.Pool.AllCalls.Status)
	assert.Equal(t, "unmatched", report.Pool.CustomerCalls.Status)
	assert.Nil(t, report.Pool.CustomerCalls.SubUsageUsd)
	assert.Nil(t, report.Pool.CustomerCalls.EffectiveRatio)
	require.NotNil(t, report.Pool.ProfitRmb)
	assert.Equal(t, 3.0, *report.Pool.ProfitRmb)
}

func TestProfitabilityPoolDoesNotDividePartialChannelsByWholePool(t *testing.T) {
	setupProfitabilityBreakdownTest(t, `
		{"request_id":"selected","account_id":11,"total_cost":80,"created_at":"1970-01-01T00:01:50Z"},
		{"request_id":"outside","account_id":12,"total_cost":20,"created_at":"1970-01-01T00:01:51Z"}`)
	require.NoError(t, DB.Create(&[]Channel{{Id: 7, Name: "selected"}, {Id: 8, Name: "outside"}}).Error)
	require.NoError(t, DB.Create(&[]Log{
		{UserId: 2, Type: LogTypeConsume, ChannelId: 7, Quota: 2_000_000, UpstreamRequestId: "selected", CreatedAt: 110},
		{UserId: 1, Type: LogTypeConsume, ChannelId: 8, Quota: 500_000, UpstreamRequestId: "outside", CreatedAt: 111},
	}).Error)

	report, err := GetProfitabilityReportWithOptions(100, 250, ProfitabilityReportOptions{ChannelSelectionPresent: true, ChannelIDs: []int{7}})
	require.NoError(t, err)
	assert.Equal(t, 4.0, report.Pool.AllCalls.ChargeUsd)
	assert.Equal(t, 20.0, report.Pool.OutsideSelectedSubUsageUsd)
	assert.Equal(t, "scope_mismatch", report.Pool.AllCalls.Status)
	assert.Nil(t, report.Pool.AllCalls.EffectiveRatio)
	assert.Nil(t, report.Pool.CustomerCalls.EffectiveRatio)
	assert.Nil(t, report.Pool.ProfitRmb)
	assert.Equal(t, "missing_purchase", report.Pool.ProfitStatus)
}

func TestProfitabilityPoolUnavailableAndZeroUsageDoNotProduceRatios(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		t.Run(map[bool]string{false: "zero usage", true: "not configured"}[unavailable], func(t *testing.T) {
			setupProfitabilityBreakdownTest(t, ``)
			if unavailable {
				t.Setenv("SUB2API_BASE_URL", "")
			}
			report, err := GetProfitabilityReport(100, 250)
			require.NoError(t, err)
			assert.Nil(t, report.Pool.AllCalls.EffectiveRatio)
			assert.Nil(t, report.Pool.CustomerCalls.EffectiveRatio)
			if unavailable {
				assert.Nil(t, report.Pool.AllCalls.SubUsageUsd)
				assert.Equal(t, "not_configured", report.Pool.AllCalls.Status)
			} else {
				require.NotNil(t, report.Pool.AllCalls.SubUsageUsd)
				assert.Zero(t, *report.Pool.AllCalls.SubUsageUsd)
				assert.Equal(t, "no_usage", report.Pool.AllCalls.Status)
			}
		})
	}
}

func TestProfitabilityUsageWithoutAdminsDoesNotCountCustomersAsSelf(t *testing.T) {
	setupProfitabilityBreakdownTest(t, ``)
	require.NoError(t, DB.Create(&Log{UserId: 2, Type: LogTypeConsume, Quota: int(common.QuotaPerUnit), CreatedAt: 110}).Error)
	usage, err := getProfitabilityUsage(100, 250, nil, 0)
	require.NoError(t, err)
	assert.Equal(t, 1.0, usage.CustomerUsd)
	assert.Zero(t, usage.SelfUsd)
}

func TestProfitabilityPoolProfitUsesPaidIncomeAndFullPurchaseEvenWithoutSub(t *testing.T) {
	setupProfitabilityBreakdownTest(t, ``)
	t.Setenv("SUB2API_BASE_URL", "")
	require.NoError(t, DB.Create(&Channel{Id: 7, Name: "pool"}).Error)
	require.NoError(t, DB.Create(&ProfitabilityEntry{
		EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 7, AccountCount: 1, AmountRmbCents: 200, OccurredAt: 100,
	}).Error)
	require.NoError(t, DB.Create(&[]Log{
		{UserId: 2, Type: LogTypeConsume, ChannelId: 7, Quota: 2_500_000, CreatedAt: 110},
		{UserId: 1, Type: LogTypeConsume, ChannelId: 7, Quota: 500_000, CreatedAt: 111},
	}).Error)
	require.NoError(t, DB.Create(&TopUp{UserId: 2, Money: 4, Status: common.TopUpStatusSuccess, CompleteTime: 100}).Error)

	for _, tc := range []struct {
		basis  string
		profit float64
	}{
		{ProfitabilityRevenueBasisCalls, 3},
		{ProfitabilityRevenueBasisCash, 2},
	} {
		t.Run(tc.basis, func(t *testing.T) {
			report, err := GetProfitabilityReportWithOptions(100, 250, ProfitabilityReportOptions{RevenueBasis: tc.basis})
			require.NoError(t, err)
			assert.Equal(t, 6.0, report.Pool.AllCalls.ChargeUsd)
			assert.Nil(t, report.Pool.AllCalls.SubUsageUsd)
			assert.Nil(t, report.Pool.CustomerCalls.SubUsageUsd)
			require.NotNil(t, report.Pool.ProfitRmb)
			assert.Equal(t, tc.profit, *report.Pool.ProfitRmb)
			assert.Equal(t, "complete", report.Pool.ProfitStatus)
		})
	}
}

func TestProfitabilityPoolDeletedAdminIsStillSelfUsage(t *testing.T) {
	setupProfitabilityBreakdownTest(t, `{"request_id":"admin","account_id":11,"total_cost":10,"created_at":"1970-01-01T00:01:50Z"}`)
	require.NoError(t, DB.Create(&Channel{Id: 7, Name: "pool"}).Error)
	require.NoError(t, DB.Create(&Log{UserId: 1, Type: LogTypeConsume, ChannelId: 7, Quota: 500_000, UpstreamRequestId: "admin", CreatedAt: 110}).Error)
	require.NoError(t, DB.Delete(&User{}, 1).Error)

	report, err := GetProfitabilityReport(100, 250)
	require.NoError(t, err)
	assert.Equal(t, 1.0, report.Pool.AdminChargeUsd)
	assert.Zero(t, report.Pool.CustomerCalls.ChargeUsd)
	assert.Zero(t, report.CallChargeRmb)
	require.NotNil(t, report.Pool.CustomerCalls.SubUsageUsd)
	assert.Zero(t, *report.Pool.CustomerCalls.SubUsageUsd)
}

func TestProfitabilityPoolCashNeverUsesOtherProviderChargesForRatio(t *testing.T) {
	setupProfitabilityBreakdownTest(t, `{"request_id":"sub","account_id":11,"total_cost":10,"created_at":"1970-01-01T00:01:50Z"}`)
	require.NoError(t, DB.Create(&[]Channel{{Id: 7, Name: "Sub"}, {Id: 8, Name: "other provider"}}).Error)
	require.NoError(t, DB.Create(&[]Log{
		{UserId: 2, Type: LogTypeConsume, ChannelId: 7, Quota: 500_000, UpstreamRequestId: "sub", CreatedAt: 110},
		{UserId: 2, Type: LogTypeConsume, ChannelId: 8, Quota: 5_000_000, CreatedAt: 111},
	}).Error)

	report, err := GetProfitabilityReportWithOptions(100, 250, ProfitabilityReportOptions{RevenueBasis: ProfitabilityRevenueBasisCash})
	require.NoError(t, err)
	assert.Equal(t, 11.0, report.Pool.AllCalls.ChargeUsd)
	require.NotNil(t, report.Pool.AllCalls.SubUsageUsd)
	assert.Equal(t, 10.0, *report.Pool.AllCalls.SubUsageUsd)
	assert.Equal(t, "cash_scope", report.Pool.AllCalls.Status)
	assert.Equal(t, "cash_scope", report.Pool.CustomerCalls.Status)
	assert.Nil(t, report.Pool.AllCalls.EffectiveRatio)
	assert.Nil(t, report.Pool.CustomerCalls.EffectiveRatio)
}
