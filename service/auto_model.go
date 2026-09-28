package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const (
	AutoModelStrategyPrice = "price"
	AutoModelStrategyOrder = "order"
)

type AutoModelConfig struct {
	Models        []string
	Strategy      string
	Custom        bool
	AllowedModels []string
	DefaultModels []string
}

func NormalizeAutoModelStrategy(strategy string) string {
	if strategy == AutoModelStrategyOrder {
		return strategy
	}
	return AutoModelStrategyPrice
}

func FilterAutoModelNames(models, allowed []string) []string {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, modelName := range allowed {
		allowedSet[modelName] = struct{}{}
	}
	result := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == AutoModelName {
			continue
		}
		if _, ok := allowedSet[modelName]; !ok {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		result = append(result, modelName)
	}
	return result
}

func GetAutoModelConfig(userID int, userGroup string) (AutoModelConfig, error) {
	return GetAutoModelConfigForGroups(userID, userGroup, GetUserAutoGroup(userGroup))
}

func GetAutoModelConfigForGroups(userID int, userGroup string, groups []string) (AutoModelConfig, error) {
	return getAutoModelConfig(userID, groups, "")
}

func GetAutoModelConfigForGroup(userID int, group string) (AutoModelConfig, error) {
	return getAutoModelConfig(userID, []string{group}, group)
}

func getAutoModelConfig(userID int, groups []string, fixedGroup string) (AutoModelConfig, error) {
	userSetting, err := model.GetUserSetting(userID, false)
	if err != nil {
		return AutoModelConfig{}, fmt.Errorf("读取用户 Auto 模型设置: %w", err)
	}
	allowed := GetGroupsEnabledModels(groups)
	if !operation_setting.SelfUseModeEnabled && !userSetting.AcceptUnsetRatioModel {
		filtered := allowed[:0]
		for _, modelName := range allowed {
			if autoModelHasBillingConfig(modelName) {
				filtered = append(filtered, modelName)
			}
		}
		allowed = filtered
	}
	defaults := FilterAutoModelNames(setting.GetAutoModelDefaultModels(), allowed)
	if len(defaults) == 0 {
		defaults = FilterAutoModelNames(allowed, allowed)
	}
	configuredModels := userSetting.AutoModelNames
	configuredStrategy := userSetting.AutoModelStrategy
	configuredCustom := userSetting.AutoModelCustom
	if fixedGroup != "" {
		groupSetting, ok := userSetting.AutoModelGroups[fixedGroup]
		if ok {
			configuredModels = groupSetting.Models
			configuredStrategy = groupSetting.Strategy
			configuredCustom = groupSetting.Custom
		} else {
			configuredModels = nil
			configuredStrategy = ""
			configuredCustom = false
		}
	}
	config := AutoModelConfig{
		Models:        defaults,
		Strategy:      NormalizeAutoModelStrategy(configuredStrategy),
		Custom:        configuredCustom,
		AllowedModels: allowed,
		DefaultModels: defaults,
	}
	if configuredCustom {
		config.Models = FilterAutoModelNames(configuredModels, allowed)
	}
	return config, nil
}

func autoModelHasBillingConfig(modelName string) bool {
	if _, ok := ratio_setting.GetModelPrice(modelName, false); ok {
		return true
	}
	if _, ok, _ := ratio_setting.GetModelRatio(modelName); ok {
		return true
	}
	if billing_setting.GetBillingMode(modelName) != billing_setting.BillingModeTieredExpr {
		return false
	}
	expr, ok := billing_setting.GetBillingExpr(modelName)
	return ok && strings.TrimSpace(expr) != ""
}
