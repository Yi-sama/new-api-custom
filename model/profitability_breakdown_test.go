package model

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProfitabilityReportMatchesSubAccountsAndExcludesGiftCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/admin/usage", r.URL.Path)
		require.Equal(t, "test-admin-key", r.Header.Get("x-api-key"))
		page := r.URL.Query().Get("page")
		var account, cost, request string
		if page == "1" {
			account, cost, request = "11", "2", "client:sub-1"
		} else {
			account, cost, request = "12", "3", "client:sub-2"
		}
		_, err := fmt.Fprintf(w, `{"code":0,"data":{"pages":2,"items":[{"request_id":%q,"account_id":%s,"total_cost":%s,"created_at":"1970-01-01T00:01:50Z"}]}}`, request, account, cost)
		require.NoError(t, err)
	}))
	defer server.Close()
	t.Setenv("SUB2API_BASE_URL", server.URL)
	t.Setenv("SUB2API_ADMIN_API_KEY", "test-admin-key")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	originalDB, originalLogDB, originalGroup, originalQuota := DB, LOG_DB, logGroupCol, common.QuotaPerUnit
	t.Cleanup(func() {
		DB, LOG_DB, logGroupCol, common.QuotaPerUnit = originalDB, originalLogDB, originalGroup, originalQuota
		require.NoError(t, sqlDB.Close())
	})
	DB, LOG_DB, logGroupCol, common.QuotaPerUnit = db, db, "`group`", 500_000
	require.NoError(t, db.AutoMigrate(&User{}, &Channel{}, &Log{}, &ProfitabilityEntry{}, &TopUp{}))
	require.NoError(t, db.Create(&[]User{{Id: 1, Username: "admin", AffCode: "admin-aff", Role: common.RoleAdminUser}, {Id: 2, Username: "buyer", AffCode: "buyer-aff", Role: 1}}).Error)
	require.NoError(t, db.Create(&Channel{Id: 7, Name: "Sub channel"}).Error)
	require.NoError(t, db.Create(&[]ProfitabilityEntry{
		{EntryType: ProfitabilityEntryAdminAdjustment, UserId: 2, Quota: 500_000, AmountCents: 100, OccurredAt: 90},
		{EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 7, AccountCount: 2, AmountRmbCents: 400, OccurredAt: 100},
	}).Error)
	require.NoError(t, db.Create(&TopUp{UserId: 2, Amount: 10, Money: 6, Status: common.TopUpStatusSuccess, CompleteTime: 120}).Error)
	require.NoError(t, db.Create(&[]Log{
		{UserId: 2, Type: LogTypeConsume, ChannelId: 7, ModelName: "gpt-a", Group: "default", Quota: 1_500_000, UpstreamRequestId: "client:sub-1", CreatedAt: 110},
		{UserId: 2, Type: LogTypeConsume, ChannelId: 7, ModelName: "gpt-b", Group: "default", Quota: 1_000_000, UpstreamRequestId: "client:sub-2", CreatedAt: 210},
	}).Error)

	report, err := GetProfitabilityReport(100, 250)
	require.NoError(t, err)
	assert.Equal(t, "connected", report.SubUsageStatus)
	assert.Equal(t, 6.0, report.CashReceivedRmb)
	assert.Equal(t, 4.0, report.CallChargeRmb)
	assert.Equal(t, 4.0, report.CashRecognizedRmb)
	require.NotNil(t, report.EstimatedProfitRmb)
	assert.Equal(t, 0.0, *report.EstimatedProfitRmb)
	require.NotNil(t, report.CashProfitRmb)
	assert.Equal(t, 0.0, *report.CashProfitRmb)
	require.Len(t, report.Accounts, 2)
	assert.Equal(t, int64(11), report.Accounts[0].AccountId)
	assert.Equal(t, int64(12), report.Accounts[1].AccountId)
	require.Len(t, report.Channels, 1)
	assert.Equal(t, 5.0, report.Channels[0].SubUsageUsd)
	assert.Equal(t, int64(2), report.Channels[0].MatchedRequests)
	require.Len(t, report.Models, 2)
	assert.InDelta(t, 1.6, report.Models[0].AllocatedCostRmb, 0.001)
	assert.InDelta(t, 2.4, report.Models[1].AllocatedCostRmb, 0.001)
	require.Len(t, report.Groups, 1)
	assert.InDelta(t, 4.0, report.Groups[0].AllocatedCostRmb, 0.001)
	assert.Equal(t, 4.0, report.Groups[0].CustomerChargeRmb)
}

