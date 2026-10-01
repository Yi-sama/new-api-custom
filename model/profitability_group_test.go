package model

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfitabilityGroupBreakEvenUsesHistoricalRatesNotCurrentConfiguration(t *testing.T) {
	setupProfitabilityBreakdownTest(t, `
		{"request_id":"standard-old","account_id":11,"total_cost":100,"created_at":"1970-01-01T00:01:50Z"},
		{"request_id":"standard-new","account_id":11,"total_cost":100,"created_at":"1970-01-01T00:01:51Z"},
		{"request_id":"premium","account_id":12,"total_cost":800,"created_at":"1970-01-01T00:01:52Z"}`)
	original := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(original)) })
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"standard":0.5,"premium":0.8}`))
	require.NoError(t, DB.Create(&Channel{Id: 7, Name: "shared"}).Error)
	require.NoError(t, DB.Create(&ProfitabilityEntry{
		EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 7, AccountCount: 2, AmountRmbCents: 2000, OccurredAt: 100,
	}).Error)
	require.NoError(t, DB.Create(&[]Log{
		{UserId: 2, Type: LogTypeConsume, ChannelId: 7, Group: "standard", Quota: 1_000_000, Other: `{"group_ratio":0.1}`, UpstreamRequestId: "standard-old", CreatedAt: 110},
		{UserId: 2, Type: LogTypeConsume, ChannelId: 7, Group: "standard", Quota: 2_000_000, Other: `{"group_ratio":0.2}`, UpstreamRequestId: "standard-new", CreatedAt: 111},
		{UserId: 2, Type: LogTypeConsume, ChannelId: 7, Group: "premium", Quota: 10_000_000, Other: `{"group_ratio":0.4}`, UpstreamRequestId: "premium", CreatedAt: 112},
	}).Error)

	report, err := GetProfitabilityReport(100, 250)
	require.NoError(t, err)
	require.Len(t, report.Groups, 2)
	groups := make(map[string]*ProfitabilityGroup)
	for _, group := range report.Groups {
		groups[group.Name] = group
	}
	assert.Equal(t, 0.5, groups["standard"].Ratio)
	assert.Equal(t, 0.8, groups["premium"].Ratio)
	assert.Equal(t, 4.0, groups["standard"].AllocatedCostRmb)
	assert.Equal(t, 16.0, groups["premium"].AllocatedCostRmb)
	require.NotNil(t, groups["standard"].BreakEvenRatio)
	require.NotNil(t, groups["premium"].BreakEvenRatio)
	// standard: 4 / (2 / 0.1 + 4 / 0.2); premium: 16 / (20 / 0.4).
	assert.InDelta(t, 0.1, *groups["standard"].BreakEvenRatio, 1e-9)
	assert.InDelta(t, 0.32, *groups["premium"].BreakEvenRatio, 1e-9)
	require.NotNil(t, groups["standard"].ConfiguredRatio)
	assert.Equal(t, 0.5, *groups["standard"].ConfiguredRatio)
	require.NotNil(t, groups["standard"].AllCalls.EffectiveRatio)
	require.NotNil(t, groups["premium"].AllCalls.EffectiveRatio)
	assert.InDelta(t, 0.03, *groups["standard"].AllCalls.EffectiveRatio, 1e-9)
	assert.InDelta(t, 0.025, *groups["premium"].AllCalls.EffectiveRatio, 1e-9)

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"standard":0,"premium":0.9}`))
	changed, err := GetProfitabilityReport(100, 250)
	require.NoError(t, err)
	for _, group := range changed.Groups {
		require.NotNil(t, group.ConfiguredRatio)
		assert.InDelta(t, *groups[group.Name].BreakEvenRatio, *group.BreakEvenRatio, 1e-9)
		if group.Name == "standard" {
			assert.Zero(t, *group.ConfiguredRatio)
		}
	}
}

func TestProfitabilityGroupIncludesAdministratorOnlyGroups(t *testing.T) {
	setupProfitabilityBreakdownTest(t, `
		{"request_id":"customer","account_id":11,"total_cost":80,"created_at":"1970-01-01T00:01:50Z"},
		{"request_id":"admin","account_id":11,"total_cost":20,"created_at":"1970-01-01T00:01:51Z"}`)
	require.NoError(t, DB.Create(&Channel{Id: 7, Name: "shared"}).Error)
	require.NoError(t, DB.Create(&ProfitabilityEntry{
		EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 7, AccountCount: 1, AmountRmbCents: 200, OccurredAt: 100,
	}).Error)
	require.NoError(t, DB.Create(&[]Log{
		{UserId: 2, Type: LogTypeConsume, ChannelId: 7, Group: "default", Quota: 2_000_000, Other: `{"group_ratio":0.1}`, UpstreamRequestId: "customer", CreatedAt: 110},
		{UserId: 1, Type: LogTypeConsume, ChannelId: 7, Group: "admin-only", Quota: 500_000, Other: `{"group_ratio":0.2}`, UpstreamRequestId: "admin", CreatedAt: 111},
	}).Error)

	report, err := GetProfitabilityReport(100, 250)
	require.NoError(t, err)
	require.Len(t, report.Groups, 2)
	var allocation, selfCharge float64
	for _, group := range report.Groups {
		allocation += group.AllocatedCostRmb
		selfCharge += group.SelfUsd
		if group.Name == "admin-only" {
			assert.Nil(t, group.ConfiguredRatio)
			assert.Equal(t, "no_activity", group.CustomerCalls.Status)
			assert.Nil(t, group.CustomerCalls.BreakEvenRatio)
			require.NotNil(t, group.AllCalls.BreakEvenRatio)
			assert.InDelta(t, 0.08, *group.AllCalls.BreakEvenRatio, 1e-9)
		}
	}
	assert.InDelta(t, report.AccountCostRmb, allocation, 1e-9)
	assert.Equal(t, 1.0, selfCharge)
}

