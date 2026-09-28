package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRechargeEpayRejectsMismatchedPaymentAmount(t *testing.T) {
	truncateTables(t)

	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	user := insertUserForPaymentGuardTest(t, 507, 0)
	order := createEpayTestOrder(t, user.Id, "EPAYTESTMONEY", PaymentProviderEpay, common.TopUpStatusPending)
	order.Money = 1.00
	require.NoError(t, DB.Save(&order).Error)

	_, err := RechargeEpay(order.TradeNo, "alipay", "1.01", "127.0.0.1")
	require.ErrorIs(t, err, ErrPaymentAmountMismatch)
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, user.Id))
	assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, order.TradeNo))

	alreadyDone, err := RechargeEpay(order.TradeNo, "alipay", "1.00", "127.0.0.1")
	require.NoError(t, err)
	assert.False(t, alreadyDone)
	assert.Equal(t, 2*500000, getUserQuotaForPaymentGuardTest(t, user.Id))
}
