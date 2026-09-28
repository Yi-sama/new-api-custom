package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	appI18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAutoModelResolverTest(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalIsMasterNode := common.IsMasterNode
	originalSQLitePath := common.SQLitePath
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRedisEnabled := common.RedisEnabled
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalModelRatios := ratio_setting.ModelRatio2JSONString()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.IsMasterNode = false
	common.SQLitePath = dsn
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	db := model.DB
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}))
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["default"]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"auto-expensive":5,"auto-cheap":1,"global-first":1,"global-second":1}`))

	t.Cleanup(func() {
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalModelRatios))
		sqlDB, err := db.DB()
		if err == nil {
			require.NoError(t, sqlDB.Close())
		}
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.SetDatabaseTypes(originalDatabaseType, originalLogDatabaseType)
		common.IsMasterNode = originalIsMasterNode
		common.SQLitePath = originalSQLitePath
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RedisEnabled = originalRedisEnabled
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	})

	return db
}

func createAutoModelResolverChannel(t *testing.T, db *gorm.DB, id int, group, modelName string) {
	t.Helper()
	priority := int64(0)
	weight := uint(100)
	require.NoError(t, db.Create(&model.Channel{
		Id:       id,
		Type:     constant.ChannelTypeOpenAI,
		Key:      fmt.Sprintf("key-%d", id),
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("channel-%d", id),
		Group:    group,
		Models:   modelName,
		Priority: &priority,
		Weight:   &weight,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: id,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)
}

func TestRewriteJSONRequestModelReplacesCachedAndRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"Auto","messages":[]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	_, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	require.NoError(t, RewriteJSONRequestModel(c, "gpt-test"))

	body, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"gpt-test","messages":[]}`, string(body))

	storage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	cached, err := storage.Bytes()
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"gpt-test","messages":[]}`, string(cached))
}

func TestResolveAutoModelRequestAppliesConfiguredStrategy(t *testing.T) {
	db := setupAutoModelResolverTest(t)
	user := &model.User{
		Username: "auto-strategy-user",
		Password: "password-placeholder",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "auto-strategy-aff",
	}
	require.NoError(t, db.Create(user).Error)
	createAutoModelResolverChannel(t, db, 2401, "default", "auto-expensive")
	createAutoModelResolverChannel(t, db, 2402, "default", "auto-cheap")

	for _, test := range []struct {
		name       string
		strategy   string
		expected   string
		candidates []string
	}{
		{
			name:       "price sorts by estimated cost",
			strategy:   service.AutoModelStrategyPrice,
			expected:   "auto-cheap",
			candidates: []string{"auto-cheap", "auto-expensive"},
		},
		{
			name:       "order preserves user order",
			strategy:   service.AutoModelStrategyOrder,
			expected:   "auto-expensive",
			candidates: []string{"auto-expensive", "auto-cheap"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, model.UpdateUserAutoModelGroupSetting(
				user.Id,
				"default",
				[]string{"auto-expensive", "auto-cheap"},
				test.strategy,
				true,
			))

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"Auto","messages":[],"max_tokens":10}`))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set("id", user.Id)
			common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
			common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
			common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{
				service.AutoModelName: true,
			})

			request := &ModelRequest{Model: service.AutoModelName}
			channel, err := resolveAutoModelRequest(c, request, true)
			require.NoError(t, err)
			require.NotNil(t, channel)
			assert.Equal(t, test.expected, request.Model)

			value, ok := common.GetContextKey(c, constant.ContextKeyAutoModelCandidates)
			require.True(t, ok)
			assert.Equal(t, test.candidates, value)
		})
	}
}