func TestProfitabilityGroupCustomerBreakEvenKeepsCostAndExcludesGiftIncome(t *testing.T) {
	setupProfitabilityBreakdownTest(t, `
		{"request_id":"customer","account_id":11,"total_cost":80,"created_at":"1970-01-01T00:01:50Z"},
		{"request_id":"admin","account_id":11,"total_cost":20,"created_at":"1970-01-01T00:01:51Z"}`)
	require.NoError(t, DB.Create(&Channel{Id: 7, Name: "shared"}).Error)
	require.NoError(t, DB.Create(&[]ProfitabilityEntry{
		{EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 7, AccountCount: 1, AmountRmbCents: 200, OccurredAt: 100},
		{EntryType: ProfitabilityEntryAdminAdjustment, UserId: 2, Quota: 500_000, OccurredAt: 100},
	}).Error)
	require.NoError(t, DB.Create(&[]Log{
		{UserId: 2, Type: LogTypeConsume, ChannelId: 7, Group: "default", Quota: 2_000_000, Other: `{"group_ratio":0.1}`, UpstreamRequestId: "customer", CreatedAt: 110},
		{UserId: 1, Type: LogTypeConsume, ChannelId: 7, Group: "default", Quota: 500_000, Other: `{"group_ratio":0.2}`, UpstreamRequestId: "admin", CreatedAt: 111},
	}).Error)

	report, err := GetProfitabilityReport(100, 250)
	require.NoError(t, err)
	require.Len(t, report.Groups, 1)
	group := report.Groups[0]
	assert.Equal(t, 2.0, group.AllocatedCostRmb)
	assert.Equal(t, 3.0, group.CustomerChargeRmb)
	assert.Equal(t, 5.0, group.AllCalls.ChargeUsd)
	assert.Equal(t, 4.0, group.CustomerCalls.ChargeUsd)
	require.NotNil(t, group.AllCalls.SubUsageUsd)
	require.NotNil(t, group.CustomerCalls.SubUsageUsd)
	assert.Equal(t, 100.0, *group.AllCalls.SubUsageUsd)
	assert.Equal(t, 80.0, *group.CustomerCalls.SubUsageUsd)
	require.NotNil(t, group.AllCalls.BreakEvenRatio)
	require.NotNil(t, group.CustomerCalls.BreakEvenRatio)
	assert.InDelta(t, 2.0/45, *group.AllCalls.BreakEvenRatio, 1e-9)
	assert.InDelta(t, 2.0/30, *group.CustomerCalls.BreakEvenRatio, 1e-9)
	require.NotNil(t, report.Pool.ProfitRmb)
	assert.Equal(t, 1.0, *report.Pool.ProfitRmb)
}

func TestProfitabilityGroupCannotSuggestBaseRateWhenHistoryIsMissingOrOverridden(t *testing.T) {
	for _, tc := range []struct {
		name, pricing, status string
	}{
		{"legacy", "", "missing_ratio"},
		{"malformed", "{", "missing_ratio"},
		{"zero", `{"group_ratio":0}`, "missing_ratio"},
		{"channel", `{"group_ratio":0.1,"channel_ratio":0.1,"effective_ratio":0.1}`, "ratio_override"},
		{"user", `{"group_ratio":0.2,"user_group_ratio":0.2}`, "ratio_override"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupProfitabilityBreakdownTest(t, `{"request_id":"one","account_id":11,"total_cost":10,"created_at":"1970-01-01T00:01:50Z"}`)
			require.NoError(t, DB.Create(&Channel{Id: 7, Name: "shared"}).Error)
			require.NoError(t, DB.Create(&ProfitabilityEntry{
				EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 7, AccountCount: 1, AmountRmbCents: 100, OccurredAt: 100,
			}).Error)
			require.NoError(t, DB.Create(&Log{
				UserId: 2, Type: LogTypeConsume, ChannelId: 7, Group: "default", Quota: 500_000, Other: tc.pricing, UpstreamRequestId: "one", CreatedAt: 110,
			}).Error)
			report, err := GetProfitabilityReport(100, 250)
			require.NoError(t, err)
			require.Len(t, report.Groups, 1)
			group := report.Groups[0]
			assert.Equal(t, "complete", group.AllCalls.Status)
			assert.Equal(t, tc.status, group.AllCalls.BreakEvenStatus)
			assert.Equal(t, tc.status, group.CustomerCalls.BreakEvenStatus)
			assert.Nil(t, group.AllCalls.BreakEvenRatio)
			assert.Nil(t, group.CustomerCalls.BreakEvenRatio)
			assert.Nil(t, group.BreakEvenRatio)
		})
	}
}

