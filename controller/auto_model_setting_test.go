package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateUserAutoModelSettingPreservesOtherPreferences(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	user := &model.User{
		Id:       2201,
		Username: "auto-model-setting-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}
	user.SetSetting(dto.UserSetting{
		NotifyType:        dto.NotifyTypeEmail,
		NotificationEmail: "user@example.com",
		Language:          "zh",
	})
	require.NoError(t, db.Create(user).Error)

	require.NoError(t, model.UpdateUserAutoModelSetting(user.Id, []string{"model-b", "model-a"}, "order", true))

	setting, err := model.GetUserSetting(user.Id, true)
	require.NoError(t, err)
	assert.Equal(t, dto.NotifyTypeEmail, setting.NotifyType)
	assert.Equal(t, "user@example.com", setting.NotificationEmail)
	assert.Equal(t, "zh", setting.Language)
	assert.Equal(t, []string{"model-b", "model-a"}, setting.AutoModelNames)
	assert.Equal(t, "order", setting.AutoModelStrategy)
	assert.True(t, setting.AutoModelCustom)
}

func TestGetAutoModelSettingReturnsEmptyArraysInsteadOfNull(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	user := &model.User{
		Id:       2202,
		Username: "auto-model-empty-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", user.Id)
	GetAutoModelSetting(c)

	var response struct {
		Models        []string `json:"models"`
		AllowedModels []string `json:"allowed_models"`
		DefaultModels []string `json:"default_models"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.NotNil(t, response.Models)
	assert.NotNil(t, response.AllowedModels)
	assert.NotNil(t, response.DefaultModels)
}

func TestUpdateUserAutoModelGroupSettingPreservesOtherAutoScopes(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	user := &model.User{
		Id:       2203,
		Username: "auto-model-group-setting-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}
	user.SetSetting(dto.UserSetting{
		AutoModelNames:    []string{"global-model"},
		AutoModelStrategy: "order",
		AutoModelCustom:   true,
		AutoModelGroups: map[string]dto.AutoModelGroupSetting{
			"vip": {Models: []string{"vip-model"}, Strategy: "price", Custom: true},
		},
	})
	require.NoError(t, db.Create(user).Error)

	require.NoError(t, model.UpdateUserAutoModelGroupSetting(
		user.Id,
		"default",
		[]string{"default-model"},
		"order",
		true,
	))

	setting, err := model.GetUserSetting(user.Id, true)
	require.NoError(t, err)
	assert.Equal(t, []string{"global-model"}, setting.AutoModelNames)
	assert.Equal(t, []string{"vip-model"}, setting.AutoModelGroups["vip"].Models)
	assert.Equal(t, []string{"default-model"}, setting.AutoModelGroups["default"].Models)
}
