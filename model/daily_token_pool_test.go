package model

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepareDailyPoolTest(t *testing.T) DailyTokenPoolConfig {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Option{}, &DailyTokenPoolDay{}, &DailyTokenPoolReservation{}))
	t.Cleanup(func() {
		require.NoError(t, DB.Where("1 = 1").Delete(&DailyTokenPoolReservation{}).Error)
		require.NoError(t, DB.Where("1 = 1").Delete(&DailyTokenPoolDay{}).Error)
		require.NoError(t, DB.Where(&Option{Key: DailyTokenPoolOptionKey}).Delete(&Option{}).Error)
	})
	config := DefaultDailyTokenPoolConfig()
	config.Enabled = true
	config.DailyLimit = 100
	return config
}

func TestDailyTokenPoolConcurrentSharedAdmissionAndIdempotentSettlement(t *testing.T) {
	config := prepareDailyPoolTest(t)
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	type result struct {
		id  string
		err error
	}
	results := make(chan result, 4)
	var workers sync.WaitGroup
	for range 4 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			id, err := ReserveDailyTokenPool(config, 30, now)
			results <- result{id, err}
		}()
	}
	workers.Wait()
	close(results)
	var ids []string
	rejected := 0
	for result := range results {
		if result.err != nil {
			require.ErrorIs(t, result.err, ErrDailyTokenPoolExhausted)
			rejected++
		} else {
			ids = append(ids, result.id)
		}
	}
	require.Len(t, ids, 3)
	assert.Equal(t, 1, rejected)
	status, err := GetDailyTokenPoolStatus(config, now)
	require.NoError(t, err)
	assert.EqualValues(t, 90, status.ReservedTokens)
	assert.EqualValues(t, 10, status.RemainingTokens)
	require.NoError(t, SettleDailyTokenPool(ids[0], 20))
	require.NoError(t, SettleDailyTokenPool(ids[0], 20))
	require.NoError(t, SettleDailyTokenPool(ids[1], 0))
	require.NoError(t, SettleDailyTokenPool(ids[2], 30))
	status, err = GetDailyTokenPoolStatus(config, now)
	require.NoError(t, err)
	assert.EqualValues(t, 50, status.UsedTokens)
	assert.Zero(t, status.ReservedTokens)
	assert.EqualValues(t, 50, status.RemainingTokens)
}

func TestDailyTokenPoolMidnightKeepsLateSettlementOnOriginalDay(t *testing.T) {
	config := prepareDailyPoolTest(t)
	now := time.Date(2026, 10, 19, 15, 59, 59, 0, time.UTC)
	id, err := ReserveDailyTokenPool(config, 80, now)
	require.NoError(t, err)
	tomorrow := now.Add(2 * time.Second)
	status, err := GetDailyTokenPoolStatus(config, tomorrow)
	require.NoError(t, err)
	assert.EqualValues(t, 100, status.RemainingTokens)
	require.NoError(t, SettleDailyTokenPool(id, 60))
	status, err = GetDailyTokenPoolStatus(config, tomorrow)
	require.NoError(t, err)
	assert.Zero(t, status.UsedTokens)
	status, err = GetDailyTokenPoolStatus(config, now)
	require.NoError(t, err)
	assert.EqualValues(t, 60, status.UsedTokens)
	assert.Equal(t, now.Add(time.Second).Unix(), status.ResetAt)
}

func TestDailyTokenPoolConfigChangesPreserveConsumptionAndMatchBothFilters(t *testing.T) {
	config := prepareDailyPoolTest(t)
	config.Groups = []string{"community"}
	config.ChannelIDs = []int{7}
	require.NoError(t, SaveDailyTokenPoolConfig(config))
	reloaded, err := GetDailyTokenPoolConfig()
	require.NoError(t, err)
	assert.True(t, reloaded.Matches("community", 7))
	assert.False(t, reloaded.Matches("paid", 7))
	assert.False(t, reloaded.Matches("community", 8))
	now := time.Now()
	id, err := ReserveDailyTokenPool(config, 80, now)
	require.NoError(t, err)
	require.NoError(t, SettleDailyTokenPool(id, 60))
	config.DailyLimit = 50
	require.NoError(t, SaveDailyTokenPoolConfig(config))
	status, err := GetDailyTokenPoolStatus(config, now)
	require.NoError(t, err)
	assert.EqualValues(t, 60, status.UsedTokens)
	assert.True(t, status.Exhausted)
	_, err = ReserveDailyTokenPool(config, 1, now)
	assert.ErrorIs(t, err, ErrDailyTokenPoolExhausted)
	config.DailyLimit = 120
	status, err = GetDailyTokenPoolStatus(config, now)
	require.NoError(t, err)
	assert.EqualValues(t, 60, status.RemainingTokens)
}

func TestDailyTokenPoolRecordsProviderOverrunWithoutHidingUsage(t *testing.T) {
	config := prepareDailyPoolTest(t)
	now := time.Now()
	id, err := ReserveDailyTokenPool(config, 50, now)
	require.NoError(t, err)
	require.NoError(t, SettleDailyTokenPool(id, 110))
	status, err := GetDailyTokenPoolStatus(config, now)
	require.NoError(t, err)
	assert.EqualValues(t, 110, status.UsedTokens)
	assert.True(t, status.Exhausted)
	assert.Zero(t, status.RemainingTokens)
	_, err = ReserveDailyTokenPool(config, 1, now)
	assert.True(t, errors.Is(err, ErrDailyTokenPoolExhausted))
}

func TestDailyTokenPoolResetUsesCalendarMidnightAcrossDST(t *testing.T) {
	config := DefaultDailyTokenPoolConfig()
	config.ResetTimezone = "America/New_York"
	start := time.Date(2026, 11, 1, 4, 0, 0, 0, time.UTC)
	_, next, err := config.Window(start)
	require.NoError(t, err)
	assert.Equal(t, 25*time.Hour, next.Sub(start))
	config.ResetTimezone = "../invalid"
	assert.Error(t, config.Validate())
	config = DefaultDailyTokenPoolConfig()
	config.DailyLimit = 0
	assert.Error(t, config.Validate())
}
