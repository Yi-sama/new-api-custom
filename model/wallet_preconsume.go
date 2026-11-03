package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	walletPreConsumePending  = "pending"
	walletPreConsumeSettled  = "settled"
	walletPreConsumeRefunded = "refunded"
)

// WalletPreConsumeRecord separates unsettled holds from completed wallet charges.
type WalletPreConsumeRecord struct {
	Id          int    `gorm:"primaryKey"`
	RequestId   string `gorm:"type:varchar(64);not null;uniqueIndex"`
	UserId      int    `gorm:"not null;index:idx_wallet_hold_user_status,priority:1"`
	Quota       int64  `gorm:"type:bigint;not null"`
	ActualQuota int64  `gorm:"type:bigint;not null;default:0"`
	Status      string `gorm:"type:varchar(16);not null;index:idx_wallet_hold_user_status,priority:2"`
	CreatedAt   int64  `gorm:"type:bigint;not null"`
	UpdatedAt   int64  `gorm:"type:bigint;not null;index"`
}

type WalletBalance struct {
	Quota               int64 `json:"quota"`
	SettledQuota        int64 `json:"settled_quota"`
	PendingQuota        int64 `json:"pending_quota"`
	PendingRequestCount int64 `json:"pending_request_count"`
}

func GetWalletBalance(userId int) (WalletBalance, error) {
	var balance WalletBalance
	holds := DB.Model(&WalletPreConsumeRecord{}).
		Select("user_id, SUM(quota) AS pending_quota, COUNT(*) AS pending_request_count").
		Where("user_id = ? AND status = ?", userId, walletPreConsumePending).
		Group("user_id")
	// Read both components in one statement so settlement cannot split the snapshot.
	result := DB.Model(&User{}).
		Select("users.quota, users.quota + COALESCE(holds.pending_quota, 0) AS settled_quota, COALESCE(holds.pending_quota, 0) AS pending_quota, COALESCE(holds.pending_request_count, 0) AS pending_request_count").
		Joins("LEFT JOIN (?) AS holds ON holds.user_id = users.id", holds).
		Where("users.id = ?", userId).Scan(&balance)
	if result.Error != nil {
		return balance, result.Error
	}
	if result.RowsAffected != 1 {
		return balance, gorm.ErrRecordNotFound
	}
	return balance, nil
}

