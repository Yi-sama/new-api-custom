package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenFastModeDefaultsFalseAndLegacyNullScansFalse(t *testing.T) {
	truncateTables(t)
	token := Token{
		UserId:         1,
		Key:            "fast-mode-default-key",
		Name:           "fast-mode-default",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}
	require.NoError(t, token.Insert())

	var inserted Token
	require.NoError(t, DB.First(&inserted, token.Id).Error)
	assert.False(t, inserted.FastMode)

	require.NoError(t, DB.Model(&Token{}).Where("id = ?", token.Id).Update("fast_mode", nil).Error)
	var legacy Token
	require.NoError(t, DB.First(&legacy, token.Id).Error)
	assert.False(t, legacy.FastMode)
}

func TestUpdateFastModeOnlyChangesFastModeAndInvalidatesCache(t *testing.T) {
	truncateTables(t)
	useUserCacheMiniRedis(t)
	token := Token{
		UserId:         7,
		Key:            "fast-mode-cache-key",
		Name:           "preserve-settings",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		RemainQuota:    123,
		UnlimitedQuota: false,
		Group:          "default",
		FastMode:       true,
	}
	require.NoError(t, token.Insert())
	require.NoError(t, cacheSetTokenForTest(token))
	cached, err := cacheGetTokenByKey(token.Key)
	require.NoError(t, err)
	assert.True(t, cached.FastMode)

	update := Token{Id: token.Id, Key: token.Key, FastMode: false}
	require.NoError(t, update.UpdateFastMode())
	_, err = cacheGetTokenByKey(token.Key)
	require.Error(t, err, "fast mode updates must invalidate the pre-update cache")

	var persisted Token
	require.NoError(t, DB.First(&persisted, token.Id).Error)
	assert.False(t, persisted.FastMode)
	assert.Equal(t, token.Name, persisted.Name)
	assert.Equal(t, token.RemainQuota, persisted.RemainQuota)
	assert.Equal(t, token.UnlimitedQuota, persisted.UnlimitedQuota)
	assert.Equal(t, token.Group, persisted.Group)

	update.FastMode = true
	require.NoError(t, update.UpdateFastMode())
	require.NoError(t, DB.First(&persisted, token.Id).Error)
	assert.True(t, persisted.FastMode)
}

func TestLegacyTokenCacheWithoutFastModeRehydratesWithoutLosingQuota(t *testing.T) {
	truncateTables(t)
	server := useUserCacheMiniRedis(t)
	token := Token{
		UserId:         8,
		Key:            "legacy-fast-mode-cache-key",
		Name:           "legacy-fast-mode",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		FastMode:       true,
	}
	require.NoError(t, token.Insert())
	require.NoError(t, cacheSetTokenForTest(token))
	require.NoError(t, common.RDB.HSet(t.Context(), getTokenCacheKey(token.Key), "RemainQuota", 37).Err())
	require.NoError(t, common.RDB.HDel(t.Context(), getTokenCacheKey(token.Key), "FastMode").Err())

	code, err := cacheInitToken(token)
	require.NoError(t, err)
	assert.Equal(t, 2, code)
	cached, err := cacheGetTokenByKey(token.Key)
	require.NoError(t, err)
	assert.True(t, cached.FastMode)

	require.NoError(t, common.RDB.HDel(t.Context(), getTokenCacheKey(token.Key), "FastMode").Err())
	_, err = cacheGetTokenByKey(token.Key)
	require.Error(t, err)
	assert.True(t, server.Exists(getTokenCacheKey(token.Key)))

	reloaded, err := GetTokenByKey(token.Key, false)
	require.NoError(t, err)
	assert.True(t, reloaded.FastMode)
	cached, err = cacheGetTokenByKey(token.Key)
	require.NoError(t, err)
	assert.True(t, cached.FastMode)
	assert.Equal(t, 37, cached.RemainQuota)
}
