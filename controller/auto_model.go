package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

type autoModelUpdateRequest struct {
	Models         []string `json:"models"`
	Strategy       string   `json:"strategy"`
	InheritDefault bool     `json:"inherit_default"`
	Group          string   `json:"group"`
}

func getAutoModelRequestGroup(c *gin.Context, userGroup string) (string, bool) {
	group := strings.TrimSpace(c.Query("group"))
	if group == "" || group == "auto" {
		return "", true
	}
	if !service.IsUserSelectableGroup(userGroup, group) {
		common.ApiErrorMsg(c, "当前用户不可使用该分组")
		return "", false
	}
	return group, true
}

func GetAutoModelSetting(c *gin.Context) {
	userID := c.GetInt("id")
	userGroup, err := model.GetUserGroup(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	group, ok := getAutoModelRequestGroup(c, userGroup)
	if !ok {
		return
	}
	var config service.AutoModelConfig
	if group == "" {
		config, err = service.GetAutoModelConfig(userID, userGroup)
	} else {
		config, err = service.GetAutoModelConfigForGroup(userID, group)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"models":         config.Models,
		"strategy":       config.Strategy,
		"custom":         config.Custom,
		"allowed_models": config.AllowedModels,
		"default_models": config.DefaultModels,
	})
}

func UpdateAutoModelSetting(c *gin.Context) {
	var req autoModelUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的 Auto 模型设置")
		return
	}
	userID := c.GetInt("id")
	userGroup, err := model.GetUserGroup(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	group := strings.TrimSpace(req.Group)
	if group != "" && group != "auto" {
		if !service.IsUserSelectableGroup(userGroup, group) {
			common.ApiErrorMsg(c, "当前用户不可使用该分组")
			return
		}
	} else {
		group = ""
	}
	var config service.AutoModelConfig
	if group == "" {
		config, err = service.GetAutoModelConfig(userID, userGroup)
	} else {
		config, err = service.GetAutoModelConfigForGroup(userID, group)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	selected := service.FilterAutoModelNames(req.Models, config.AllowedModels)
	if len(selected) != len(req.Models) && !req.InheritDefault {
		common.ApiErrorMsg(c, "Auto 模型列表包含不可用或重复的模型")
		return
	}
	if !req.InheritDefault && len(selected) == 0 {
		common.ApiErrorMsg(c, "请至少选择一个 Auto 模型")
		return
	}
	models := selected
	if req.InheritDefault {
		models = nil
	}
	strategy := service.NormalizeAutoModelStrategy(req.Strategy)
	var updateErr error
	if group == "" {
		updateErr = model.UpdateUserAutoModelSetting(userID, models, strategy, !req.InheritDefault)
	} else {
		updateErr = model.UpdateUserAutoModelGroupSetting(userID, group, models, strategy, !req.InheritDefault)
	}
	if updateErr != nil {
		common.ApiError(c, updateErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UpdateAutoModelDefault(c *gin.Context) {
	var req autoModelUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的 Auto 模型设置")
		return
	}
	selected := service.FilterAutoModelNames(req.Models, model.GetEnabledModels())
	if len(selected) != len(req.Models) {
		common.ApiErrorMsg(c, "站内默认 Auto 列表包含不可用或重复的模型")
		return
	}
	if len(selected) == 0 {
		common.ApiErrorMsg(c, "请至少选择一个站内默认 Auto 模型")
		return
	}
	value, err := common.Marshal(selected)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption(setting.AutoModelDefaultModelsOptionKey, string(value)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "auto_model.default.update", map[string]interface{}{"model_count": len(selected)})
	c.JSON(http.StatusOK, gin.H{"success": true})
}
