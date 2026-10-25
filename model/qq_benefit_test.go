package model

import (
	"bytes"
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupQQBenefit(t *testing.T) (*User, string) {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&QQBenefitRequest{}, &QQBenefitBinding{}))
	for _, row := range []interface{}{&QQBenefitRequest{}, &QQBenefitBinding{}} {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(row).Error)
	}
	t.Cleanup(func() {
		for _, row := range []interface{}{&QQBenefitRequest{}, &QQBenefitBinding{}} {
			require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(row).Error)
		}
	})
	previous := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = previous })
	user := &User{Username: "qq-benefit-user", Password: "test", Status: common.UserStatusEnabled, AuthVersion: 1, Quota: 17}
	require.NoError(t, DB.Create(user).Error)
	session := UserSession{
		SID: "qq-benefit-session", UserID: user.Id, UserAuthVersion: user.AuthVersion,
		Status: UserSessionStatusActive, RefreshHash: "unused-test-hash",
		LoginMethod: "password", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, DB.Create(&session).Error)
	return user, session.SID
}

func TestQQBenefitRequiresGroupInitiationAndLiveSession(t *testing.T) {
	user, sid := setupQQBenefit(t)
	_, _, err := CreateQQBenefitCode(user.Id, sid, "100001")
	assert.ErrorIs(t, err, ErrQQBenefitNeedStart)
	_, bound, err := StartQQBenefit("100001", "1128511890", "message-1")
	require.NoError(t, err)
	assert.False(t, bound)
	code, expiry, err := CreateQQBenefitCode(user.Id, sid, "100001")
	require.NoError(t, err)
	assert.Regexp(t, `^[0-9]{6}$`, code)
	assert.True(t, expiry.After(time.Now()))
	var request QQBenefitRequest
	require.NoError(t, DB.First(&request, "qq = ?", "100001").Error)
	assert.NotEqual(t, code, request.CodeHash)
	require.NoError(t, DB.Model(&UserSession{}).Where("sid = ?", sid).Update("status", UserSessionStatusRevoked).Error)
	_, _, err = ConfirmQQBenefit("100001", "1128511890", code)
	assert.ErrorIs(t, err, ErrQQBenefitSession)
	require.NoError(t, DB.First(user, user.Id).Error)
	assert.Equal(t, 17, user.Quota)
}

func TestQQBenefitCannotBeClaimedByAnotherQQOrGroup(t *testing.T) {
	user, sid := setupQQBenefit(t)
	_, _, err := StartQQBenefit("100001", "1128511890", "message-1")
	require.NoError(t, err)
	code, _, err := CreateQQBenefitCode(user.Id, sid, "100001")
	require.NoError(t, err)
	for _, identity := range [][2]string{{"100002", "1128511890"}, {"100001", "1125690897"}} {
		_, _, err = ConfirmQQBenefit(identity[0], identity[1], code)
		assert.ErrorIs(t, err, ErrQQBenefitNeedStart)
	}
	_, granted, err := ConfirmQQBenefit("100001", "1128511890", code)
	require.NoError(t, err)
	assert.True(t, granted)
	require.NoError(t, DB.First(user, user.Id).Error)
	assert.Equal(t, 500017, user.Quota)
}

