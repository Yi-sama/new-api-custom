package controller

import (
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func filterPricingByUsableGroups(pricing []model.Pricing, usableGroup map[string]string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(usableGroup) == 0 {
		return []model.Pricing{}
	}

	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		if common.StringsContains(item.EnableGroup, "all") {
			filtered = append(filtered, item)
			continue
		}
		for _, group := range item.EnableGroup {
			if _, ok := usableGroup[group]; ok {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

func getAutoEnableGroups(pricing []model.Pricing, selectableGroups []string) []string {
	available := make(map[string]struct{}, len(selectableGroups))
	selectable := make(map[string]struct{}, len(selectableGroups))
	for _, group := range selectableGroups {
		selectable[group] = struct{}{}
	}
	for _, item := range pricing {
		if item.ModelName == service.AutoModelName {
			continue
		}
		if common.StringsContains(item.EnableGroup, "all") {
			for group := range selectable {
				available[group] = struct{}{}
			}
			continue
		}
		for _, group := range item.EnableGroup {
			if _, ok := selectable[group]; ok {
				available[group] = struct{}{}
			}
		}
	}
	groups := make([]string, 0, len(available))
	for group := range available {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}

func GetPricing(c *gin.Context) {
	pricing := model.GetPricing()
	userId, exists := c.Get("id")
	usableGroup := map[string]string{}
	groupRatio := map[string]float64{}
	for s, f := range ratio_setting.GetGroupRatioCopy() {
		groupRatio[s] = f
	}
	var group string
	if exists && len(pricing) > 0 {
		user, err := model.GetUserCache(userId.(int))
		if err == nil {
			group = user.Group
			for g := range groupRatio {
				ratio, ok := ratio_setting.GetGroupGroupRatio(group, g)
				if ok {
					groupRatio[g] = ratio
				}
			}
		}
	}

	usableGroup = service.GetUserUsableGroups(group)
	ranges, err := service.GetChannelRatioRanges(group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for name, r := range ranges {
		if _, ok := usableGroup[name]; !ok {
			delete(ranges, name)
		} else {
			groupRatio[name] = r.Min
		}
	}
	pricing = filterPricingByUsableGroups(pricing, usableGroup)
	selectableGroups := make([]string, 0, len(usableGroup))
	for groupName := range usableGroup {
		if service.IsUserSelectableGroup(group, groupName) {
			selectableGroups = append(selectableGroups, groupName)
		}
	}
	autoEnableGroups := getAutoEnableGroups(pricing, selectableGroups)
	if exists && len(autoEnableGroups) > 0 {
		pricing = append([]model.Pricing{{
			ModelName:              service.AutoModelName,
			Description:            "可以自由选择 Auto 使用的模型，并设置模型选择策略。",
			Icon:                   "Sparkles",
			Tags:                   "Auto",
			QuotaType:              0,
			EnableGroup:            autoEnableGroups,
			SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse},
		}}, pricing...)
	}
	// check groupRatio contains usableGroup
	for group := range ratio_setting.GetGroupRatioCopy() {
		if _, ok := usableGroup[group]; !ok {
			delete(groupRatio, group)
		}
	}

	c.JSON(200, gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"group_ratio_range":  ranges,
		"usable_group":       usableGroup,
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        service.GetUserAutoGroup(group),
		"pricing_version":    "a42d372ccf0b5dd13ecf71203521f9d2",
	})
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}
