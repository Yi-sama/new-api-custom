package model

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrQQBenefitInvalid      = errors.New("QQ_BENEFIT_INVALID")
	ErrQQBenefitNeedStart    = errors.New("QQ_BENEFIT_NEED_START")
	ErrQQBenefitAlreadyBound = errors.New("QQ_BENEFIT_ALREADY_BOUND")
	ErrQQBenefitInvalidCode  = errors.New("QQ_BENEFIT_INVALID_CODE")
	ErrQQBenefitSession      = errors.New("QQ_BENEFIT_SESSION_INVALID")
	qqBenefitIDPattern       = regexp.MustCompile(`^[1-9][0-9]{4,11}$`)
	qqBenefitCodePattern     = regexp.MustCompile(`^[0-9]{6}$`)
)

// A request is created only by the trusted bot after /领福利.
type QQBenefitRequest struct {
	QQ            string `gorm:"type:varchar(12);primaryKey"`
	GroupID       string `gorm:"type:varchar(12);not null"`
	RequestID     string `gorm:"type:varchar(64);not null"`
	MessageID     string `gorm:"type:varchar(128);not null"`
	CreatedAt     time.Time
	ExpiresAt     time.Time
	UserID        int
	SessionID     string `gorm:"type:varchar(64)"`
	CodeHash      string `json:"-" gorm:"type:char(64)"`
	CodeExpiresAt time.Time
	Attempts      int
	ConsumedAt    *time.Time
}

// This is also the permanent reward ledger. Account deletion must not remove it.
type QQBenefitBinding struct {
	QQ           string    `json:"qq" gorm:"type:varchar(12);primaryKey"`
	UserID       int       `json:"-" gorm:"not null;uniqueIndex"`
	GroupID      string    `json:"-" gorm:"type:varchar(12);not null"`
	AwardedQuota int       `json:"-" gorm:"not null"`
	AwardedAt    time.Time `json:"awarded_at"`
}

func ValidQQBenefitIdentity(qq, groupID string) bool {
	return qqBenefitIDPattern.MatchString(qq) && qqBenefitIDPattern.MatchString(groupID)
}

func GetQQBenefitBinding(qq string, userID int) (*QQBenefitBinding, error) {
	var binding QQBenefitBinding
	query := DB
	if qq != "" {
		query = query.Where("qq = ?", qq)
	} else if userID > 0 {
		query = query.Where("user_id = ?", userID)
	} else {
		return nil, ErrQQBenefitInvalid
	}
	err := query.First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &binding, err
}

func QQBenefitCommandsAllowed(qq string) (bool, error) {
	var count int64
	err := DB.Table("qq_benefit_bindings AS binding").
		Joins("JOIN users ON users.id = binding.user_id").
		Where("binding.qq = ? AND users.status = ? AND users.deleted_at IS NULL", qq, common.UserStatusEnabled).
		Count(&count).Error
	return count == 1, err
}

func StartQQBenefit(qq, groupID, messageID string) (*QQBenefitRequest, bool, error) {
	if !ValidQQBenefitIdentity(qq, groupID) || messageID == "" || len(messageID) > 128 {
		return nil, false, ErrQQBenefitInvalid
	}
	binding, err := GetQQBenefitBinding(qq, 0)
	if err != nil {
		return nil, false, err
	}
	if binding != nil {
		allowed, err := QQBenefitCommandsAllowed(qq)
		if err != nil {
			return nil, false, err
		}
		if !allowed {
			return nil, false, ErrQQBenefitSession
		}
		return nil, true, nil
	}
	now := time.Now()
	request := QQBenefitRequest{
		QQ: qq, GroupID: groupID, RequestID: common.GetUUID(), MessageID: messageID,
		CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute),
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		var previous QQBenefitRequest
		err := lockForUpdate(tx).Where("qq = ?", qq).First(&previous).Error
		if err == nil && previous.GroupID == groupID && previous.ConsumedAt == nil && previous.ExpiresAt.After(now) &&
			(previous.MessageID == messageID || previous.Attempts < 5 && now.Sub(previous.CreatedAt) < 30*time.Second) {
			request = previous
			return nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "qq"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"group_id", "request_id", "message_id", "created_at", "expires_at",
				"user_id", "session_id", "code_hash", "code_expires_at", "attempts", "consumed_at",
			}),
		}).Create(&request).Error
	})
	return &request, false, err
}

func qqBenefitSessionUser(tx *gorm.DB, userID int, sessionID string) (*User, error) {
	if userID <= 0 || sessionID == "" {
		return nil, ErrQQBenefitSession
	}
	var user User
	if err := tx.First(&user, userID).Error; err != nil {
		return nil, err
	}
	if user.Status != common.UserStatusEnabled {
		return nil, ErrQQBenefitSession
	}
	var session UserSession
	if err := tx.Where("sid = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		return nil, ErrQQBenefitSession
	}
	if session.Status != UserSessionStatusActive || session.RevokedAt != 0 ||
		session.ExpiresAt <= time.Now().Unix() || session.UserAuthVersion != user.AuthVersion {
		return nil, ErrQQBenefitSession
	}
	return &user, nil
}