func TestProfitabilityGroupUsesOnlySelectedChannelsDespiteKnownOutsidePoolUsage(t *testing.T) {
	setupProfitabilityBreakdownTest(t, `
		{"request_id":"selected","account_id":11,"total_cost":10,"created_at":"1970-01-01T00:01:50Z"},
		{"request_id":"outside","account_id":12,"total_cost":90,"created_at":"1970-01-01T00:01:51Z"}`)
	require.NoError(t, DB.Create(&[]Channel{{Id: 7, Name: "selected"}, {Id: 8, Name: "outside"}}).Error)
	require.NoError(t, DB.Create(&ProfitabilityEntry{
		EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 7, AccountCount: 1, AmountRmbCents: 100, OccurredAt: 100,
	}).Error)
	require.NoError(t, DB.Create(&[]Log{
		{UserId: 2, Type: LogTypeConsume, ChannelId: 7, Group: "default", Quota: 500_000, Other: `{"group_ratio":0.1}`, UpstreamRequestId: "selected", CreatedAt: 110},
		{UserId: 2, Type: LogTypeConsume, ChannelId: 8, Group: "default", Quota: 10_000_000, Other: `{"group_ratio":0.4}`, UpstreamRequestId: "outside", CreatedAt: 111},
	}).Error)
	report, err := GetProfitabilityReportWithOptions(100, 250, ProfitabilityReportOptions{ChannelSelectionPresent: true, ChannelIDs: []int{7}})
	require.NoError(t, err)
	require.Len(t, report.Groups, 1)
	assert.Equal(t, "scope_mismatch", report.Pool.AllCalls.Status)
	group := report.Groups[0]
	require.NotNil(t, group.AllCalls.SubUsageUsd)
	require.NotNil(t, group.CustomerCalls.BreakEvenRatio)
	assert.Equal(t, 10.0, *group.AllCalls.SubUsageUsd)
	assert.InDelta(t, 0.1, *group.CustomerCalls.BreakEvenRatio, 1e-9)
}

func TestProfitabilityGroupDoesNotInventBreakEvenWithIncompleteAccounting(t *testing.T) {
	for _, tc := range []struct {
		name, status string
	}{
		{"missing purchase", "missing_purchase"},
		{"idle purchase", "unallocated_purchase"},
		{"unassigned usage", "unverified_scope"},
		{"missing request id", "unmatched"},
		{"gift only", "no_paid_usage"},
		{"cash", "cash_scope"},
		{"zero usage", "no_usage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := `{"request_id":"one","account_id":11,"total_cost":10,"created_at":"1970-01-01T00:01:50Z"}`
			if tc.name == "unassigned usage" {
				items += `,{"request_id":"","account_id":12,"total_cost":5,"created_at":"1970-01-01T00:01:51Z"}`
			} else if tc.name == "zero usage" {
				items = `{"request_id":"one","account_id":11,"total_cost":0,"created_at":"1970-01-01T00:01:50Z"}`
			}
			setupProfitabilityBreakdownTest(t, items)
			require.NoError(t, DB.Create(&[]Channel{{Id: 7, Name: "used"}, {Id: 8, Name: "idle"}}).Error)
			if tc.name != "missing purchase" {
				require.NoError(t, DB.Create(&ProfitabilityEntry{
					EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 7, AccountCount: 1, AmountRmbCents: 100, OccurredAt: 100,
				}).Error)
			}
			if tc.name == "idle purchase" {
				require.NoError(t, DB.Create(&ProfitabilityEntry{
					EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 8, AccountCount: 1, AmountRmbCents: 200, OccurredAt: 100,
				}).Error)
			}
			if tc.name == "gift only" {
				require.NoError(t, DB.Create(&ProfitabilityEntry{
					EntryType: ProfitabilityEntryAdminAdjustment, UserId: 2, Quota: 500_000, OccurredAt: 100,
				}).Error)
			}
			requestID := "one"
			if tc.name == "missing request id" {
				requestID = ""
			}
			require.NoError(t, DB.Create(&Log{
				UserId: 2, Type: LogTypeConsume, ChannelId: 7, Group: "default", Quota: 500_000, Other: `{"group_ratio":0.1}`, UpstreamRequestId: requestID, CreatedAt: 110,
			}).Error)
			options := ProfitabilityReportOptions{}
			if tc.name == "cash" {
				options.RevenueBasis = ProfitabilityRevenueBasisCash
			}
			report, err := GetProfitabilityReportWithOptions(100, 250, options)
			require.NoError(t, err)
			require.Len(t, report.Groups, 1)
			assert.Equal(t, tc.status, report.Groups[0].CustomerCalls.BreakEvenStatus)
			assert.Nil(t, report.Groups[0].CustomerCalls.BreakEvenRatio)
		})
	}
}
