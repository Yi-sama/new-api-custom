package model

import (
	"errors"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestWalletBalanceTracksOnlySettledSpending(t *testing.T) {
	for _, redis := range []bool{false, true} {
		t.Run(map[bool]string{false: "database", true: "redis"}[redis], func(t *testing.T) {
			truncateTables(t)
			resetBatchUpdateTestState(t)
			if redis {
				useUserCacheMiniRedis(t)
			}
			user := createReserveTestUser(t, 500000)
			reserved, err := PreConsumeWalletQuota("wallet-snapshot", user.Id, 110000)
			require.NoError(t, err)
			require.True(t, reserved)
			balance, err := GetWalletBalance(user.Id)
			require.NoError(t, err)
			assert.Equal(t, WalletBalance{Quota: 390000, SettledQuota: 500000, PendingQuota: 110000, PendingRequestCount: 1}, balance)

			reserved, err = PreConsumeWalletQuota("wallet-snapshot", user.Id, 110000)
			require.NoError(t, err)
			assert.True(t, reserved)
			require.NoError(t, AdjustWalletPreConsume("wallet-snapshot", user.Id, 10000))
			balance, err = GetWalletBalance(user.Id)
			require.NoError(t, err)
			assert.Equal(t, int64(500000), balance.SettledQuota)
			assert.Equal(t, int64(120000), balance.PendingQuota)
			require.NoError(t, AdjustWalletPreConsume("wallet-snapshot", user.Id, -10000))

			require.NoError(t, IncreaseUserQuota(user.Id, 150000, true))
			balance, err = GetWalletBalance(user.Id)
			require.NoError(t, err)
			assert.Equal(t, int64(650000), balance.SettledQuota, "top-ups remain visible during a request")

			require.NoError(t, SettleWalletPreConsume("wallet-snapshot", user.Id, 95000))
			require.NoError(t, SettleWalletPreConsume("wallet-snapshot", user.Id, 95000))
			balance, err = GetWalletBalance(user.Id)
			require.NoError(t, err)
			assert.Equal(t, WalletBalance{Quota: 555000, SettledQuota: 555000}, balance)
			assert.Error(t, RefundWalletPreConsume("wallet-snapshot", user.Id))
			if redis {
				cached, cacheErr := GetUserQuota(user.Id, false)
				require.NoError(t, cacheErr)
				assert.Equal(t, 555000, cached)
			}
		})
	}
}

func TestWalletHoldsKeepParallelRequestsSeparate(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	user := createReserveTestUser(t, 500000)
	for _, id := range []string{"wallet-first", "wallet-second"} {
		reserved, err := PreConsumeWalletQuota(id, user.Id, 110000)
		require.NoError(t, err)
		require.True(t, reserved)
	}
	require.NoError(t, SettleWalletPreConsume("wallet-first", user.Id, 95000))
	balance, err := GetWalletBalance(user.Id)
	require.NoError(t, err)
	assert.Equal(t, WalletBalance{Quota: 295000, SettledQuota: 405000, PendingQuota: 110000, PendingRequestCount: 1}, balance)

	// A new database session reads persisted holds, not a process-local counter.
	previous := DB
	DB = DB.Session(&gorm.Session{NewDB: true})
	t.Cleanup(func() { DB = previous })
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			results <- RefundWalletPreConsume("wallet-second", user.Id)
		}()
	}
	workers.Wait()
	close(results)
	for refundErr := range results {
		require.NoError(t, refundErr)
	}
	balance, err = GetWalletBalance(user.Id)
	require.NoError(t, err)
	assert.Equal(t, WalletBalance{Quota: 405000, SettledQuota: 405000}, balance)
}

func TestWalletSettlementEqualExcessAndFree(t *testing.T) {
	for _, actual := range []int{0, 110000, 130000} {
		t.Run(map[int]string{0: "free", 110000: "equal", 130000: "additional-charge"}[actual], func(t *testing.T) {
			truncateTables(t)
			resetBatchUpdateTestState(t)
			user := createReserveTestUser(t, 500000)
			reserved, err := PreConsumeWalletQuota("wallet-final", user.Id, 110000)
			require.NoError(t, err)
			require.True(t, reserved)
			require.NoError(t, SettleWalletPreConsume("wallet-final", user.Id, actual))
			balance, err := GetWalletBalance(user.Id)
			require.NoError(t, err)
			assert.Equal(t, int64(500000-actual), balance.Quota)
			assert.Equal(t, balance.Quota, balance.SettledQuota)
			assert.Zero(t, balance.PendingRequestCount)
			assert.Zero(t, balance.PendingQuota)
		})
	}
}