func TestResolveAutoModelRequestScopesCandidatesToFixedTokenGroup(t *testing.T) {
	db := setupAutoModelResolverTest(t)
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["vip"]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))

	user := &model.User{
		Username: "auto-fixed-group-user",
		Password: "password-placeholder",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "auto-fixed-group-aff",
	}
	require.NoError(t, db.Create(user).Error)
	createAutoModelResolverChannel(t, db, 2501, "default", "auto-cheap")
	createAutoModelResolverChannel(t, db, 2502, "vip", "auto-vip")
	require.NoError(t, model.UpdateUserAutoModelSetting(
		user.Id,
		[]string{"auto-vip"},
		service.AutoModelStrategyOrder,
		true,
	))
	require.NoError(t, model.UpdateUserAutoModelGroupSetting(
		user.Id,
		"default",
		[]string{"auto-cheap"},
		service.AutoModelStrategyOrder,
		true,
	))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"Auto","messages":[]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", user.Id)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")

	request := &ModelRequest{Model: service.AutoModelName}
	channel, err := resolveAutoModelRequest(c, request, true)

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, "default", channel.Group)
	assert.Equal(t, "auto-cheap", request.Model)
	assert.Equal(t, "default", common.GetContextKeyString(c, constant.ContextKeyAutoRoutingGroup))
	value, ok := common.GetContextKey(c, constant.ContextKeyAutoModelCandidates)
	require.True(t, ok)
	assert.Equal(t, []string{"auto-cheap"}, value)
}

func TestResolveAutoModelRequestKeepsGlobalConfigWhenAutoHasOneGroup(t *testing.T) {
	db := setupAutoModelResolverTest(t)
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["default"]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))

	user := &model.User{
		Username: "auto-single-global-user",
		Password: "password-placeholder",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "auto-single-global-aff",
	}
	require.NoError(t, db.Create(user).Error)
	createAutoModelResolverChannel(t, db, 2551, "default", "global-first")
	createAutoModelResolverChannel(t, db, 2552, "default", "global-second")
	require.NoError(t, model.UpdateUserAutoModelSetting(
		user.Id,
		[]string{"global-second", "global-first"},
		service.AutoModelStrategyOrder,
		true,
	))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"Auto","messages":[]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", user.Id)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "auto")

	request := &ModelRequest{Model: service.AutoModelName}
	channel, err := resolveAutoModelRequest(c, request, true)

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, "global-second", request.Model)
	value, ok := common.GetContextKey(c, constant.ContextKeyAutoModelCandidates)
	require.True(t, ok)
	assert.Equal(t, []string{"global-second", "global-first"}, value)
}

func TestDistributeAuthorizesAutoAsIndependentModelLimit(t *testing.T) {
	require.NoError(t, appI18n.Init())
	db := setupAutoModelResolverTest(t)
	user := &model.User{
		Username: "auto-model-limit-user",
		Password: "password-placeholder",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "auto-model-limit-aff",
	}
	require.NoError(t, db.Create(user).Error)
	createAutoModelResolverChannel(t, db, 2601, "default", "auto-cheap")
	require.NoError(t, model.UpdateUserAutoModelSetting(
		user.Id,
		[]string{"auto-cheap"},
		service.AutoModelStrategyOrder,
		true,
	))

	request := func(limits map[string]bool) (*gin.Context, *httptest.ResponseRecorder) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"Auto","messages":[]}`))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("id", user.Id)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
		common.SetContextKey(c, constant.ContextKeyTokenModelLimit, limits)
		Distribute()(c)
		return c, recorder
	}

	allowed, allowedRecorder := request(map[string]bool{service.AutoModelName: true})
	assert.False(t, allowed.IsAborted())
	assert.Equal(t, http.StatusOK, allowedRecorder.Code)
	assert.Equal(t, service.AutoModelName, common.GetContextKeyString(allowed, constant.ContextKeyRequestedModel))
	assert.Equal(t, 2601, common.GetContextKeyInt(allowed, constant.ContextKeyChannelId))

	denied, deniedRecorder := request(map[string]bool{"auto-cheap": true})
	assert.True(t, denied.IsAborted())
	assert.Equal(t, http.StatusForbidden, deniedRecorder.Code)
}
