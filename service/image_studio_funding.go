package service

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

const BillingSourceImageStudio = "image_studio"

func imageStudioCreditEligible(c *gin.Context, info *relaycommon.RelayInfo) bool {
	if info == nil || !info.IsPlayground || !c.GetBool("image_studio_authenticated") ||
		c.Request == nil || c.Request.Method != http.MethodPost {
		return false
	}
	switch c.Request.URL.Path {
	case "/pg/chat/completions", "/pg/responses", "/pg/images/generations", "/pg/images/edits",
		"/pg/audio/speech", "/pg/audio/transcriptions", "/pg/audio/translations":
		return true
	default:
		return false
	}
}

// ImageStudioFunding reserves restricted credit first, retaining the user's paid billing preference.
type ImageStudioFunding struct {
	context           *gin.Context
	info              *relaycommon.RelayInfo
	fallback          *BillingSession
	giftReserved      int
	totalReserved     int
	committed         bool
	refunded          bool
	pendingGiftRefund int
}

func (s *ImageStudioFunding) Source() string              { return BillingSourceImageStudio }
func (s *ImageStudioFunding) PreConsume(amount int) error { return s.Reserve(amount) }

func (s *ImageStudioFunding) newFallback(amount int, allowEmptyWallet bool) error {
	info := *s.info
	info.Billing = nil
	info.ForcePreConsume = true
	session, apiErr := newRegularBillingSession(s.context, &info, amount, allowEmptyWallet)
	if apiErr != nil {
		return apiErr
	}
	s.fallback = session
	return nil
}

func (s *ImageStudioFunding) Reserve(amount int) error {
	gift, err := model.ReserveImageStudioCredit(s.info.UserId, amount)
	if err != nil {
		return err
	}
	paid := amount - gift
	if paid > 0 {
		if s.fallback == nil {
			err = s.newFallback(paid, false)
		} else {
			err = s.fallback.Reserve(s.fallback.preConsumedQuota + paid)
		}
		if err != nil {
			if refundErr := model.RefundImageStudioCredit(s.info.UserId, gift); refundErr != nil {
				common.SysError("failed to restore studio credit after reserve failure: " + refundErr.Error())
			}
			return err
		}
	}
	s.giftReserved += gift
	s.totalReserved += amount
	s.syncInfo()
	return nil
}

func (s *ImageStudioFunding) Settle(delta int) error {
	if s.committed {
		return s.finishGiftRefund()
	}
	actual := s.totalReserved + delta
	if actual < 0 {
		return errors.New("negative studio settlement")
	}
	extraGift := 0
	if actual > s.giftReserved {
		var err error
		extraGift, err = model.ReserveImageStudioCredit(s.info.UserId, actual-s.giftReserved)
		if err != nil {
			return err
		}
	}
	giftUsed := min(s.giftReserved+extraGift, actual)
	paid := actual - giftUsed
	var err error
	if paid > 0 && s.fallback == nil {
		// Delivered usage must still be charged when the estimate was too small.
		err = s.newFallback(0, true)
	}
	if err == nil && s.fallback != nil {
		err = s.fallback.Settle(paid)
	}
	if err != nil {
		if refundErr := model.RefundImageStudioCredit(s.info.UserId, extraGift); refundErr != nil {
			common.SysError("failed to restore extra studio credit: " + refundErr.Error())
		}
		return err
	}
	s.committed = true
	s.pendingGiftRefund = s.giftReserved + extraGift - giftUsed
	s.giftReserved += extraGift
	s.totalReserved = actual
	s.info.ImageStudioGiftConsumed = giftUsed
	s.info.ImageStudioPaidConsumed = paid
	s.syncInfo()
	return s.finishGiftRefund()
}

func (s *ImageStudioFunding) finishGiftRefund() error {
	if err := model.RefundImageStudioCredit(s.info.UserId, s.pendingGiftRefund); err != nil {
		return err
	}
	s.giftReserved -= s.pendingGiftRefund
	s.pendingGiftRefund = 0
	s.syncInfo()
	return nil
}

func (s *ImageStudioFunding) Refund() error {
	if s.committed || s.refunded {
		return nil
	}
	if s.fallback != nil {
		if !s.fallback.refunded {
			if err := s.fallback.funding.Refund(); err != nil {
				return err
			}
			s.fallback.refunded = true
		}
		if s.fallback.extraReserved > 0 && s.fallback.funding.Source() == BillingSourceSubscription {
			if err := model.PostConsumeUserSubscriptionDelta(s.fallback.relayInfo.SubscriptionId, -int64(s.fallback.extraReserved)); err != nil {
				return err
			}
		}
		s.fallback = nil
	}
	if err := model.RefundImageStudioCredit(s.info.UserId, s.giftReserved); err != nil {
		return err
	}
	s.giftReserved = 0
	s.refunded = true
	return nil
}

func (s *ImageStudioFunding) syncInfo() {
	s.info.ImageStudioGiftPreConsumed = s.giftReserved
	if s.fallback == nil {
		return
	}
	base := s.fallback.relayInfo
	s.info.ImageStudioFallbackSource = base.BillingSource
	s.info.SubscriptionId = base.SubscriptionId
	s.info.SubscriptionPreConsumed = base.SubscriptionPreConsumed
	s.info.SubscriptionPostDelta = base.SubscriptionPostDelta
	s.info.SubscriptionPlanId = base.SubscriptionPlanId
	s.info.SubscriptionPlanTitle = base.SubscriptionPlanTitle
	s.info.SubscriptionAmountTotal = base.SubscriptionAmountTotal
	s.info.SubscriptionAmountUsedAfterPreConsume = base.SubscriptionAmountUsedAfterPreConsume
}
