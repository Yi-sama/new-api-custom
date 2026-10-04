package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TavernCredit is a one-time, non-transferable chat allowance, not wallet quota.
type TavernCredit struct {
	UserId       int   `json:"user_id" gorm:"primaryKey;autoIncrement:false"`
	Quota        int   `json:"quota" gorm:"not null"`
	GrantedQuota int   `json:"granted_quota" gorm:"not null"`
	CreatedTime  int64 `json:"created_time"`
}

func ClaimTavernCredit(userId int) (*TavernCredit, bool, error) {
	if userId <= 0 {
		return nil, false, errors.New("invalid tavern user")
	}
	quota := common.QuotaFromFloat(common.QuotaPerUnit)
	credit := TavernCredit{UserId: userId, Quota: quota, GrantedQuota: quota, CreatedTime: common.GetTimestamp()}
	result := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&credit)
	if result.Error != nil {
		return nil, false, result.Error
	}
	granted := result.RowsAffected == 1
	current, err := GetTavernCredit(userId)
	return current, granted, err
}

func GetTavernCredit(userId int) (*TavernCredit, error) {
	var credit TavernCredit
	err := DB.Where("user_id = ?", userId).First(&credit).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &TavernCredit{UserId: userId}, nil
	}
	return &credit, err
}

// ReserveTavernCredit takes up to amount without allowing concurrent overdrafts.
func ReserveTavernCredit(userId, amount int) (int, error) {
	if amount < 0 {
		return 0, errors.New("negative tavern reserve")
	}
	if amount == 0 {
		return 0, nil
	}
	reserved := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		var credit TavernCredit
		err := lockForUpdate(tx).Where("user_id = ?", userId).First(&credit).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		reserved = min(amount, credit.Quota)
		if reserved <= 0 {
			reserved = 0
			return nil
		}
		result := tx.Model(&TavernCredit{}).Where("user_id = ? AND quota = ?", userId, credit.Quota).
			Update("quota", gorm.Expr("quota - ?", reserved))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("tavern credit reservation conflict")
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return reserved, nil
}

func RefundTavernCredit(userId, amount int) error {
	if amount < 0 {
		return errors.New("negative tavern refund")
	}
	if amount == 0 {
		return nil
	}
	result := DB.Model(&TavernCredit{}).
		Where("user_id = ? AND granted_quota - quota >= ?", userId, amount).
		Update("quota", gorm.Expr("quota + ?", amount))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("invalid tavern credit refund")
	}
	return nil
}