func TestProfitabilityReportLeavesProfitUnknownWhenSubCallCannotMatch(t *testing.T) {
	t.Setenv("SUB2API_BASE_URL", "")
	t.Setenv("SUB2API_ADMIN_API_KEY", "")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	originalDB, originalLogDB, originalGroup, originalQuota := DB, LOG_DB, logGroupCol, common.QuotaPerUnit
	t.Cleanup(func() {
		DB, LOG_DB, logGroupCol, common.QuotaPerUnit = originalDB, originalLogDB, originalGroup, originalQuota
		require.NoError(t, sqlDB.Close())
	})
	DB, LOG_DB, logGroupCol, common.QuotaPerUnit = db, db, "`group`", 500_000
	require.NoError(t, db.AutoMigrate(&User{}, &Channel{}, &Log{}, &ProfitabilityEntry{}, &TopUp{}))
	require.NoError(t, db.Create(&Channel{Id: 7, Name: "Sub channel"}).Error)
	require.NoError(t, db.Create(&Log{UserId: 2, Type: LogTypeConsume, ChannelId: 7, ModelName: "unmatched-model", Group: "default", Quota: 500_000, CreatedAt: 110}).Error)

	report, err := GetProfitabilityReport(100, 250)
	require.NoError(t, err)
	assert.Equal(t, "not_configured", report.SubUsageStatus)
	assert.Equal(t, 1.0, report.CallChargeRmb)
	assert.Nil(t, report.EstimatedProfitRmb)
	assert.Equal(t, int64(1), report.Channels[0].UnmatchedRequests)
	require.Len(t, report.Models, 1)
	assert.Equal(t, 1.0, report.Models[0].CustomerChargeRmb)
	require.Len(t, report.Groups, 1)
	assert.Equal(t, 1.0, report.Groups[0].CustomerChargeRmb)
}

