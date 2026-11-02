package model

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const DailyTokenPoolOptionKey = "DailyTokenPoolConfig"
const MaxDailyTokenPoolLimit int64 = 1_000_000_000_000

var ErrDailyTokenPoolExhausted = errors.New("daily shared token pool has insufficient available tokens")

type DailyTokenPoolConfig struct {
	Enabled       bool     `json:"enabled"`
	FreeUsage     bool     `json:"free_usage"`
	Title         string   `json:"title"`
	DailyLimit    int64    `json:"daily_limit"`
	ResetTimezone string   `json:"reset_timezone"`
	Groups        []string `json:"groups"`
	ChannelIDs    []int    `json:"channel_ids"`
	PublicVisible bool     `json:"public_visible"`
}

type DailyTokenPoolDay struct {
	ID             string `gorm:"primaryKey;size:128"`
	UsedTokens     int64  `json:"used_tokens"`
	ReservedTokens int64  `json:"reserved_tokens"`
}

type DailyTokenPoolReservation struct {
	ID       string `gorm:"primaryKey;size:64"`
	DayID    string `gorm:"size:128;index"`
	Reserved int64
	Actual   int64
	Settled  bool
}

type DailyTokenPoolStatus struct {
	Enabled         bool   `json:"enabled"`
	Title           string `json:"title"`
	DailyLimit      int64  `json:"daily_limit"`
	UsedTokens      int64  `json:"used_tokens"`
	ReservedTokens  int64  `json:"reserved_tokens"`
	RemainingTokens int64  `json:"remaining_tokens"`
	ResetAt         int64  `json:"reset_at"`
	ResetTimezone   string `json:"reset_timezone"`
	Exhausted       bool   `json:"exhausted"`
}

func DefaultDailyTokenPoolConfig() DailyTokenPoolConfig {
	return DailyTokenPoolConfig{
		Title: "Daily shared token pool", DailyLimit: 10_000_000,
		ResetTimezone: "Asia/Shanghai", Groups: []string{},
		ChannelIDs: []int{}, PublicVisible: true,
	}
}

func (config DailyTokenPoolConfig) Validate() error {
	if config.DailyLimit < 1 || config.DailyLimit > MaxDailyTokenPoolLimit {
		return fmt.Errorf("daily_limit must be between 1 and %d", MaxDailyTokenPoolLimit)
	}
	if strings.TrimSpace(config.Title) == "" || len(config.Title) > 160 {
		return errors.New("title must contain 1 to 160 bytes")
	}
	if len(config.ResetTimezone) > 80 {
		return errors.New("invalid reset timezone")
	}
	if _, err := time.LoadLocation(config.ResetTimezone); err != nil || config.ResetTimezone == "" || config.ResetTimezone == "Local" {
		return errors.New("reset_timezone must be an explicit IANA timezone")
	}
	if len(config.Groups) > 256 || len(config.ChannelIDs) > 1024 {
		return errors.New("too many groups or channels")
	}
	if config.FreeUsage && (len(config.Groups) == 0 || len(config.ChannelIDs) == 0) {
		return errors.New("free shared usage requires explicit groups and channel_ids")
	}
	for _, group := range config.Groups {
		if strings.TrimSpace(group) == "" || len(group) > 128 || group == "auto" {
			return errors.New("groups must contain concrete routing group names")
		}
	}
	for _, id := range config.ChannelIDs {
		if id < 1 || id > 2147483647 {
			return errors.New("channel_ids must contain positive channel IDs")
		}
	}
	return nil
}

func GetDailyTokenPoolConfig() (DailyTokenPoolConfig, error) {
	config := DefaultDailyTokenPoolConfig()
	var option Option
	err := DB.Where(&Option{Key: DailyTokenPoolOptionKey}).First(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err = common.UnmarshalJsonStr(option.Value, &config); err != nil {
		return config, err
	}
	return config, config.Validate()
}

func SaveDailyTokenPoolConfig(config DailyTokenPoolConfig) error {
	if err := config.Validate(); err != nil {
		return err
	}
	if config.Groups == nil {
		config.Groups = []string{}
	}
	if config.ChannelIDs == nil {
		config.ChannelIDs = []int{}
	}
	value, err := common.Marshal(config)
	if err != nil {
		return err
	}
	option := Option{Key: DailyTokenPoolOptionKey, Value: string(value)}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&option).Error
}

