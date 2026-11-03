package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingSessionSettlesWalletDisplayEvenWhenDeltaIsZero(t *testing.T) {
	truncate(t)
	const userID = 731
	seedUser(t, userID, 500000)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{UserId: userID, IsPlayground: true, ForcePreConsume: true,
		UserSetting: dto.UserSetting{BillingPreference: "wallet_only"}}
	require.Nil(t, PreConsumeBilling(ctx, 110000, info))
	balance, err := model.GetWalletBalance(userID)
	require.NoError(t, err)
	assert.Equal(t, int64(500000), balance.SettledQuota)
	require.NoError(t, info.Billing.Reserve(120000))
	balance, err = model.GetWalletBalance(userID)
	require.NoError(t, err)
	assert.Equal(t, int64(120000), balance.PendingQuota)
	assert.Equal(t, int64(500000), balance.SettledQuota)
	require.NoError(t, SettleBilling(ctx, info, 120000))
	balance, err = model.GetWalletBalance(userID)
	require.NoError(t, err)
	assert.Equal(t, model.WalletBalance{Quota: 380000, SettledQuota: 380000}, balance)
}
