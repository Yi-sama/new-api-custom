package model

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const assistantPasswordResetPurpose = "assistant_password_reset"

var ErrAssistantPasswordResetInvalid = errors.New("invalid or expired password reset code")

type assistantPasswordResetPayload struct {
	CodeHash       string    `json:"code_hash"`
	LastSentAt     time.Time `json:"last_sent_at"`
	FirstFailureAt time.Time `json:"first_failure_at"`
	Failures       int       `json:"failures"`
	AuthVersion    int64     `json:"auth_version"`
}

// The purpose-separated email HMAC identifies a shared row without storing the email or code.
func assistantPasswordResetKey(email string) string {
	return authFlowTokenHash("assistant-password-reset:" + NormalizeEmail(email))
}

func assistantPasswordResetCodeHash(email, code string) string {
	return common.GenerateHMACWithKey([]byte("assistant-password-reset-code:"+common.SessionSecret),
		NormalizeEmail(email)+":"+code)
}

func CreateAssistantPasswordReset(email string) (string, int64, error) {
	email = NormalizeEmail(email)
	number, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", 0, err
	}
	code := fmt.Sprintf("%06d", number)
	var retryAfter int64
	err = DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		flow := AuthFlow{TokenHash: assistantPasswordResetKey(email),
			Purpose: assistantPasswordResetPurpose, ExpiresAt: now.Add(10 * time.Minute)}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "token_hash"}}, DoNothing: true,
		}).Create(&flow).Error; err != nil {
			return err
		}
		if err := lockForUpdate(tx).Where("token_hash = ? AND purpose = ?",
			flow.TokenHash, assistantPasswordResetPurpose).First(&flow).Error; err != nil {
			return err
		}
		var state assistantPasswordResetPayload
		if flow.Payload != "" {
			if err := common.UnmarshalJsonStr(flow.Payload, &state); err != nil {
				return err
			}
		}
		until := state.LastSentAt.Add(time.Minute)
		if state.Failures >= 5 && state.FirstFailureAt.Add(10*time.Minute).After(until) {
			until = state.FirstFailureAt.Add(10 * time.Minute)
		}
		if until.After(now) {
			retryAfter = int64((until.Sub(now) + time.Second - 1) / time.Second)
			code = ""
			return nil
		}
		if !state.FirstFailureAt.Add(10 * time.Minute).After(now) {
			state.FirstFailureAt, state.Failures = time.Time{}, 0
		}
		var users []User
		if err := tx.Select("id", "auth_version").Where("LOWER(email) = ?", email).
			Limit(2).Find(&users).Error; err != nil {
			return err
		}
		flow.UserId = 0
		state.AuthVersion = 0
		if len(users) == 1 {
			flow.UserId, state.AuthVersion = users[0].Id, users[0].AuthVersion
		}
		state.CodeHash, state.LastSentAt = assistantPasswordResetCodeHash(email, code), now
		payload, err := common.Marshal(state)
		if err != nil {
			return err
		}
		if flow.UserId == 0 {
			code = ""
		}
		return tx.Model(&flow).Updates(map[string]interface{}{
			"user_id": flow.UserId, "payload": string(payload),
			"expires_at": now.Add(10 * time.Minute), "consumed_at": nil,
		}).Error
	})
	return code, retryAfter, err
}

func ResetAssistantPassword(email, code, password string) error {
	email = NormalizeEmail(email)
	var userID int
	invalid := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var flow AuthFlow
		if err := lockForUpdate(tx).Where("token_hash = ? AND purpose = ?",
			assistantPasswordResetKey(email), assistantPasswordResetPurpose).First(&flow).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAssistantPasswordResetInvalid
			}
			return err
		}
		now := time.Now()
		if flow.ConsumedAt != nil || !flow.ExpiresAt.After(now) {
			return ErrAssistantPasswordResetInvalid
		}
		var state assistantPasswordResetPayload
		if err := common.UnmarshalJsonStr(flow.Payload, &state); err != nil {
			return err
		}
		if !state.FirstFailureAt.Add(10 * time.Minute).After(now) {
			state.FirstFailureAt, state.Failures = time.Time{}, 0
		}
		if state.Failures >= 5 {
			return ErrAssistantPasswordResetInvalid
		}
		if flow.UserId == 0 || subtle.ConstantTimeCompare([]byte(state.CodeHash),
			[]byte(assistantPasswordResetCodeHash(email, code))) != 1 {
			if state.Failures == 0 {
				state.FirstFailureAt = now
			}
			state.Failures++
			payload, err := common.Marshal(state)
			if err != nil {
				return err
			}
			invalid = true
			// Commit failed attempts even though the public result is a rejection.
			return tx.Model(&flow).Update("payload", string(payload)).Error
		}
		var user User
		if err := lockForUpdate(tx).Where("id = ? AND LOWER(email) = ? AND auth_version = ?",
			flow.UserId, email, state.AuthVersion).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAssistantPasswordResetInvalid
			}
			return err
		}
		hashed, err := common.Password2Hash(password)
		if err != nil {
			return err
		}
		result := tx.Model(&flow).Where("consumed_at IS NULL AND expires_at > ?", now).
			Update("consumed_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrAssistantPasswordResetInvalid
		}
		if _, err := IncrementUserAuthVersionWithTx(tx, user.Id); err != nil {
			return err
		}
		userID = user.Id
		return tx.Model(&User{}).Where("id = ?", user.Id).Update("password", hashed).Error
	})
	if err != nil {
		return err
	}
	if invalid {
		return ErrAssistantPasswordResetInvalid
	}
	if err := PublishUserAuthCache(userID); err != nil {
		return err
	}
	_, err = RevokeAllUserSessions(userID, "password_reset")
	return err
}