func (config DailyTokenPoolConfig) Matches(group string, channelID int) bool {
	return config.Enabled &&
		(len(config.Groups) == 0 || slices.Contains(config.Groups, group)) &&
		(len(config.ChannelIDs) == 0 || slices.Contains(config.ChannelIDs, channelID))
}

func (config DailyTokenPoolConfig) Window(now time.Time) (string, time.Time, error) {
	location, err := time.LoadLocation(config.ResetTimezone)
	if err != nil {
		return "", time.Time{}, err
	}
	local := now.In(location)
	next := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, location)
	return local.Format("2006-01-02") + "@" + config.ResetTimezone, next, nil
}

func GetDailyTokenPoolStatus(config DailyTokenPoolConfig, now time.Time) (DailyTokenPoolStatus, error) {
	dayID, resetAt, err := config.Window(now)
	if err != nil {
		return DailyTokenPoolStatus{}, err
	}
	day := DailyTokenPoolDay{ID: dayID}
	err = DB.Where("id = ?", dayID).First(&day).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return DailyTokenPoolStatus{}, err
	}
	remaining := max(int64(0), config.DailyLimit-day.UsedTokens-day.ReservedTokens)
	return DailyTokenPoolStatus{
		Enabled: config.Enabled, Title: config.Title, DailyLimit: config.DailyLimit,
		UsedTokens: day.UsedTokens, ReservedTokens: day.ReservedTokens,
		RemainingTokens: remaining, ResetAt: resetAt.Unix(),
		ResetTimezone: config.ResetTimezone, Exhausted: remaining == 0,
	}, nil
}

func ReserveDailyTokenPool(config DailyTokenPoolConfig, amount int64, now time.Time) (string, error) {
	if amount < 1 || amount > config.DailyLimit {
		return "", ErrDailyTokenPoolExhausted
	}
	dayID, _, err := config.Window(now)
	if err != nil {
		return "", err
	}
	id := common.GetUUID()
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&DailyTokenPoolDay{ID: dayID}).Error; err != nil {
			return err
		}
		// Conditional UPDATE is the admission lock, including across processes.
		result := tx.Model(&DailyTokenPoolDay{}).
			Where("id = ? AND used_tokens + reserved_tokens <= ?", dayID, config.DailyLimit-amount).
			Update("reserved_tokens", gorm.Expr("reserved_tokens + ?", amount))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrDailyTokenPoolExhausted
		}
		return tx.Create(&DailyTokenPoolReservation{ID: id, DayID: dayID, Reserved: amount}).Error
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func SettleDailyTokenPool(id string, actual int64) error {
	if id == "" {
		return nil
	}
	if actual < 0 || actual > MaxDailyTokenPoolLimit {
		return errors.New("invalid shared pool token usage")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var reservation DailyTokenPoolReservation
		if err := tx.Where("id = ?", id).First(&reservation).Error; err != nil {
			return err
		}
		claimed := tx.Model(&DailyTokenPoolReservation{}).
			Where("id = ? AND settled = ?", id, false).
			Updates(map[string]interface{}{"settled": true, "actual": actual})
		if claimed.Error != nil || claimed.RowsAffected == 0 {
			return claimed.Error
		}
		result := tx.Model(&DailyTokenPoolDay{}).
			Where("id = ? AND reserved_tokens >= ?", reservation.DayID, reservation.Reserved).
			Updates(map[string]interface{}{
				"reserved_tokens": gorm.Expr("reserved_tokens - ?", reservation.Reserved),
				"used_tokens":     gorm.Expr("used_tokens + ?", actual),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("shared pool reservation ledger is inconsistent")
		}
		return nil
	})
}
