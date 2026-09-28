package model

import (
	"context"
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const inactiveUserDaysOption = "InactiveUserDays"

var ErrUserNotInactive = errors.New("account is no longer eligible for inactive-account cleanup")

// BeforeCreate starts observation independently of optional usage logs.
func (user *User) BeforeCreate(_ *gorm.DB) error {
	if user.APIActivityStartedAt == 0 {
		user.APIActivityStartedAt = common.GetTimestamp()
	}
	return nil
}

// Historical activity is unknown: grant a full observation window, never
// backdate it from an incomplete or pruned consumption log.
func InitializeUserAPIActivity() error {
	return DB.Model(&User{}).
		Where("api_activity_started_at = 0 OR api_activity_started_at IS NULL").
		Update("api_activity_started_at", common.GetTimestamp()).Error
}

func RecordUserAPICall(ctx context.Context, userID int, at int64) error {
	if userID <= 0 || at <= 0 {
		return errors.New("invalid API activity")
	}
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user User
		// Use the same user lock as deletion. A request either records activity
		// before eligibility is checked, or fails before reaching the relay.
		if err := lockForUpdate(tx).Select("id", "last_api_call_at", "api_activity_started_at").
			Where("id = ? AND status = ?", userID, common.UserStatusEnabled).First(&user).Error; err != nil {
			return err
		}
		if user.APIActivityStartedAt <= 0 {
			user.APIActivityStartedAt = at
		}
		return tx.Model(&user).Updates(map[string]interface{}{
			"last_api_call_at":        max(user.LastAPICallAt, at),
			"api_activity_started_at": user.APIActivityStartedAt,
		}).Error
	})
}

func ValidateInactiveUserDays(days int) error {
	if days < 1 || days > 3650 {
		return errors.New("inactivity days must be an integer from 1 to 3650")
	}
	return nil
}

func GetInactiveUserDays() (int, error) {
	var option Option
	err := DB.Where(&Option{Key: inactiveUserDaysOption}).First(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 30, nil
	}
	if err != nil {
		return 0, err
	}
	days, err := strconv.Atoi(option.Value)
	if err != nil {
		return 0, err
	}
	return days, ValidateInactiveUserDays(days)
}

func SaveInactiveUserDays(days int) error {
	if err := ValidateInactiveUserDays(days); err != nil {
		return err
	}
	return UpdateOptionsBulk(map[string]string{inactiveUserDaysOption: strconv.Itoa(days)})
}

type InactiveUser struct {
	ID                   int    `json:"id" gorm:"column:id"`
	Username             string `json:"username"`
	DisplayName          string `json:"display_name"`
	Email                string `json:"email"`
	CreatedAt            int64  `json:"created_at"`
	LastAPICallAt        int64  `json:"last_api_call_at"`
	APIActivityStartedAt int64  `json:"api_activity_started_at"`
	InactiveDays         int64  `json:"inactive_days" gorm:"-"`
}

type InactiveUserPage struct {
	Items            []InactiveUser `json:"items"`
	Total            int64          `json:"total"`
	Page             int            `json:"page"`
	PageSize         int            `json:"page_size"`
	Cutoff           int64          `json:"cutoff"`
	ObservationCount int64          `json:"observation_count"`
}

func inactiveUsersQuery(db *gorm.DB, cutoff int64) *gorm.DB {
	return db.Model(&User{}).Where("role = ?", common.RoleCommonUser).
		Where("api_activity_started_at > 0 AND api_activity_started_at < ?", cutoff).
		Where("created_at < ? AND last_api_call_at < ?", cutoff, cutoff)
}

func ListInactiveUsers(ctx context.Context, days, page, pageSize int, now int64) (*InactiveUserPage, error) {
	if err := ValidateInactiveUserDays(days); err != nil {
		return nil, err
	}
	if page < 1 || page > 1000000 || pageSize < 1 || pageSize > 100 {
		return nil, errors.New("invalid pagination")
	}
	result := &InactiveUserPage{
		Items: []InactiveUser{}, Page: page, PageSize: pageSize, Cutoff: now - int64(days)*86400,
	}
	db := DB.WithContext(ctx)
	if err := inactiveUsersQuery(db, result.Cutoff).Count(&result.Total).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&User{}).Where("role = ?", common.RoleCommonUser).
		Where("api_activity_started_at IS NULL OR api_activity_started_at = 0 OR api_activity_started_at >= ? OR created_at >= ?", result.Cutoff, result.Cutoff).
		Count(&result.ObservationCount).Error; err != nil {
		return nil, err
	}
	err := inactiveUsersQuery(db, result.Cutoff).
		Select("id", "username", "display_name", "email", "created_at", "last_api_call_at", "api_activity_started_at").
		Order("id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&result.Items).Error
	if err != nil {
		return nil, err
	}
	for i := range result.Items {
		user := &result.Items[i]
		user.InactiveDays = (now - max(user.CreatedAt, user.LastAPICallAt, user.APIActivityStartedAt)) / 86400
	}
	return result, nil
}

func InactiveUserIDs(ctx context.Context, cutoff int64) ([]int, error) {
	ids := []int{}
	err := inactiveUsersQuery(DB.WithContext(ctx), cutoff).Order("id ASC").Pluck("id", &ids).Error
	return ids, err
}

func DeleteInactiveUser(id int, cutoff int64) error {
	if id <= 0 || cutoff <= 0 {
		return ErrUserNotInactive
	}
	return (&User{Id: id}).hardDelete(&cutoff)
}
