package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ImageStudioCredit is a one-time allowance restricted to the signed Studio bridge.
type ImageStudioCredit struct {
	UserId       int   `json:"user_id" gorm:"primaryKey;autoIncrement:false"`
	Quota        int   `json:"quota" gorm:"not null"`
	GrantedQuota int   `json:"granted_quota" gorm:"not null"`
	CreatedTime  int64 `json:"created_time"`
}

func ClaimImageStudioCredit(userId int) (*ImageStudioCredit, bool, error) {
	if userId <= 0 {
		return nil, false, errors.New("invalid studio user")
	}
	quota := common.QuotaFromFloat(common.QuotaPerUnit)
	credit := ImageStudioCredit{UserId: userId, Quota: quota, GrantedQuota: quota, CreatedTime: common.GetTimestamp()}
	result := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&credit)
	if result.Error != nil {
		return nil, false, result.Error
	}
	current, err := GetImageStudioCredit(userId)
	return current, result.RowsAffected == 1, err
}

func GetImageStudioCredit(userId int) (*ImageStudioCredit, error) {
	var credit ImageStudioCredit
	err := DB.Where("user_id = ?", userId).First(&credit).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &ImageStudioCredit{UserId: userId}, nil
	}
	return &credit, err
}

func ReserveImageStudioCredit(userId, amount int) (int, error) {
	if amount < 0 {
		return 0, errors.New("negative studio reserve")
	}
	if amount == 0 {
		return 0, nil
	}
	reserved := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		var credit ImageStudioCredit
		err := lockForUpdate(tx).Where("user_id = ?", userId).First(&credit).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		reserved = max(0, min(amount, credit.Quota))
		if reserved == 0 {
			return nil
		}
		result := tx.Model(&ImageStudioCredit{}).Where("user_id = ? AND quota = ?", userId, credit.Quota).
			Update("quota", gorm.Expr("quota - ?", reserved))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("studio credit reservation conflict")
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return reserved, nil
}

func RefundImageStudioCredit(userId, amount int) error {
	if amount < 0 {
		return errors.New("negative studio refund")
	}
	if amount == 0 {
		return nil
	}
	result := DB.Model(&ImageStudioCredit{}).
		Where("user_id = ? AND granted_quota - quota >= ?", userId, amount).
		Update("quota", gorm.Expr("quota + ?", amount))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("invalid studio credit refund")
	}
	return nil
}
