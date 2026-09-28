package model

import (
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const NikoGroupSyncOptionKey = "NikoGroupSyncConfig"

var defaultNikoGroupMappings []NikoGroupMapping

func normalizeNikoGroupMappings(mappings []NikoGroupMapping) []NikoGroupMapping {
	if len(mappings) == 0 {
		return append([]NikoGroupMapping(nil), defaultNikoGroupMappings...)
	}
	return append([]NikoGroupMapping(nil), mappings...)
}

type NikoGroupMapping struct {
	ID          string   `json:"id,omitempty"`
	Enabled     bool     `json:"enabled"`
	LocalGroup  string   `json:"local_group"`
	RemoteGroup string   `json:"remote_group"`
	RatioMode   string   `json:"ratio_mode"`
	ManualRatio *float64 `json:"manual_ratio,omitempty"`
}

type NikoGroupSyncConfig struct {
	Enabled         bool               `json:"enabled"`
	SourceURL       string             `json:"source_url"`
	TimeoutSeconds  int                `json:"timeout_seconds"`
	RatioAdjustment float64            `json:"ratio_adjustment"`
	BaseGroupRatio  map[string]float64 `json:"base_group_ratio,omitempty"`
	Mappings        []NikoGroupMapping `json:"mappings"`
}

func DefaultNikoGroupSyncConfig() NikoGroupSyncConfig {
	return NikoGroupSyncConfig{
		Enabled:         false,
		SourceURL:       "https://us.nikoapi.xyz/api/pricing",
		TimeoutSeconds:  10,
		RatioAdjustment: 0,
		Mappings:        append([]NikoGroupMapping(nil), defaultNikoGroupMappings...),
	}
}

func DefaultNikoGroupSyncConfigJSONString() string {
	data, err := common.Marshal(DefaultNikoGroupSyncConfig())
	if err != nil {
		return `{"enabled":false,"source_url":"https://us.nikoapi.xyz/api/pricing","timeout_seconds":10,"ratio_adjustment":0,"mappings":[]}`
	}
	return string(data)
}

func ParseNikoGroupSyncConfig(raw string) (NikoGroupSyncConfig, error) {
	cfg := DefaultNikoGroupSyncConfig()
	if strings.TrimSpace(raw) == "" {
		return cfg, nil
	}
	if err := common.UnmarshalJsonStr(raw, &cfg); err != nil {
		return cfg, err
	}
	if cfg.SourceURL == "" {
		cfg.SourceURL = DefaultNikoGroupSyncConfig().SourceURL
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 10
	}
	if cfg.TimeoutSeconds > 120 {
		return cfg, fmt.Errorf("timeout_seconds must be between 1 and 120")
	}
	cfg.Mappings = normalizeNikoGroupMappings(cfg.Mappings)
	if math.IsNaN(cfg.RatioAdjustment) || math.IsInf(cfg.RatioAdjustment, 0) {
		return cfg, fmt.Errorf("ratio_adjustment must be finite")
	}
	if cfg.RatioAdjustment < 0 || cfg.RatioAdjustment > 10 {
		return cfg, fmt.Errorf("ratio_adjustment must be between 0 and 10")
	}
	for i := range cfg.Mappings {
		m := &cfg.Mappings[i]
		if strings.TrimSpace(m.LocalGroup) == "" || strings.TrimSpace(m.RemoteGroup) == "" {
			continue
		}
		if m.RatioMode == "" {
			m.RatioMode = "remote"
		}
		if m.RatioMode != "remote" && m.RatioMode != "manual" {
			return cfg, fmt.Errorf("mapping %d ratio_mode must be remote or manual", i+1)
		}
		if m.RatioMode == "manual" {
			if m.ManualRatio == nil || math.IsNaN(*m.ManualRatio) || math.IsInf(*m.ManualRatio, 0) || *m.ManualRatio < 0 || *m.ManualRatio > 10 {
				return cfg, fmt.Errorf("mapping %d manual_ratio must be between 0 and 10", i+1)
			}
		}
	}
	return cfg, nil
}

func ValidateNikoGroupSyncConfig(raw string) error {
	_, err := ParseNikoGroupSyncConfig(raw)
	return err
}
