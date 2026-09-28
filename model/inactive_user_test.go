package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInactiveUsersRespectObservationActivityRolesAndPagination(t *testing.T) {
	truncateTables(t)
	now := int64(1800000000)
	cutoff := now - 30*86400
	for i, data := range []struct {
		role                 int
		start, last, created int64
		deleted              bool
	}{
		{common.RoleCommonUser, cutoff - 86400, 0, cutoff - 86400, false},
		{common.RoleCommonUser, cutoff - 86400, cutoff - 1, cutoff - 86400, false},
		{common.RoleCommonUser, cutoff - 86400, cutoff, cutoff - 86400, false},
		{common.RoleCommonUser, now, 0, cutoff - 86400, false},
		{common.RoleCommonUser, 0, 0, cutoff - 86400, false},
		{common.RoleAdminUser, cutoff - 86400, 0, cutoff - 86400, false},
		{common.RoleRootUser, cutoff - 86400, 0, cutoff - 86400, false},
		{common.RoleCommonUser, cutoff - 86400, 0, now, false},
		{common.RoleCommonUser, cutoff - 86400, 0, cutoff - 86400, true},
	} {
		user := User{Id: i + 1, Username: fmt.Sprintf("inactive-%d", i), AffCode: fmt.Sprintf("inactive-%d", i), Password: "unused",
			Role: data.role, CreatedAt: data.created, LastAPICallAt: data.last, APIActivityStartedAt: data.start}
		require.NoError(t, DB.Create(&user).Error)
		if data.start == 0 {
			require.NoError(t, DB.Model(&user).Update("api_activity_started_at", 0).Error)
		}
		if data.deleted {
			require.NoError(t, DB.Delete(&user).Error)
		}
	}
	page, err := ListInactiveUsers(t.Context(), 30, 1, 1, now)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, 1, page.Items[0].ID)
	assert.EqualValues(t, 31, page.Items[0].InactiveDays)
	assert.EqualValues(t, 2, page.Total)
	assert.EqualValues(t, 3, page.ObservationCount)
	page, err = ListInactiveUsers(t.Context(), 30, 2, 1, now)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, 2, page.Items[0].ID)
	ids, err := InactiveUserIDs(t.Context(), cutoff)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, ids)
}

func TestInactiveUserMigrationPreservesKnownActivityAndProtectsLegacyUsers(t *testing.T) {
	truncateTables(t)
	user := User{Username: "legacy-observation", Password: "unused", CreatedAt: 100}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Model(&user).Update("api_activity_started_at", 0).Error)
	before := time.Now().Unix()
	require.NoError(t, InitializeUserAPIActivity())
	require.NoError(t, DB.First(&user, user.Id).Error)
	assert.GreaterOrEqual(t, user.APIActivityStartedAt, before)
	assert.Zero(t, user.LastAPICallAt)
	start := user.APIActivityStartedAt
	require.NoError(t, InitializeUserAPIActivity())
	require.NoError(t, DB.First(&user, user.Id).Error)
	assert.Equal(t, start, user.APIActivityStartedAt)
	require.ErrorIs(t, DeleteInactiveUser(user.Id, before-86400), ErrUserNotInactive)
}

func TestInactiveDeletionRechecksActivityAndCleansAuthentication(t *testing.T) {
	truncateTables(t)
	server := useUserCacheMiniRedis(t)
	now := common.GetTimestamp()
	user := User{Username: "inactive-delete", Password: "unused", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, AuthVersion: 1, CreatedAt: now - 90*86400,
		APIActivityStartedAt: now - 90*86400}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&Token{UserId: user.Id, Key: "inactive-delete-token"}).Error)
	require.NoError(t, DB.Create(&UserSession{SID: "inactive-session", UserID: user.Id,
		Version: 1, UserAuthVersion: 1, Status: UserSessionStatusActive, RefreshHash: "inactive-refresh"}).Error)
	cutoff := now - 30*86400
	ids, err := InactiveUserIDs(t.Context(), cutoff)
	require.NoError(t, err)
	assert.Contains(t, ids, user.Id)
	require.NoError(t, RecordUserAPICall(t.Context(), user.Id, now))
	require.ErrorIs(t, DeleteInactiveUser(user.Id, cutoff), ErrUserNotInactive)
	// A later observation window can legitimately expire.
	require.NoError(t, DeleteInactiveUser(user.Id, now+31*86400))
	var count int64
	for _, record := range []any{&Token{}, &UserSession{}} {
		require.NoError(t, DB.Unscoped().Model(record).Where("user_id = ?", user.Id).Count(&count).Error)
		assert.Zero(t, count)
	}
	require.NoError(t, DB.Unscoped().Model(&User{}).Where("id = ?", user.Id).Count(&count).Error)
	assert.Zero(t, count)
	assert.True(t, server.Exists(getUserAuthVersionKey(user.Id)))
	require.ErrorIs(t, RecordUserAPICall(t.Context(), user.Id, now+1), gorm.ErrRecordNotFound)
}

func TestInactiveDeletionRejectsPromotedUser(t *testing.T) {
	truncateTables(t)
	user := User{Username: "inactive-promoted", Password: "unused", Role: common.RoleCommonUser,
		CreatedAt: 100, APIActivityStartedAt: 100}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Model(&user).Update("role", common.RoleAdminUser).Error)
	require.ErrorIs(t, DeleteInactiveUser(user.Id, 200), ErrUserNotInactive)
}

func TestInactiveUserSettingsPersistAndRejectInvalidDays(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}))
	require.NoError(t, DB.Where(&Option{Key: inactiveUserDaysOption}).Delete(&Option{}).Error)
	common.OptionMapRWMutex.Lock()
	oldMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		require.NoError(t, DB.Where(&Option{Key: inactiveUserDaysOption}).Delete(&Option{}).Error)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldMap
		common.OptionMapRWMutex.Unlock()
	})
	days, err := GetInactiveUserDays()
	require.NoError(t, err)
	assert.Equal(t, 30, days)
	require.NoError(t, SaveInactiveUserDays(45))
	for _, invalid := range []int{-1, 0, 3651} {
		require.Error(t, SaveInactiveUserDays(invalid))
	}
	days, err = GetInactiveUserDays()
	require.NoError(t, err)
	assert.Equal(t, 45, days)
}

func TestAPIActivityIsMonotonicAndCannotBeOverwrittenByProfileUpdates(t *testing.T) {
	truncateTables(t)
	oldLogEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() { common.LogConsumeEnabled = oldLogEnabled })
	user := User{Username: "activity-profile", Password: "unused", Status: common.UserStatusEnabled,
		Role: common.RoleCommonUser, APIActivityStartedAt: 100}
	require.NoError(t, DB.Create(&user).Error)
	stale := user
	require.NoError(t, RecordUserAPICall(t.Context(), user.Id, 300))
	require.NoError(t, RecordUserAPICall(t.Context(), user.Id, 200))
	stale.LastAPICallAt = 150
	stale.APIActivityStartedAt = 1
	stale.DisplayName = "Changed"
	require.NoError(t, stale.Update(false))
	require.NoError(t, DB.First(&user, user.Id).Error)
	assert.EqualValues(t, 300, user.LastAPICallAt)
	assert.EqualValues(t, 100, user.APIActivityStartedAt)
	assert.Equal(t, "Changed", user.DisplayName)
}