func TestSub2APIUsageLogsExcludeAdjacentDayRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "1970-01-01", r.URL.Query().Get("start_date"))
		_, err := w.Write([]byte(`{"code":0,"data":{"pages":1,"items":[
			{"request_id":"before","account_id":11,"total_cost":2,"created_at":"1970-01-01T00:01:39Z"},
			{"request_id":"inside","account_id":12,"total_cost":3,"created_at":"1970-01-01T00:01:40Z"},
			{"request_id":"after","account_id":13,"total_cost":4,"created_at":"1970-01-01T00:04:11Z"}
		]}}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	t.Setenv("SUB2API_BASE_URL", server.URL)
	t.Setenv("SUB2API_ADMIN_API_KEY", "test-admin-key")

	logs, status, message := GetSub2APIUsageLogs(100, 250)
	require.Equal(t, "connected", status, message)
	require.Len(t, logs, 1)
	assert.Equal(t, "inside", logs[0].RequestId)
}

func TestSub2APIUsageLogsPaginationStableDuringNewCalls(t *testing.T) {
	// A new highest-ID row arrives after page one. Descending offset pages overlap.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		first := `{"request_id":"one","account_id":1,"total_cost":1,"created_at":"1970-01-01T00:01:50Z"}`
		second := `{"request_id":"two","account_id":1,"total_cost":2,"created_at":"1970-01-01T00:02:00Z"}`
		third := `{"request_id":"three","account_id":1,"total_cost":3,"created_at":"1970-01-01T00:02:10Z"}`
		newest := `{"request_id":"new","account_id":1,"total_cost":4,"created_at":"1970-01-01T00:05:00Z"}`
		items := third + "," + second
		if query.Get("page") == "2" {
			items = second + "," + first
		}
		if query.Get("sort_by") == "id" && query.Get("sort_order") == "asc" {
			items = first + "," + second
			if query.Get("page") == "2" {
				items = third + "," + newest
			}
		}
		_, err := fmt.Fprintf(w, `{"code":0,"data":{"pages":2,"items":[%s]}}`, items)
		require.NoError(t, err)
	}))
	defer server.Close()
	t.Setenv("SUB2API_BASE_URL", server.URL)
	t.Setenv("SUB2API_ADMIN_API_KEY", "test-admin-key")

	logs, status, message := GetSub2APIUsageLogs(100, 250)
	require.Equal(t, "connected", status, message)
	require.Len(t, logs, 3)
	assert.Equal(t, []string{"one", "two", "three"}, []string{logs[0].RequestId, logs[1].RequestId, logs[2].RequestId})
	assert.Equal(t, 6.0, logs[0].accountCost()+logs[1].accountCost()+logs[2].accountCost())
}

func TestProfitabilityReportLeavesProfitUnknownForUnmatchedSubUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{"code":0,"data":{"pages":1,"items":[{"request_id":"client:unknown","account_id":11,"total_cost":2,"created_at":"1970-01-01T00:01:50Z"}]}}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	t.Setenv("SUB2API_BASE_URL", server.URL)
	t.Setenv("SUB2API_ADMIN_API_KEY", "test-admin-key")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	originalDB, originalLogDB, originalGroup := DB, LOG_DB, logGroupCol
	t.Cleanup(func() {
		DB, LOG_DB, logGroupCol = originalDB, originalLogDB, originalGroup
		require.NoError(t, sqlDB.Close())
	})
	DB, LOG_DB, logGroupCol = db, db, "`group`"
	require.NoError(t, db.AutoMigrate(&User{}, &Channel{}, &Log{}, &ProfitabilityEntry{}, &TopUp{}))
	require.NoError(t, db.Create(&Channel{Id: 7, Name: "Sub channel"}).Error)
	require.NoError(t, db.Create(&ProfitabilityEntry{EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 7, AccountCount: 1, AmountRmbCents: 400, OccurredAt: 100}).Error)

	report, err := GetProfitabilityReport(100, 250)
	require.NoError(t, err)
	assert.Equal(t, "connected", report.SubUsageStatus)
	assert.Equal(t, int64(1), report.UnmatchedSubRequests)
	assert.Nil(t, report.EstimatedProfitRmb)
	require.NotNil(t, report.Channels[0].EstimatedProfitRmb)
	assert.Equal(t, -4.0, *report.Channels[0].EstimatedProfitRmb)
}

func TestProfitabilitySelectedChannelsFilterEveryBreakdown(t *testing.T) {
	setupProfitabilityBreakdownTest(t, `
		{"request_id":"req-1","account_id":11,"total_cost":2,"created_at":"1970-01-01T00:01:50Z"},
		{"request_id":"req-2","account_id":12,"total_cost":3,"created_at":"1970-01-01T00:01:51Z"}`)
	require.NoError(t, DB.Create(&[]Channel{
		{Id: 1, Name: "one", Status: common.ChannelStatusEnabled},
		{Id: 2, Name: "two", Status: common.ChannelStatusManuallyDisabled},
	}).Error)
	require.NoError(t, DB.Create(&[]ProfitabilityEntry{
		{EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 1, AccountCount: 1, AmountRmbCents: 100, OccurredAt: 100},
		{EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 2, AccountCount: 2, AmountRmbCents: 900, OccurredAt: 100},
	}).Error)
	require.NoError(t, DB.Create(&[]Log{
		{UserId: 2, Type: LogTypeConsume, ChannelId: 1, ModelName: "m1", Group: "default", Quota: 500_000, UpstreamRequestId: "req-1", CreatedAt: 110},
		{UserId: 2, Type: LogTypeConsume, ChannelId: 2, ModelName: "m2", Group: "other", Quota: 500_000, UpstreamRequestId: "req-2", CreatedAt: 111},
	}).Error)

	report, err := GetProfitabilityReportWithOptions(100, 250, ProfitabilityReportOptions{ChannelSelectionPresent: true, ChannelIDs: []int{1}})
	require.NoError(t, err)
	assert.Equal(t, "selected", report.Scope)
	assert.Equal(t, []int{1}, report.SelectedChannelIds)
	assert.Equal(t, 1.0, report.RevenueUsd)
	assert.Equal(t, 1.0, report.AccountCostRmb)
	assert.Equal(t, int64(1), report.AccountPurchaseCount)
	assert.Equal(t, int64(500_000), report.Usage.Quota)
	assert.Equal(t, 1.0, report.Usage.AmountUsd)
	assert.Equal(t, 1.0, report.Usage.CustomerUsd)
	assert.Zero(t, report.Usage.SelfUsd)
	require.Len(t, report.Usage.Users, 1)
	assert.Equal(t, 2, report.Usage.Users[0].UserId)
	require.Len(t, report.Usage.Groups, 1)
	assert.Equal(t, "default", report.Usage.Groups[0].Name)
	require.Len(t, report.Channels, 1)
	assert.Equal(t, 1, report.Channels[0].Id)
	assert.Equal(t, common.ChannelStatusEnabled, report.Channels[0].Status)
	assert.Equal(t, "complete", report.Channels[0].ProfitStatus)
	require.Len(t, report.Models, 1)
	assert.Equal(t, 1, report.Models[0].ChannelId)
	require.Len(t, report.Accounts, 1)
	assert.Equal(t, 1, report.Accounts[0].ChannelId)
	assert.Equal(t, int64(1), report.MatchedRequests)
	assert.Equal(t, int64(0), report.UnmatchedSubRequests)
	require.Len(t, report.AvailableChannels, 2)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, report.AvailableChannels[1].Status)
}

func TestProfitabilityGiftIsConsumedBeforeChannelFiltering(t *testing.T) {
	setupProfitabilityBreakdownTest(t, `
		{"request_id":"excluded","account_id":21,"total_cost":1,"created_at":"1970-01-01T00:01:40Z"},
		{"request_id":"selected","account_id":11,"total_cost":1,"created_at":"1970-01-01T00:01:50Z"}`)
	require.NoError(t, DB.Create(&[]Channel{{Id: 1, Name: "selected"}, {Id: 2, Name: "excluded"}}).Error)
	require.NoError(t, DB.Create(&[]ProfitabilityEntry{
		{EntryType: ProfitabilityEntryAdminAdjustment, UserId: 2, Quota: 500_000, AmountCents: 100, OccurredAt: 90},
		{EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 1, AccountCount: 1, AmountRmbCents: 100, OccurredAt: 100},
	}).Error)
	require.NoError(t, DB.Create(&[]Log{
		{UserId: 2, Type: LogTypeConsume, ChannelId: 2, ModelName: "m", Group: "default", Quota: 500_000, UpstreamRequestId: "excluded", CreatedAt: 100},
		{UserId: 2, Type: LogTypeConsume, ChannelId: 1, ModelName: "m", Group: "default", Quota: 500_000, UpstreamRequestId: "selected", CreatedAt: 110},
	}).Error)

	report, err := GetProfitabilityReportWithOptions(100, 250, ProfitabilityReportOptions{ChannelSelectionPresent: true, ChannelIDs: []int{1}})
	require.NoError(t, err)
	assert.Equal(t, 1.0, report.CallChargeRmb)
	assert.Equal(t, int64(0), report.UnmatchedSubRequests)
	assert.Equal(t, "complete", report.ProfitStatus)
}

func TestProfitabilityGlobalOrphanDoesNotPoisonSelectedChannels(t *testing.T) {
	setupProfitabilityBreakdownTest(t, `
		{"request_id":"selected","account_id":11,"total_cost":2,"created_at":"1970-01-01T00:01:50Z"},
		{"request_id":"orphan","account_id":99,"total_cost":3,"created_at":"1970-01-01T00:01:51Z"}`)
	require.NoError(t, DB.Create(&Channel{Id: 1, Name: "selected"}).Error)
	require.NoError(t, DB.Create(&ProfitabilityEntry{EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 1, AccountCount: 1, AmountRmbCents: 100, OccurredAt: 100}).Error)
	require.NoError(t, DB.Create(&Log{UserId: 2, Type: LogTypeConsume, ChannelId: 1, ModelName: "m", Group: "default", Quota: 500_000, UpstreamRequestId: "selected", CreatedAt: 110}).Error)

	report, err := GetProfitabilityReportWithOptions(100, 250, ProfitabilityReportOptions{ChannelSelectionPresent: true, ChannelIDs: []int{1}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), report.UnmatchedSubRequests)
	assert.Equal(t, 3.0, report.UnmatchedSubUsageUsd)
	assert.Equal(t, "complete", report.ProfitStatus)
	require.NotNil(t, report.EstimatedProfitRmb)
	assert.Equal(t, 0.0, *report.EstimatedProfitRmb)
}

func TestProfitabilityMissingPurchaseIsUnknown(t *testing.T) {
	setupProfitabilityBreakdownTest(t, `{"request_id":"selected","account_id":11,"total_cost":2,"created_at":"1970-01-01T00:01:50Z"}`)
	require.NoError(t, DB.Create(&Channel{Id: 1, Name: "selected"}).Error)
	require.NoError(t, DB.Create(&Log{UserId: 2, Type: LogTypeConsume, ChannelId: 1, ModelName: "m", Group: "default", Quota: 500_000, UpstreamRequestId: "selected", CreatedAt: 110}).Error)

	report, err := GetProfitabilityReportWithOptions(100, 250, ProfitabilityReportOptions{ChannelSelectionPresent: true, ChannelIDs: []int{1}})
	require.NoError(t, err)
	assert.Equal(t, "missing_purchase", report.ProfitStatus)
	assert.Equal(t, "missing_purchase", report.Channels[0].ProfitStatus)
	assert.False(t, report.Channels[0].PurchaseRecorded)
	assert.Nil(t, report.Channels[0].EstimatedProfitRmb)
	assert.Nil(t, report.EstimatedProfitRmb)
	assert.Equal(t, "missing_purchase", report.ProfitStatus)
}

func TestProfitabilityExplicitEmptySelection(t *testing.T) {
	setupProfitabilityBreakdownTest(t, ``)
	require.NoError(t, DB.Create(&Channel{Id: 1, Name: "available"}).Error)

	report, err := GetProfitabilityReportWithOptions(100, 250, ProfitabilityReportOptions{ChannelSelectionPresent: true})
	require.NoError(t, err)
	assert.Equal(t, "selected", report.Scope)
	assert.Empty(t, report.SelectedChannelIds)
	assert.Empty(t, report.Channels)
	assert.Equal(t, "empty_selection", report.ProfitStatus)
	assert.Zero(t, report.RevenueUsd)
	assert.Len(t, report.AvailableChannels, 1)
	payload, err := common.Marshal(report)
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"selected_channel_ids":[]`)
}

