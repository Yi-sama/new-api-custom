package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

const BillingSourceTavern = "tavern"

// TavernFunding uses restricted credit first and delegates paid usage to the
// existing wallet/subscription session, retaining the user's billing preference.
type TavernFunding struct {
	context           *gin.Context
	info              *relaycommon.RelayInfo
	fallback          *BillingSession
	giftReserved      int
	totalReserved     int
	committed         bool
	refunded          bool
	pendingGiftRefund int
}

func (t *TavernFunding) Source() string { return BillingSourceTavern }

func (t *TavernFunding) PreConsume(amount int) error {
	return t.Reserve(amount)
}

func (t *TavernFunding) newFallback(amount int, allowEmptyWallet bool) error {
	info := *t.info
	info.Billing = nil
	info.ForcePreConsume = true
	session, apiErr := newRegularBillingSession(t.context, &info, amount, allowEmptyWallet)
	if apiErr != nil {
		return apiErr
	}
	t.fallback = session
	return nil
}

func (t *TavernFunding) Reserve(amount int) error {
	gift, err := model.ReserveTavernCredit(t.info.UserId, amount)
	if err != nil {
		return err
	}
	paid := amount - gift
	if paid > 0 {
		if t.fallback == nil {
			err = t.newFallback(paid, false)
		} else {
			err = t.fallback.Reserve(t.fallback.preConsumedQuota + paid)
		}
		if err != nil {
			if refundErr := model.RefundTavernCredit(t.info.UserId, gift); refundErr != nil {
				common.SysError("failed to restore tavern credit after reserve failure: " + refundErr.Error())
			}
			return err
		}
	}
	t.giftReserved += gift
	t.totalReserved += amount
	t.syncInfo()
	return nil
}

func (t *TavernFunding) Settle(delta int) error {
	if t.committed {
		return t.finishGiftRefund()
	}
	actual := t.totalReserved + delta
	if actual < 0 {
		return errors.New("negative tavern settlement")
	}
	extraGift := 0
	if actual > t.giftReserved {
		var err error
		extraGift, err = model.ReserveTavernCredit(t.info.UserId, actual-t.giftReserved)
		if err != nil {
			return err
		}
	}
	giftUsed := min(t.giftReserved+extraGift, actual)
	paid := actual - giftUsed
	var err error
	if paid > 0 && t.fallback == nil {
		// The request is already delivered. Like regular wallet settlement,
		// an underestimated precharge may leave wallet debt, never a free call.
		err = t.newFallback(0, true)
	}
	if err == nil && t.fallback != nil {
		err = t.fallback.Settle(paid)
	}
	if err != nil {
		if refundErr := model.RefundTavernCredit(t.info.UserId, extraGift); refundErr != nil {
			common.SysError("failed to restore extra tavern credit: " + refundErr.Error())
		}
		return err
	}
	t.committed = true
	t.pendingGiftRefund = t.giftReserved + extraGift - giftUsed
	t.giftReserved += extraGift
	t.totalReserved = actual
	t.info.TavernGiftConsumed = giftUsed
	t.info.TavernPaidConsumed = paid
	t.syncInfo()
	return t.finishGiftRefund()
}

func (t *TavernFunding) finishGiftRefund() error {
	if err := model.RefundTavernCredit(t.info.UserId, t.pendingGiftRefund); err != nil {
		return err
	}
	t.giftReserved -= t.pendingGiftRefund
	t.pendingGiftRefund = 0
	t.syncInfo()
	return nil
}

func (t *TavernFunding) Refund() error {
	if t.committed || t.refunded {
		return nil
	}
	if t.fallback != nil {
		if !t.fallback.refunded {
			if err := t.fallback.funding.Refund(); err != nil {
				return err
			}
			t.fallback.refunded = true
		}
		if t.fallback.extraReserved > 0 && t.fallback.funding.Source() == BillingSourceSubscription {
			if err := model.PostConsumeUserSubscriptionDelta(t.fallback.relayInfo.SubscriptionId, -int64(t.fallback.extraReserved)); err != nil {
				return err
			}
		}
		t.fallback = nil
	}
	if err := model.RefundTavernCredit(t.info.UserId, t.giftReserved); err != nil {
		return err
	}
	t.giftReserved = 0
	t.refunded = true
	return nil
}

func (t *TavernFunding) syncInfo() {
	t.info.TavernGiftPreConsumed = t.giftReserved
	if t.fallback == nil {
		return
	}
	base := t.fallback.relayInfo
	t.info.TavernFallbackSource = base.BillingSource
	t.info.SubscriptionId = base.SubscriptionId
	t.info.SubscriptionPreConsumed = base.SubscriptionPreConsumed
	t.info.SubscriptionPostDelta = base.SubscriptionPostDelta
	t.info.SubscriptionPlanId = base.SubscriptionPlanId
	t.info.SubscriptionPlanTitle = base.SubscriptionPlanTitle
	t.info.SubscriptionAmountTotal = base.SubscriptionAmountTotal
	t.info.SubscriptionAmountUsedAfterPreConsume = base.SubscriptionAmountUsedAfterPreConsume
}
