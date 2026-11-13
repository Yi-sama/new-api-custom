package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelfWalletBalanceReturnsOnlyAuthenticatedAccount(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	db := openTokenControllerTestDB(t)
	t.Cleanup(func() { model.DB, model.LOG_DB = previousDB, previousLogDB })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.WalletPreConsumeRecord{}))
	require.NoError(t, db.Create(&[]model.User{
		{Id: 101, Username: "wallet-owner", AffCode: "wallet-owner", Quota: 500000},
		{Id: 102, Username: "another-wallet", AffCode: "another-wallet", Quota: 900000},
	}).Error)
	reserved, err := model.PreConsumeWalletQuota("controller-wallet", 101, 110000)
	require.NoError(t, err)
	require.True(t, reserved)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/balance?user_id=102", nil)
	ctx.Set("id", 101)
	GetSelfWalletBalance(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	var result struct {
		Success bool                `json:"success"`
		Data    model.WalletBalance `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
	require.True(t, result.Success)
	assert.Equal(t, model.WalletBalance{Quota: 390000, SettledQuota: 500000, PendingQuota: 110000, PendingRequestCount: 1}, result.Data)
}
