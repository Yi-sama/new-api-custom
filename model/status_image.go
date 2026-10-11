package model

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

const StatusImageConfigOptionKey = "StatusImageConfig"

// StatusImageConfig controls presentation only, never upstream availability.
type StatusImageConfig struct {
	SiteName           *string `json:"site_name"`
	ChannelTitle       *string `json:"channel_title"`
	SubTitle           *string `json:"sub_title"`
	Footer             *string `json:"footer"`
	GroupFilter        *string `json:"group_filter"`
	ChannelGroupFilter *string `json:"channel_group_filter,omitempty"`
	SubGroupFilter     *string `json:"sub_group_filter,omitempty"`
	// Absent/null preserves all models; an explicit empty list selects none.
	ChannelModelFilter *[]StatusImageModelSelection `json:"channel_model_filter,omitempty"`
	ChannelColumns     *int                         `json:"channel_columns"`
	ShowSuccessRate    *bool                        `json:"show_success_rate"`
	ShowLatency        *bool                        `json:"show_latency"`
	ShowHistory        *bool                        `json:"show_history"`
	ShowUpdatedAt      *bool                        `json:"show_updated_at"`
	ShowSubDetails     *bool                        `json:"show_sub_details"`
}

type StatusImageModelSelection struct {
	Group string `json:"group"`
	Model string `json:"model"`
}

func DefaultStatusImageConfig() StatusImageConfig {
	siteName, channelTitle, subTitle, footer, groupFilter := "Yisama的自用站", "渠道状态", "Sub2API 状态", "", ""
	columns := 2
	successRate, latency, history, updatedAt, subDetails := true, true, true, true, false
	channelGroupFilter, subGroupFilter := "", ""
	return StatusImageConfig{
		SiteName: &siteName, ChannelTitle: &channelTitle, SubTitle: &subTitle,
		Footer: &footer, GroupFilter: &groupFilter,
		ChannelGroupFilter: &channelGroupFilter, SubGroupFilter: &subGroupFilter,
		ChannelColumns: &columns, ShowSuccessRate: &successRate, ShowLatency: &latency,
		ShowHistory: &history, ShowUpdatedAt: &updatedAt, ShowSubDetails: &subDetails,
	}
}

func DefaultStatusImageConfigJSONString() string {
	data, _ := common.Marshal(DefaultStatusImageConfig())
	return string(data)
}

func ValidateStatusImageConfig(raw string) error {
	if len(raw) > 2*1024*1024 || !strings.HasPrefix(strings.TrimSpace(raw), "{") {
		return fmt.Errorf("invalid status image configuration")
	}
	var cfg StatusImageConfig
	if err := common.UnmarshalJsonStr(raw, &cfg); err != nil {
		return err
	}
	for _, field := range []struct {
		name  string
		value string
		max   int
	}{
		{"site_name", stringValue(cfg.SiteName), 60},
		{"channel_title", stringValue(cfg.ChannelTitle), 60},
		{"sub_title", stringValue(cfg.SubTitle), 60},
	} {
		if strings.TrimSpace(field.value) == "" || utf8.RuneCountInString(field.value) > field.max {
			return fmt.Errorf("%s must contain 1-%d characters", field.name, field.max)
		}
	}
	if cfg.Footer == nil || cfg.GroupFilter == nil || cfg.ChannelColumns == nil ||
		cfg.ShowSuccessRate == nil || cfg.ShowLatency == nil || cfg.ShowHistory == nil ||
		cfg.ShowUpdatedAt == nil || cfg.ShowSubDetails == nil {
		return fmt.Errorf("status image configuration has missing fields")
	}
	if utf8.RuneCountInString(*cfg.Footer) > 160 || utf8.RuneCountInString(*cfg.GroupFilter) > 500 {
		return fmt.Errorf("status image footer or group filter is too long")
	}
	for _, filter := range []*string{cfg.ChannelGroupFilter, cfg.SubGroupFilter} {
		if utf8.RuneCountInString(stringValue(filter)) > 500 {
			return fmt.Errorf("status image group filter is too long")
		}
	}
	if cfg.ChannelModelFilter != nil {
		if len(*cfg.ChannelModelFilter) > 500 {
			return fmt.Errorf("select at most 500 group-model pairs")
		}
		for _, selection := range *cfg.ChannelModelFilter {
			for _, name := range []string{selection.Group, selection.Model} {
				if strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 500 {
					return fmt.Errorf("selected group and model names must contain 1-500 characters")
				}
			}
		}
	}
	if *cfg.ChannelColumns != 1 && *cfg.ChannelColumns != 2 {
		return fmt.Errorf("channel_columns must be 1 or 2")
	}
	return nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
