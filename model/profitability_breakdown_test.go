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
	require.NoError(t, db.Create(&Log{UserId: 2, Type: LogTypeConsume, ChannelId: 7, Group: "default", Quota: 500_000, CreatedAt: 110}).Error)

	report, err := GetProfitabilityReport(100, 250)
	require.NoError(t, err)
	assert.Equal(t, "not_configured", report.SubUsageStatus)
	assert.Equal(t, 1.0, report.CallChargeRmb)
	assert.Nil(t, report.EstimatedProfitRmb)
	assert.Equal(t, int64(1), report.Channels[0].UnmatchedRequests)
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
	assert.Nil(t, report.Channels[0].EstimatedProfitRmb)
}
