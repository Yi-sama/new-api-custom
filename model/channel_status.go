package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const ChannelStatusConfigOptionKey = "ChannelStatusConfig"

const (
	ChannelStatusModeUser        = "user"
	ChannelStatusModeAPI         = "api"
	ChannelStatusModeUserThenAPI = "user_then_api"
)

type ChannelStatusGroupConfig struct {
	Group   string   `json:"group"`
	Enabled bool     `json:"enabled"`
	Models  []string `json:"models"`
}

type ChannelStatusConfig struct {
	Enabled bool                       `json:"enabled"`
	Mode    string                     `json:"mode"`
	Groups  []ChannelStatusGroupConfig `json:"groups"`
}

type ChannelStatusCheck struct {
	ID        int64  `json:"id" gorm:"primaryKey"`
	RequestID string `json:"request_id" gorm:"type:varchar(64);index"`
	ChannelID int    `json:"channel_id" gorm:"index;index:channel_status_lookup,priority:1"`
	Group     string `json:"group" gorm:"type:varchar(64);index;index:channel_status_lookup,priority:2"`
	Model     string `json:"model" gorm:"type:varchar(255);index;index:channel_status_lookup,priority:3"`
	Status    string `json:"status" gorm:"type:varchar(16);index"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error" gorm:"type:text"`
	Source    string `json:"source" gorm:"type:varchar(16);index"`
	CheckedAt int64  `json:"checked_at" gorm:"bigint;index;index:channel_status_lookup,priority:4"`
}

func DefaultChannelStatusConfig() ChannelStatusConfig {
	return ChannelStatusConfig{Enabled: false, Mode: ChannelStatusModeUser, Groups: []ChannelStatusGroupConfig{}}
}

func DefaultChannelStatusConfigJSONString() string {
	data, err := common.Marshal(DefaultChannelStatusConfig())
	if err != nil {
		return `{"enabled":false,"mode":"user","groups":[]}`
	}
	return string(data)
}

func ParseChannelStatusConfig(raw string) (ChannelStatusConfig, error) {
	cfg := DefaultChannelStatusConfig()
	if strings.TrimSpace(raw) == "" {
		return cfg, nil
	}
	if err := common.UnmarshalJsonStr(raw, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Mode == "" {
		cfg.Mode = ChannelStatusModeUser
	}
	if cfg.Mode != ChannelStatusModeUser && cfg.Mode != ChannelStatusModeAPI && cfg.Mode != ChannelStatusModeUserThenAPI {
		return cfg, fmt.Errorf("渠道状态统计方式无效")
	}
	seenGroups := make(map[string]bool, len(cfg.Groups))
	for i := range cfg.Groups {
		group := strings.TrimSpace(cfg.Groups[i].Group)
		if group == "" {
			return cfg, fmt.Errorf("第 %d 个分组不能为空", i+1)
		}
		if seenGroups[group] {
			return cfg, fmt.Errorf("分组 %q 重复", group)
		}
		seenGroups[group] = true
		cfg.Groups[i].Group = group
		seenModels := map[string]bool{}
		models := cfg.Groups[i].Models[:0]
		for _, value := range cfg.Groups[i].Models {
			model := strings.TrimSpace(value)
			if model != "" && !seenModels[model] {
				seenModels[model] = true
				models = append(models, model)
			}
		}
		cfg.Groups[i].Models = models
		if cfg.Groups[i].Enabled && len(models) == 0 {
			return cfg, fmt.Errorf("分组 %q 至少需要一个检测模型", group)
		}
	}
	return cfg, nil
}

func ValidateChannelStatusConfig(raw string) error {
	_, err := ParseChannelStatusConfig(raw)
	return err
}

func SortedChannelStatusModels(channels []*Channel, group string) []string {
	seen := map[string]bool{}
	for _, channel := range channels {
		if channel == nil || !containsString(channel.GetGroups(), group) {
			continue
		}
		for _, value := range channel.GetModels() {
			if model := strings.TrimSpace(value); model != "" {
				seen[model] = true
			}
		}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}
