package controller

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyAutoModelCandidateUpdatesRequestAndRelayState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"first","messages":[]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	_, err := common.GetBodyStorage(c)
	require.NoError(t, err)

	request := &dto.GeneralOpenAIRequest{Model: "first"}
	info := &relaycommon.RelayInfo{Request: request, OriginModelName: "first"}
	retryParam := &service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "first", Retry: common.GetPointer(3)}

	require.NoError(t, applyAutoModelCandidate(c, info, retryParam, "second"))
	assert.Equal(t, "second", request.Model)
	assert.Equal(t, "second", info.OriginModelName)
	assert.Equal(t, "second", retryParam.ModelName)
	assert.Equal(t, "second", common.GetContextKeyString(c, constant.ContextKeyOriginalModel))
	assert.Equal(t, 3, common.GetContextKeyInt(c, constant.ContextKeyAutoGroupRetryIndex))

	body, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"second","messages":[]}`, string(body))
}

func TestPrepareSelectedChannelBillingClearsPreviousModelTieredSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"flat-model","messages":[]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyAutoGroup, "default")

	originalModelRatios := ratio_setting.ModelRatio2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalFreePreConsume := operation_setting.GetQuotaSetting().EnableFreeModelPreConsume
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"flat-model":0}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = false
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalModelRatios))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = originalFreePreConsume
	})

	info := &relaycommon.RelayInfo{
		Request:         &dto.GeneralOpenAIRequest{Model: "flat-model"},
		OriginModelName: "flat-model",
		UsingGroup:      "default",
		RetryIndex:      1,
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode: "tiered_expr",
			ModelName:   "old-tiered-model",
		},
	}

	require.Nil(t, prepareSelectedChannelBilling(c, info, 10, &relaytypes.TokenCountMeta{}))
	assert.Nil(t, info.TieredBillingSnapshot)
	assert.True(t, info.PriceData.FreeModel)
	assert.Equal(t, float64(0), info.PriceData.ModelRatio)
}

func TestGetChannelSkipsUnavailableAutoModelCandidate(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	common.MemoryCacheEnabled = false
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
	})

	channel := &model.Channel{
		Id:     2301,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "sk-auto-model-test",
		Status: common.ChannelStatusEnabled,
		Name:   "auto-model-test-channel",
		Group:  "default",
		Models: "available-model",
	}
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "available-model",
		ChannelId: channel.Id,
		Enabled:   true,
	}).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"missing-model","messages":[]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	_, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyAutoModelCandidates, []string{"missing-model", "available-model"})
	common.SetContextKey(c, constant.ContextKeyAutoModelIndex, 0)

	request := &dto.GeneralOpenAIRequest{Model: "missing-model"}
	info := &relaycommon.RelayInfo{
		Request:         request,
		OriginModelName: "missing-model",
		UserGroup:       "default",
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}
	retryParam := &service.RetryParam{
		Ctx:         c,
		TokenGroup:  "auto",
		ModelName:   "missing-model",
		RequestPath: c.Request.URL.Path,
		Retry:       common.GetPointer(0),
	}

	selected, selectErr := getChannel(c, info, retryParam)
	require.Nil(t, selectErr)
	require.NotNil(t, selected)
	assert.Equal(t, channel.Id, selected.Id)
	assert.Equal(t, "available-model", info.OriginModelName)
	assert.Equal(t, "available-model", request.Model)
}

func TestGetAutoModelRetryGroupKeepsFixedGroup(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyRequestedModel, service.AutoModelName)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyAutoRoutingGroup, "default")

	assert.Equal(t, "default", getAutoModelRetryGroup(c, "auto"))

	common.SetContextKey(c, constant.ContextKeyAutoRoutingGroup, "auto")
	assert.Equal(t, "auto", getAutoModelRetryGroup(c, "auto"))
}