func qqBenefitCodeHash(request *QQBenefitRequest, code string) string {
	return common.GenerateHMACWithKey([]byte("qq-benefit-v1:"+common.SessionSecret),
		request.QQ+":"+request.RequestID+":"+code)
}

func CreateQQBenefitCode(userID int, sessionID, qq string) (string, time.Time, error) {
	if !qqBenefitIDPattern.MatchString(qq) {
		return "", time.Time{}, ErrQQBenefitInvalid
	}
	random, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", time.Time{}, err
	}
	code := fmt.Sprintf("%06d", random.Int64())
	var expiresAt time.Time
	err = DB.Transaction(func(tx *gorm.DB) error {
		if _, err := qqBenefitSessionUser(tx, userID, sessionID); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&QQBenefitBinding{}).Where("qq = ? OR user_id = ?", qq, userID).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return ErrQQBenefitAlreadyBound
		}
		var request QQBenefitRequest
		if err := lockForUpdate(tx).Where("qq = ?", qq).First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQQBenefitNeedStart
			}
			return err
		}
		now := time.Now()
		if request.ConsumedAt != nil || !request.ExpiresAt.After(now) || request.Attempts >= 5 {
			return ErrQQBenefitNeedStart
		}
		expiresAt = now.Add(5 * time.Minute)
		if request.ExpiresAt.Before(expiresAt) {
			expiresAt = request.ExpiresAt
		}
		result := tx.Model(&QQBenefitRequest{}).
			Where("qq = ? AND request_id = ? AND consumed_at IS NULL AND expires_at > ?", qq, request.RequestID, now).
			Updates(map[string]interface{}{
				"user_id": userID, "session_id": sessionID,
				"code_hash": qqBenefitCodeHash(&request, code), "code_expires_at": expiresAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrQQBenefitNeedStart
		}
		return nil
	})
	return code, expiresAt, err
}

func ConfirmQQBenefit(qq, groupID, code string) (*QQBenefitBinding, bool, error) {
	if !ValidQQBenefitIdentity(qq, groupID) || !qqBenefitCodePattern.MatchString(code) {
		return nil, false, ErrQQBenefitInvalid
	}
	existing, err := GetQQBenefitBinding(qq, 0)
	if err != nil || existing != nil {
		return existing, false, err
	}
	var binding QQBenefitBinding
	var rejected error
	err = DB.Transaction(func(tx *gorm.DB) error {
		var request QQBenefitRequest
		if err := lockForUpdate(tx).Where("qq = ? AND group_id = ?", qq, groupID).First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQQBenefitNeedStart
			}
			return err
		}
		now := time.Now()
		if request.ConsumedAt != nil || !request.ExpiresAt.After(now) || request.Attempts >= 5 {
			return ErrQQBenefitNeedStart
		}
		expected := qqBenefitCodeHash(&request, code)
		if request.CodeHash == "" || !request.CodeExpiresAt.After(now) ||
			subtle.ConstantTimeCompare([]byte(expected), []byte(request.CodeHash)) != 1 {
			rejected = ErrQQBenefitInvalidCode
			return tx.Model(&QQBenefitRequest{}).
				Where("qq = ? AND request_id = ? AND consumed_at IS NULL", qq, request.RequestID).
				Update("attempts", gorm.Expr("attempts + 1")).Error
		}
		if _, err := qqBenefitSessionUser(tx, request.UserID, request.SessionID); err != nil {
			return err
		}
		// Consuming the exact code and crediting the wallet are one transaction.
		result := tx.Model(&QQBenefitRequest{}).
			Where("qq = ? AND request_id = ? AND code_hash = ? AND consumed_at IS NULL AND attempts < ? AND expires_at > ? AND code_expires_at > ?",
				qq, request.RequestID, request.CodeHash, 5, now, now).
			Update("consumed_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrQQBenefitInvalidCode
		}
		if math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) || common.QuotaPerUnit <= 0 {
			return ErrInvalidTopUpQuota
		}
		quota, err := common.WalletQuotaFromDecimalStrict(decimal.NewFromFloat(common.QuotaPerUnit))
		if err != nil {
			return err
		}
		binding = QQBenefitBinding{
			QQ: qq, UserID: request.UserID, GroupID: groupID, AwardedQuota: quota, AwardedAt: now,
		}
		if err := tx.Create(&binding).Error; err != nil {
			return err
		}
		return creditTopUpQuota(tx, binding.UserID, quota, nil)
	})
	if err != nil {
		return nil, false, err
	}
	if rejected != nil {
		return nil, false, rejected
	}
	syncCreditUserQuotaCache(binding.UserID, binding.AwardedQuota, "qq-benefit")
	RecordLog(binding.UserID, LogTypeSystem, "QQ group first binding benefit: $1 API credit")
	return &binding, true, nil
}