func TestProfitabilityCashBasisIgnoresChannelSelection(t *testing.T) {
	setupProfitabilityBreakdownTest(t, `
		{"request_id":"req-1","account_id":11,"total_cost":1,"created_at":"1970-01-01T00:01:50Z"},
		{"request_id":"req-2","account_id":12,"total_cost":1,"created_at":"1970-01-01T00:01:51Z"}`)
	require.NoError(t, DB.Create(&[]Channel{{Id: 1, Name: "one"}, {Id: 2, Name: "two"}}).Error)
	require.NoError(t, DB.Create(&TopUp{UserId: 2, Money: 10, Status: common.TopUpStatusSuccess, CompleteTime: 120}).Error)
	require.NoError(t, DB.Create(&[]ProfitabilityEntry{
		{EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 1, AccountCount: 1, AmountRmbCents: 100, OccurredAt: 100},
		{EntryType: ProfitabilityEntryAccountPurchase, ChannelId: 2, AccountCount: 1, AmountRmbCents: 100, OccurredAt: 100},
	}).Error)
	require.NoError(t, DB.Create(&[]Log{
		{UserId: 2, Type: LogTypeConsume, ChannelId: 1, ModelName: "m1", Group: "default", Quota: 500_000, UpstreamRequestId: "req-1", CreatedAt: 110},
		{UserId: 2, Type: LogTypeConsume, ChannelId: 2, ModelName: "m2", Group: "default", Quota: 500_000, UpstreamRequestId: "req-2", CreatedAt: 111},
	}).Error)

	report, err := GetProfitabilityReportWithOptions(100, 250, ProfitabilityReportOptions{
		RevenueBasis: ProfitabilityRevenueBasisCash, ChannelSelectionPresent: true, ChannelIDs: []int{1},
	})
	require.NoError(t, err)
	assert.Equal(t, "all", report.Scope)
	assert.Empty(t, report.SelectedChannelIds)
	assert.Len(t, report.Channels, 2)
	assert.Equal(t, 2.0, report.AccountCostRmb)
	assert.Equal(t, 2.0, report.CashRecognizedRmb)
	assert.Equal(t, 2.0, report.RevenueUsd)
	assert.Equal(t, "complete", report.ProfitStatus)
}

func setupProfitabilityBreakdownTest(t *testing.T, items string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprintf(w, `{"code":0,"data":{"pages":1,"items":[%s]}}`, items)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)
	t.Setenv("SUB2API_BASE_URL", server.URL)
	t.Setenv("SUB2API_ADMIN_API_KEY", "test-admin-key")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	originalDB, originalLogDB, originalGroup, originalQuota := DB, LOG_DB, logGroupCol, common.QuotaPerUnit
	t.Cleanup(func() {
		DB, LOG_DB, logGroupCol, common.QuotaPerUnit = originalDB, originalLogDB, originalGroup, originalQuota
		require.NoError(t, sqlDB.Close())
	})
	DB, LOG_DB, logGroupCol, common.QuotaPerUnit = db, db, "`group`", 500_000
	require.NoError(t, db.AutoMigrate(&User{}, &Channel{}, &Log{}, &ProfitabilityEntry{}, &TopUp{}))
	require.NoError(t, db.Create(&[]User{
		{Id: 1, Username: "admin", AffCode: "admin-aff", Role: common.RoleAdminUser},
		{Id: 2, Username: "ordinary", AffCode: "ordinary-aff", Role: 1},
	}).Error)
}