// PreConsumeWalletQuota commits the wallet debit and its display hold together.
// Wallet holds are persisted immediately, including when other counters are batched.
func PreConsumeWalletQuota(requestId string, userId int, quota int) (bool, error) {
	if requestId == "" || len(requestId) > 64 || quota <= 0 || quota > common.MaxWalletQuota {
		return false, errors.New("invalid wallet pre-consume")
	}
	if common.RedisEnabled {
		// Hydration must run outside the transaction (SQLite can have one connection).
		if _, err := GetUserCache(userId); err != nil {
			return false, err
		}
	}
	reserved, applied, cached := false, false, false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id").First(&user, userId).Error; err != nil {
			return err
		}
		var existing WalletPreConsumeRecord
		err := tx.Where("request_id = ?", requestId).First(&existing).Error
		if err == nil {
			if existing.UserId != userId || existing.Quota != int64(quota) || existing.Status != walletPreConsumePending {
				return errors.New("wallet pre-consume request conflict")
			}
			reserved = true
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if common.RedisEnabled {
			result, cacheErr := cacheTryReserveUserQuota(userId, int64(quota))
			if cacheErr == nil && result == cacheQuotaInsufficient {
				return nil
			}
			cached = cacheErr == nil && result == cacheQuotaOK
			if cacheErr != nil {
				common.SysError("wallet hold cache reserve failed: " + cacheErr.Error())
			}
		}
		update := tx.Model(&User{}).Where("id = ?", userId)
		if !cached {
			update = update.Where("quota >= ?", quota)
		}
		result := update.Update("quota", gorm.Expr("quota - ?", quota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		now := common.GetTimestamp()
		if err := tx.Create(&WalletPreConsumeRecord{
			RequestId: requestId, UserId: userId, Quota: int64(quota),
			Status: walletPreConsumePending, CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			return err
		}
		reserved = true
		applied = true
		return nil
	})
	if cached && err != nil {
		syncWalletQuotaCache(userId, int64(quota))
	}
	if err == nil && applied && !cached && common.RedisEnabled {
		if cacheErr := invalidateUserCache(userId); cacheErr != nil {
			common.SysError("wallet hold cache invalidation failed: " + cacheErr.Error())
		}
	}
	return reserved && err == nil, err
}

// AdjustWalletPreConsume changes an in-flight reservation without charging it yet.
func AdjustWalletPreConsume(requestId string, userId int, delta int) error {
	if delta == 0 {
		return nil
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "quota").First(&user, userId).Error; err != nil {
			return err
		}
		if delta < 0 && int64(user.Quota) > int64(common.MaxWalletQuota)+int64(delta) {
			return ErrWalletQuotaLimitExceeded
		}
		var record WalletPreConsumeRecord
		err := tx.Where("request_id = ? AND user_id = ?", requestId, userId).First(&record).Error
		if errors.Is(err, gorm.ErrRecordNotFound) && delta > 0 {
			if requestId == "" || len(requestId) > 64 || delta > common.MaxWalletQuota {
				return errors.New("invalid wallet pre-consume adjustment")
			}
			now := common.GetTimestamp()
			record = WalletPreConsumeRecord{RequestId: requestId, UserId: userId, Status: walletPreConsumePending,
				CreatedAt: now, UpdatedAt: now}
			if err = tx.Create(&record).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if record.Status != walletPreConsumePending || int64(delta) > int64(common.MaxWalletQuota)-record.Quota ||
			int64(delta) < -record.Quota {
			return errors.New("invalid wallet pre-consume adjustment")
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("quota", gorm.Expr("quota - ?", delta)).Error; err != nil {
			return err
		}
		if record.Quota+int64(delta) == 0 {
			return tx.Delete(&record).Error
		}
		return tx.Model(&record).Updates(map[string]interface{}{
			"quota": record.Quota + int64(delta), "updated_at": common.GetTimestamp(),
		}).Error
	})
	if err == nil {
		syncWalletQuotaCache(userId, -int64(delta))
	}
	return err
}

func SettleWalletPreConsume(requestId string, userId int, actualQuota int) error {
	return finishWalletPreConsume(requestId, userId, actualQuota, walletPreConsumeSettled)
}

func RefundWalletPreConsume(requestId string, userId int) error {
	return finishWalletPreConsume(requestId, userId, 0, walletPreConsumeRefunded)
}

func finishWalletPreConsume(requestId string, userId int, actualQuota int, status string) error {
	if actualQuota < 0 || actualQuota > common.MaxWalletQuota {
		return errors.New("invalid wallet settlement quota")
	}
	var delta int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "quota").First(&user, userId).Error; err != nil {
			return err
		}
		var record WalletPreConsumeRecord
		if err := tx.Where("request_id = ? AND user_id = ?", requestId, userId).First(&record).Error; err != nil {
			return err
		}
		if record.Status != walletPreConsumePending {
			if record.Status == status && record.ActualQuota == int64(actualQuota) {
				return nil
			}
			return errors.New("wallet pre-consume already finalized")
		}
		delta = record.Quota - int64(actualQuota)
		if delta > 0 && int64(user.Quota) > int64(common.MaxWalletQuota)-delta {
			return ErrWalletQuotaLimitExceeded
		}
		if delta != 0 {
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("quota", gorm.Expr("quota + ?", delta)).Error; err != nil {
				return err
			}
		}
		return tx.Model(&record).Updates(map[string]interface{}{
			"status": status, "actual_quota": actualQuota, "updated_at": common.GetTimestamp(),
		}).Error
	})
	if err == nil && delta != 0 {
		syncWalletQuotaCache(userId, delta)
	}
	return err
}

func syncWalletQuotaCache(userId int, delta int64) {
	if !common.RedisEnabled || delta == 0 {
		return
	}
	if err := cacheIncrUserQuota(userId, delta); err != nil {
		common.SysError(fmt.Sprintf("wallet hold cache synchronization failed (user=%d): %s", userId, err))
		if invalidationErr := invalidateUserCache(userId); invalidationErr != nil {
			common.SysError("wallet hold cache invalidation failed: " + invalidationErr.Error())
		}
	}
}

func CleanupWalletPreConsumeRecords(olderThanSeconds int64) (int64, error) {
	if olderThanSeconds <= 0 {
		return 0, errors.New("invalid wallet pre-consume retention")
	}
	result := DB.
		Where("status <> ? AND updated_at < ?", walletPreConsumePending, common.GetTimestamp()-olderThanSeconds).
		Delete(&WalletPreConsumeRecord{})
	return result.RowsAffected, result.Error
}
