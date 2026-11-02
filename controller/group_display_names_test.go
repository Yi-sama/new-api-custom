package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserGroupsDisplayNamesPreserveGroupAccessAndPricing(t *testing.T) {
	initModelListColumnNames(t)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousOptions := common.OptionMap
	previousGroups := setting.UserUsableGroups2JSONString()
	previousRatios := ratio_setting.GroupRatio2JSONString()
	previousNames := setting.GroupDisplayNames2JSONString()
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Option{}))
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateGroupDisplayNamesByJSONString(previousNames))
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.OptionMap = previousOptions
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousRatios))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(
		`{"default":"Testing","vip":"Shared quota","blank":"Description","auto":"Automatic"}`,
	))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(
		`{"default":0.2,"vip":0,"blank":0.9,"hidden":1}`,
	))
	user := model.User{Id: 101, Username: "display-name-user", Group: "default"}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&[]model.Channel{
		{Key: "test-1", Group: "default", Status: common.ChannelStatusEnabled, ChannelRatio: common.GetPointer(0.2)},
		{Key: "test-2", Group: "default", Status: common.ChannelStatusEnabled, ChannelRatio: common.GetPointer(0.4)},
	}).Error)

	readGroups := func() map[string]map[string]any {
		t.Helper()
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Set("id", user.Id)
		GetUserGroups(context)
		require.Equal(t, http.StatusOK, recorder.Code)
		var payload struct {
			Success bool                      `json:"success"`
			Data    map[string]map[string]any `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
		require.True(t, payload.Success, recorder.Body.String())
		return payload.Data
	}

	require.NoError(t, model.UpdateOption("GroupDisplayNames", `{}`))
	original := readGroups()
	assert.Equal(t, map[string]any{
		"ratio": 0.2, "ratio_min": 0.2, "ratio_max": 0.4, "desc": "Testing",
	}, original["default"])
	assert.Equal(t, map[string]any{"ratio": float64(0), "desc": "Shared quota"}, original["vip"])
	assert.Equal(t, map[string]any{"ratio": "\u81ea\u52a8", "desc": "Automatic"}, original["auto"])

	require.NoError(t, model.UpdateOption("GroupDisplayNames",
		`[{"group":"hidden","name":"Hidden"},{"group":"default","name":" GPT "},{"group":"unknown","name":"Unknown"},{"group":"auto","name":"All models"}]`))
	groups := readGroups()
	assert.Equal(t, "GPT", groups["default"]["display_name"])
	assert.Equal(t, float64(2), groups["default"]["display_order"])
	assert.Equal(t, "All models", groups["auto"]["display_name"])
	assert.Equal(t, float64(4), groups["auto"]["display_order"])
	assert.NotContains(t, groups["vip"], "display_name")
	assert.NotContains(t, groups["vip"], "display_order")
	assert.NotContains(t, groups["blank"], "display_name")
	assert.NotContains(t, groups, "hidden")
	assert.NotContains(t, groups, "unknown")
	for _, info := range groups {
		delete(info, "display_name")
		delete(info, "display_order")
	}
	assert.Equal(t, original, groups)

	require.NoError(t, model.UpdateOption("GroupDisplayNames", `{"auto":"All","default":"GPT"}`))
	assert.Equal(t, float64(1), readGroups()["auto"]["display_order"])
	assert.Equal(t, float64(2), readGroups()["default"]["display_order"])

	require.NoError(t, model.UpdateOption("GroupDisplayNames", `[]`))
	assert.Equal(t, original, readGroups(), "clearing display names takes effect without a restart")
}