func TestQQBenefitMovingRequestBetweenGroupsDoesNotDuplicateReward(t *testing.T) {
	user, sid := setupQQBenefit(t)
	first, _, err := StartQQBenefit("100001", "1128511890", "message-1")
	require.NoError(t, err)
	_, _, err = CreateQQBenefitCode(user.Id, sid, "100001")
	require.NoError(t, err)
	second, bound, err := StartQQBenefit("100001", "1125690897", "message-2")
	require.NoError(t, err)
	assert.False(t, bound)
	assert.Equal(t, "1125690897", second.GroupID)
	assert.NotEqual(t, first.RequestID, second.RequestID)
	assert.Empty(t, second.CodeHash)
	code, _, err := CreateQQBenefitCode(user.Id, sid, "100001")
	require.NoError(t, err)
	_, _, err = ConfirmQQBenefit("100001", "1128511890", code)
	assert.ErrorIs(t, err, ErrQQBenefitNeedStart)
	_, granted, err := ConfirmQQBenefit("100001", "1125690897", code)
	require.NoError(t, err)
	assert.True(t, granted)
	request, bound, err := StartQQBenefit("100001", "1128511890", "message-3")
	require.NoError(t, err)
	assert.Nil(t, request)
	assert.True(t, bound)
	_, granted, err = ConfirmQQBenefit("100001", "1128511890", code)
	require.NoError(t, err)
	assert.False(t, granted)
	require.NoError(t, DB.First(user, user.Id).Error)
	assert.Equal(t, 500017, user.Quota)
	var count int64
	require.NoError(t, DB.Model(&QQBenefitBinding{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestQQBenefitOnlyCreditsOnceAndPreservesOwnershipAfterDeletion(t *testing.T) {
	user, sid := setupQQBenefit(t)
	_, _, err := StartQQBenefit("100001", "1128511890", "message-1")
	require.NoError(t, err)
	code, _, err := CreateQQBenefitCode(user.Id, sid, "100001")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		binding, granted, err := ConfirmQQBenefit("100001", "1128511890", code)
		require.NoError(t, err)
		assert.Equal(t, i == 0, granted)
		assert.Equal(t, user.Id, binding.UserID)
	}
	allowed, err := QQBenefitCommandsAllowed("100001")
	require.NoError(t, err)
	assert.True(t, allowed)
	_, _, err = StartQQBenefit("100002", "1128511890", "message-2")
	require.NoError(t, err)
	_, _, err = CreateQQBenefitCode(user.Id, sid, "100002")
	assert.ErrorIs(t, err, ErrQQBenefitAlreadyBound)
	require.NoError(t, DB.First(user, user.Id).Error)
	assert.Equal(t, 500017, user.Quota)
	require.NoError(t, DB.Delete(user).Error)
	allowed, err = QQBenefitCommandsAllowed("100001")
	require.NoError(t, err)
	assert.False(t, allowed)
	binding, err := GetQQBenefitBinding("100001", 0)
	require.NoError(t, err)
	require.NotNil(t, binding)
	assert.Equal(t, 500000, binding.AwardedQuota)
}

func TestQQBenefitCodeExpiresAndFiveFailuresRequireNewInitiation(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(map[bool]string{false: "five failures", true: "expired code"}[expired], func(t *testing.T) {
			user, sid := setupQQBenefit(t)
			_, _, err := StartQQBenefit("100001", "1128511890", "message-1")
			require.NoError(t, err)
			code, _, err := CreateQQBenefitCode(user.Id, sid, "100001")
			require.NoError(t, err)
			wrong := "000000"
			if wrong == code {
				wrong = "000001"
			}
			if expired {
				require.NoError(t, DB.Model(&QQBenefitRequest{}).Where("qq = ?", "100001").
					Update("code_expires_at", time.Now().Add(-time.Minute)).Error)
				_, _, err = ConfirmQQBenefit("100001", "1128511890", code)
				assert.ErrorIs(t, err, ErrQQBenefitInvalidCode)
			} else {
				for i := 0; i < 5; i++ {
					_, _, err = ConfirmQQBenefit("100001", "1128511890", wrong)
					assert.ErrorIs(t, err, ErrQQBenefitInvalidCode)
				}
				_, _, err = ConfirmQQBenefit("100001", "1128511890", code)
				assert.ErrorIs(t, err, ErrQQBenefitNeedStart)
				_, _, err = StartQQBenefit("100001", "1128511890", "fresh-message")
				require.NoError(t, err)
				_, _, err = CreateQQBenefitCode(user.Id, sid, "100001")
				assert.NoError(t, err)
			}
			require.NoError(t, DB.First(user, user.Id).Error)
			assert.Equal(t, 17, user.Quota)
		})
	}
}

func TestQQBenefitSixDigitCodePreservesLeadingZerosAndRejectsOtherLengths(t *testing.T) {
	user, sid := setupQQBenefit(t)
	_, _, err := StartQQBenefit("100001", "1128511890", "message-1")
	require.NoError(t, err)
	previousReader := rand.Reader
	rand.Reader = bytes.NewReader([]byte{0, 0, 0})
	t.Cleanup(func() { rand.Reader = previousReader })
	code, _, err := CreateQQBenefitCode(user.Id, sid, "100001")
	rand.Reader = previousReader
	require.NoError(t, err)
	assert.Equal(t, "000000", code)
	for _, invalid := range []string{"00000", "0000000", "00000000", "abcdef"} {
		_, _, err := ConfirmQQBenefit("100001", "1128511890", invalid)
		assert.ErrorIs(t, err, ErrQQBenefitInvalid)
	}
	_, granted, err := ConfirmQQBenefit("100001", "1128511890", code)
	require.NoError(t, err)
	assert.True(t, granted)
}

func TestQQBenefitWalletFailureRollsBackBindingAndCodeConsumption(t *testing.T) {
	user, sid := setupQQBenefit(t)
	require.NoError(t, DB.Model(user).Update("quota", common.MaxWalletQuota).Error)
	_, _, err := StartQQBenefit("100001", "1128511890", "message-1")
	require.NoError(t, err)
	code, _, err := CreateQQBenefitCode(user.Id, sid, "100001")
	require.NoError(t, err)
	_, _, err = ConfirmQQBenefit("100001", "1128511890", code)
	assert.ErrorIs(t, err, ErrTopUpQuotaLimitExceeded)
	binding, err := GetQQBenefitBinding("100001", 0)
	require.NoError(t, err)
	assert.Nil(t, binding)
	var request QQBenefitRequest
	require.NoError(t, DB.First(&request, "qq = ?", "100001").Error)
	assert.Nil(t, request.ConsumedAt)
	require.NoError(t, DB.Model(user).Update("quota", 0).Error)
	_, granted, err := ConfirmQQBenefit("100001", "1128511890", code)
	require.NoError(t, err)
	assert.True(t, granted)
}

func TestQQBenefitConcurrentConfirmationDoesNotDoubleCredit(t *testing.T) {
	user, sid := setupQQBenefit(t)
	_, _, err := StartQQBenefit("100001", "1128511890", "message-1")
	require.NoError(t, err)
	code, _, err := CreateQQBenefitCode(user.Id, sid, "100001")
	require.NoError(t, err)
	results := make(chan bool, 2)
	var wait sync.WaitGroup
	for i := 0; i < 2; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, granted, err := ConfirmQQBenefit("100001", "1128511890", code)
			results <- err == nil && granted
		}()
	}
	wait.Wait()
	close(results)
	credits := 0
	for granted := range results {
		if granted {
			credits++
		}
	}
	assert.Equal(t, 1, credits)
	require.NoError(t, DB.First(user, user.Id).Error)
	assert.Equal(t, 500017, user.Quota)
}