func TestWalletPreConsumeFailureRollsBackDatabaseAndCache(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	useUserCacheMiniRedis(t)
	user := createReserveTestUser(t, 500000)
	require.NoError(t, DB.Callback().Create().Before("gorm:create").Register("wallet-fail-create", func(tx *gorm.DB) {
		if tx.Statement.Table == "wallet_pre_consume_records" {
			_ = tx.AddError(errors.New("fixture hold write failed"))
		}
	}))
	t.Cleanup(func() { require.NoError(t, DB.Callback().Create().Remove("wallet-fail-create")) })
	reserved, err := PreConsumeWalletQuota("wallet-rollback", user.Id, 110000)
	require.Error(t, err)
	assert.False(t, reserved)
	balance, err := GetWalletBalance(user.Id)
	require.NoError(t, err)
	assert.Equal(t, WalletBalance{Quota: 500000, SettledQuota: 500000}, balance)
	cached, err := GetUserQuota(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 500000, cached)
}

func TestWalletHoldsPersistAtomicallyWithBatchedCounters(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	useUserCacheMiniRedis(t)
	common.BatchUpdateEnabled = true
	user := createReserveTestUser(t, 100)
	reserved, err := PreConsumeWalletQuota("wallet-batch", user.Id, 80)
	require.NoError(t, err)
	require.True(t, reserved)
	reserved, err = PreConsumeWalletQuota("wallet-overdraw", user.Id, 30)
	require.NoError(t, err)
	assert.False(t, reserved)
	require.NoError(t, SettleWalletPreConsume("wallet-batch", user.Id, 70))
	batchUpdate()
	balance, err := GetWalletBalance(user.Id)
	require.NoError(t, err)
	assert.Equal(t, WalletBalance{Quota: 30, SettledQuota: 30}, balance)
	cached, err := GetUserQuota(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 30, cached)
}

func TestWalletPreConsumeRejectsInvalidOrForeignRequests(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	user := createReserveTestUser(t, 500000)
	other := createReserveTestUser(t, 500000)
	reserved, err := PreConsumeWalletQuota("wallet-owner", user.Id, 110000)
	require.NoError(t, err)
	require.True(t, reserved)
	reserved, err = PreConsumeWalletQuota("wallet-owner", other.Id, 110000)
	require.Error(t, err)
	assert.False(t, reserved)
	assert.Error(t, SettleWalletPreConsume("wallet-owner", user.Id, -1))
	assert.Error(t, AdjustWalletPreConsume("wallet-owner", user.Id, -110001))
	assert.Error(t, SettleWalletPreConsume("wallet-owner", other.Id, 95000))
	balance, err := GetWalletBalance(user.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(500000), balance.SettledQuota)
	assert.Equal(t, int64(110000), balance.PendingQuota)
}

func TestWalletHoldRefundCannotOverflowWalletLimit(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	user := createReserveTestUser(t, 100)
	reserved, err := PreConsumeWalletQuota("wallet-limit", user.Id, 50)
	require.NoError(t, err)
	require.True(t, reserved)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("quota", common.MaxWalletQuota).Error)
	require.ErrorIs(t, AdjustWalletPreConsume("wallet-limit", user.Id, -1), ErrWalletQuotaLimitExceeded)
	require.ErrorIs(t, SettleWalletPreConsume("wallet-limit", user.Id, 49), ErrWalletQuotaLimitExceeded)
	require.ErrorIs(t, RefundWalletPreConsume("wallet-limit", user.Id), ErrWalletQuotaLimitExceeded)
	balance, err := GetWalletBalance(user.Id)
	require.NoError(t, err)
	assert.EqualValues(t, common.MaxWalletQuota, balance.Quota)
	assert.EqualValues(t, 50, balance.PendingQuota)
	assert.EqualValues(t, 1, balance.PendingRequestCount)
}
