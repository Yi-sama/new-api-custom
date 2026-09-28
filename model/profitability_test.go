package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProfitabilityUsageGroupsSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	originalDB, originalGroup, originalQuota := LOG_DB, logGroupCol, common.QuotaPerUnit
	t.Cleanup(func() {
		LOG_DB, logGroupCol, common.QuotaPerUnit = originalDB, originalGroup, originalQuota
		require.NoError(t, sqlDB.Close())
	})
	LOG_DB, logGroupCol, common.QuotaPerUnit = db, "`group`", 500_000
	require.NoError(t, db.AutoMigrate(&Log{}))
	require.NoError(t, db.Create(&[]Log{
		{UserId: 2, Username: "customer", Type: LogTypeConsume, Group: "default", Quota: 500_000, CreatedAt: 100},
		{UserId: 2, Username: "customer", Type: LogTypeConsume, Group: "default", Quota: 1_000_000, CreatedAt: 101},
		{UserId: 1, Username: "admin", Type: LogTypeConsume, Group: "default", Quota: 2_000_000, CreatedAt: 102},
		{UserId: 2, Username: "customer", Type: LogTypeConsume, Group: "default", Quota: 500_000, CreatedAt: 200},
	}).Error)

	usage, err := getProfitabilityUsage(100, 150, []int{1}, 0)
	require.NoError(t, err)
	assert.Equal(t, 7.0, usage.AmountUsd)
	assert.Equal(t, 3.0, usage.CustomerUsd)
	assert.Equal(t, 4.0, usage.SelfUsd)
	require.Len(t, usage.Groups, 1)
	assert.Equal(t, "default", usage.Groups[0].Name)
	assert.Equal(t, 3.0, usage.Groups[0].CustomerUsd)
}

func TestParseUSDAmountCents(t *testing.T) {
	amount, err := ParseUSDAmountCents(12.345)
	require.NoError(t, err)
	require.Equal(t, int64(1235), amount)

	_, err = ParseUSDAmountCents(0)
	require.Error(t, err)
}

func TestParseRMBAmountCents(t *testing.T) {
	amount, err := ParseRMBAmountCents(12.345)
	require.NoError(t, err)
	require.Equal(t, int64(1235), amount)

	_, err = ParseRMBAmountCents(0)
	require.Error(t, err)
}

func TestQuotaToUSDCentsUsesQuotaPerUnit(t *testing.T) {
	original := common.QuotaPerUnit
	t.Cleanup(func() {
		common.QuotaPerUnit = original
	})
	common.QuotaPerUnit = 500_000

	require.Equal(t, int64(100), QuotaToUSDCents(500_000))
	require.Equal(t, int64(1), QuotaToUSDCents(2_500))
	require.Equal(t, int64(0), QuotaToUSDCents(0))

	common.QuotaPerUnit = 0
	require.Equal(t, int64(0), QuotaToUSDCents(500_000))
}

func TestValidateProfitabilityEntry(t *testing.T) {
	require.NoError(t, ValidateProfitabilityEntry(
		ProfitabilityEntryAccountPurchase,
		1_000,
		2,
	))
	require.Error(t, ValidateProfitabilityEntry(
		ProfitabilityEntryAccountPurchase,
		1_000,
		0,
	))
	require.NoError(t, ValidateProfitabilityEntry(
		ProfitabilityEntryManualIncome,
		1_000,
		0,
	))
	require.Error(t, ValidateProfitabilityEntry("unknown", 1_000, 0))
}
