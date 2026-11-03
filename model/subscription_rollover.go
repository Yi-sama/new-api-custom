package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrSubscriptionFundingInsufficient = errors.New("subscription quota insufficient")

// ResizeSubscriptionPreConsume keeps the entire request on one funding source.
// A nil result means the reservation was atomically moved to the wallet.
func ResizeSubscriptionPreConsume(requestID string, userID int, amount int64, allowWallet, settling bool) (*SubscriptionPreConsumeResult, error) {
	if requestID == "" || userID <= 0 || amount < 0 || amount > common.MaxWalletQuota {
		return nil, errors.New("invalid subscription reservation")
	}
	now := GetDBTimestamp()
	var result *SubscriptionPreConsumeResult
	walletCharged := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var record SubscriptionPreConsumeRecord
		if err := lockForUpdate(tx).Where("request_id = ? AND user_id = ?", requestID, userID).First(&record).Error; err != nil {
			return err
		}
		if record.Status != "consumed" {
			return errors.New("subscription pre-consume already refunded")
		}
		var subs []UserSubscription
		if err := lockForUpdate(tx).Where("user_id = ?", userID).
			Order("end_time asc, id asc").Find(&subs).Error; err != nil {
			return err
		}
		original := -1
		for i := range subs {
			if subs[i].Id == record.UserSubscriptionId {
				original = i
				break
			}
		}
		if original < 0 {
			return gorm.ErrRecordNotFound
		}
		sub := &subs[original]
		usedWithoutHold := max(int64(0), sub.AmountUsed-record.PreConsumed)
		total := subscriptionGrantedAmount(sub, now)
		// Preserve authorization across expiry for requests already in flight.
		if amount <= record.PreConsumed || (!isDailySubscription(sub) && total == 0) || amount <= total-usedWithoutHold {
			delta := amount - record.PreConsumed
			if err := postConsumeUserSubscriptionDeltaTx(tx, sub.Id, delta, now); err != nil {
				return err
			}
			record.PreConsumed = amount
			result = &SubscriptionPreConsumeResult{
				UserSubscriptionId: sub.Id, PreConsumed: amount, AmountTotal: total,
				AmountUsedBefore: usedWithoutHold, AmountUsedAfter: max(int64(0), sub.AmountUsed+delta),
			}
			return tx.Save(&record).Error
		}
		for i := range subs {
			candidate := &subs[i]
			if i == original || !subscriptionCanPreConsume(candidate, now) {
				continue
			}
			if !isDailySubscription(candidate) {
				plan, err := getSubscriptionPlanByIdTx(tx, candidate.PlanId)
				if err != nil {
					return err
				}
				if err := maybeResetUserSubscriptionWithPlanTx(tx, candidate, plan, now); err != nil {
					return err
				}
			}
			total = subscriptionGrantedAmount(candidate, now)
			if (isDailySubscription(candidate) || total > 0) && amount > total-candidate.AmountUsed {
				continue
			}
			if err := postConsumeUserSubscriptionDeltaTx(tx, candidate.Id, amount, now); err != nil {
				return err
			}
			if err := postConsumeUserSubscriptionDeltaTx(tx, sub.Id, -record.PreConsumed, now); err != nil {
				return err
			}
			record.UserSubscriptionId = candidate.Id
			record.PreConsumed = amount
			result = &SubscriptionPreConsumeResult{
				UserSubscriptionId: candidate.Id, PreConsumed: amount, AmountTotal: total,
				AmountUsedBefore: candidate.AmountUsed, AmountUsedAfter: candidate.AmountUsed + amount,
			}
			return tx.Save(&record).Error
		}
		for _, candidate := range subs {
			if candidate.Status == "active" && candidate.EndTime > now && !candidate.AllowWalletOverflow {
				allowWallet = false
				break
			}
		}
		if !allowWallet {
			return ErrSubscriptionFundingInsufficient
		}
		wallet := tx.Model(&User{}).Where("id = ?", userID)
		if !settling {
			wallet = wallet.Where("quota >= ?", amount)
		}
		charged := wallet.Update("quota", gorm.Expr("quota - ?", amount))
		if charged.Error != nil {
			return charged.Error
		}
		if charged.RowsAffected != 1 {
			return fmt.Errorf("%w: wallet quota insufficient", ErrSubscriptionFundingInsufficient)
		}
		if !settling && amount > 0 {
			if err := tx.Create(&WalletPreConsumeRecord{
				RequestId: requestID, UserId: userID, Quota: amount, Status: walletPreConsumePending,
				CreatedAt: now, UpdatedAt: now,
			}).Error; err != nil {
				return err
			}
		}
		walletCharged = true
		if err := postConsumeUserSubscriptionDeltaTx(tx, sub.Id, -record.PreConsumed, now); err != nil {
			return err
		}
		record.Status = "refunded"
		if err := tx.Save(&record).Error; err != nil {
			return err
		}
		walletCharged = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if walletCharged {
		if err := cacheDecrUserQuota(userID, amount); err != nil {
			common.SysError("failed to sync subscription rollover wallet quota: " + err.Error())
		}
	}
	return result, nil
}
