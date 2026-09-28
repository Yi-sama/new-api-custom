package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func imageStudioRelayToken(t *testing.T, role int) (string, int) {
	t.Helper()
	initModelListColumnNames(t)
	setImageStudioTestEnv(t)
	identity, session := setupImageStudioSession(t, role)
	token, _, err := service.IssueImageStudioToken(identity, "user", time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)
	return token, identity.UserID
}

func configureImageStudioRelayGroups(t *testing.T) {
	t.Helper()
	originalGroups := setting.UserUsableGroups2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	originalModelRatios := ratio_setting.ModelRatio2JSONString()
	originalSelfUse := operation_setting.SelfUseModeEnabled
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP","\u9ad8\u7ea7":"Premium"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1,"\u9ad8\u7ea7":1}`))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"bridge-default-model":1,"bridge-vip-model":1}`))
	operation_setting.SelfUseModeEnabled = true
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalModelRatios))
		operation_setting.SelfUseModeEnabled = originalSelfUse
	})
}

func imageStudioBridgeTestRouter() *gin.Engine {
	router := gin.New()
	router.Use(ImageStudioBridgeAuth())
	router.GET("/context", middleware.DisableCache(), GetImageStudioContext)
	router.GET("/groups", middleware.DisableCache(), GetUserGroups)
	router.GET("/v1/models", middleware.DisableCache(), ImageStudioGroupAuthorization(), GetImageStudioModels)
	return router
}

func imageStudioBridgeRequest(method, path, token string) *http.Request {
	request := httptest.NewRequest(method, "https://studio.example.com"+path, nil)
	request.AddCookie(&http.Cookie{Name: imageStudioCookieName, Value: token})
	return request
}

func TestImageStudioBridgeContextAndModelsUseSessionGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, userID := imageStudioRelayToken(t, common.RoleCommonUser)
	configureImageStudioRelayGroups(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Ability{}, &model.Channel{}))
	require.NoError(t, model.DB.Create(&[]model.Ability{
		{Group: "default", Model: "bridge-default-model", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "bridge-vip-model", ChannelId: 2, Enabled: true},
	}).Error)

	router := imageStudioBridgeTestRouter()
	contextResponse := httptest.NewRecorder()
	router.ServeHTTP(contextResponse, imageStudioBridgeRequest(http.MethodGet, "/context", token))
	require.Equal(t, http.StatusOK, contextResponse.Code)
	assert.Contains(t, contextResponse.Header().Get("Cache-Control"), "no-store")
	var context struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Group    string `json:"group"`
		Role     string `json:"role"`
	}
	require.NoError(t, common.Unmarshal(contextResponse.Body.Bytes(), &context))
	assert.Equal(t, userID, context.ID)
	assert.Equal(t, "Studio \u4e2d\u6587 User", context.Username)
	assert.Equal(t, "default", context.Group)
	assert.Equal(t, "user", context.Role)
	assert.Equal(t, "owui-"+fmt.Sprint(userID)+"@identity.example.com", context.Email)

	defaultResponse := httptest.NewRecorder()
	router.ServeHTTP(defaultResponse, imageStudioBridgeRequest(http.MethodGet, "/v1/models", token))
	require.Equal(t, http.StatusOK, defaultResponse.Code)
	assert.Contains(t, defaultResponse.Header().Get("Cache-Control"), "no-store")
	assert.Contains(t, string(defaultResponse.Body.Bytes()), "bridge-default-model")
	assert.NotContains(t, string(defaultResponse.Body.Bytes()), "bridge-vip-model")

	vipRequest := imageStudioBridgeRequest(http.MethodGet, "/v1/models", token)
	vipRequest.Header.Set("X-NewAPI-Group", "vip")
	vipResponse := httptest.NewRecorder()
	router.ServeHTTP(vipResponse, vipRequest)
	require.Equal(t, http.StatusOK, vipResponse.Code)
	assert.Contains(t, string(vipResponse.Body.Bytes()), "bridge-vip-model")
	assert.NotContains(t, string(vipResponse.Body.Bytes()), "bridge-default-model")

	groupsResponse := httptest.NewRecorder()
	router.ServeHTTP(groupsResponse, imageStudioBridgeRequest(http.MethodGet, "/groups", token))
	require.Equal(t, http.StatusOK, groupsResponse.Code)
	assert.Contains(t, groupsResponse.Header().Get("Cache-Control"), "no-store")
}

func TestImageStudioBridgeDecodesPathEscapedGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, _ := imageStudioRelayToken(t, common.RoleCommonUser)
	configureImageStudioRelayGroups(t)
	router := gin.New()
	router.Use(ImageStudioBridgeAuth())
	router.GET("/selected-group", ImageStudioGroupAuthorization(), func(c *gin.Context) {
		c.String(http.StatusOK, common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
	})

	request := imageStudioBridgeRequest(http.MethodGet, "/selected-group", token)
	request.Header.Set("X-NewAPI-Group", "%E9%AB%98%E7%BA%A7")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "\u9ad8\u7ea7", response.Body.String())
}

func TestImageStudioBridgeRejectsAutoWhenAutoIsNotUserVisible(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, _ := imageStudioRelayToken(t, common.RoleCommonUser)
	configureImageStudioRelayGroups(t)
	router := imageStudioBridgeTestRouter()

	request := imageStudioBridgeRequest(http.MethodGet, "/v1/models", token)
	request.Header.Set("X-NewAPI-Group", "auto")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestPrepareImageStudioRelayPreservesV1ModeForResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "https://studio.example.com/v1/responses", nil)

	prepareImageStudioRelay(c)

	assert.Equal(t, relayconstant.RelayModeResponses, c.GetInt("relay_mode"))
	assert.Equal(t, "/pg/responses", c.Request.URL.Path)
	assert.Empty(t, c.Request.URL.Scheme)
	assert.Empty(t, c.Request.URL.Host)
}

func TestImageStudioBridgeRejectsMismatchedIdentityAndRevokedSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, userID := imageStudioRelayToken(t, common.RoleCommonUser)
	configureImageStudioRelayGroups(t)
	router := imageStudioBridgeTestRouter()

	mismatched := imageStudioBridgeRequest(http.MethodGet, "/context", token)
	mismatched.Header.Set("X-NewAPI-Identity", "owui-999@identity.example.com")
	mismatchResponse := httptest.NewRecorder()
	router.ServeHTTP(mismatchResponse, mismatched)
	assert.Equal(t, http.StatusForbidden, mismatchResponse.Code)

	require.NoError(t, model.DB.Model(&model.UserSession{}).Where("user_id = ?", userID).Update("status", model.UserSessionStatusRevoked).Error)
	revokedResponse := httptest.NewRecorder()
	router.ServeHTTP(revokedResponse, imageStudioBridgeRequest(http.MethodGet, "/context", token))
	assert.Equal(t, http.StatusUnauthorized, revokedResponse.Code)
}

func TestImageStudioRelayChargesSessionUserAndIgnoresBodyGroupAndSharedKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, userID := imageStudioRelayToken(t, common.RoleCommonUser)
	configureImageStudioRelayGroups(t)
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCache })
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.UserSubscription{}, &model.ChannelStatusCheck{}))
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", userID).Update("quota", 100_000).Error)
	otherUser := &model.User{
		Username: "other-image-studio-user",
		Password: "unused-password-hash",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    77_777,
		AffCode:  "other-image-studio-aff-code",
	}
	require.NoError(t, model.DB.Create(otherUser).Error)

	var upstreamAuthorization string
	var upstreamPath string
	var upstreamCookie string
	var upstreamStudioName string
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamAuthorization = request.Header.Get("Authorization")
		upstreamPath = request.URL.Path
		upstreamCookie = request.Header.Get("Cookie")
		upstreamStudioName = request.Header.Get("X-Studio-Name")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chatcmpl-bridge","object":"chat.completion","created":1,"model":"bridge-vip-model","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`))
	}))
	t.Cleanup(upstream.Close)
	channel := &model.Channel{
		Type:    constant.ChannelTypeOpenAI,
		Key:     "upstream-bridge-key",
		Status:  common.ChannelStatusEnabled,
		Name:    "image-studio-bridge",
		Group:   "vip",
		Models:  "bridge-vip-model",
		BaseURL: &upstream.URL,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "vip", Model: "bridge-vip-model", ChannelId: channel.Id, Enabled: true}).Error)

	router := gin.New()
	router.Use(ImageStudioBridgeAuth())
	router.POST("/api/image-studio/v1/chat/completions", ImageStudioGroupAuthorization(), ImageStudioWriteOriginGuard(), ImageStudioNormalizeRelayPath(), middleware.Distribute(), func(c *gin.Context) {
		ImageStudioRelay(c, types.RelayFormatOpenAI)
	})
	request := httptest.NewRequest(http.MethodPost, "https://studio.example.com/api/image-studio/v1/chat/completions", strings.NewReader(`{"model":"bridge-vip-model","group":"default","messages":[{"role":"user","content":"hello"}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://studio.example.com")
	request.Header.Set("Authorization", "Bearer shared-key-must-not-be-used")
	request.Header.Set("X-NewAPI-Group", "vip")
	request.Header.Set("X-Studio-Name", "session-metadata-must-not-leak")
	request.AddCookie(&http.Cookie{Name: imageStudioCookieName, Value: token})
	request.AddCookie(&http.Cookie{Name: "dashboard_session", Value: "must-not-leak"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, "/v1/chat/completions", upstreamPath)
	assert.Equal(t, "Bearer upstream-bridge-key", upstreamAuthorization)
	assert.Empty(t, upstreamCookie)
	assert.Empty(t, upstreamStudioName)
	quota, err := model.GetUserQuota(userID, false)
	require.NoError(t, err)
	assert.Equal(t, 99_995, quota)
	otherQuota, err := model.GetUserQuota(otherUser.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 77_777, otherQuota)
	assert.NotContains(t, upstreamAuthorization, "shared-key-must-not-be-used")
}

func TestImageStudioWriteOriginGuardAllowsNoOriginAndRejectsOtherOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, _ := imageStudioRelayToken(t, common.RoleCommonUser)
	configureImageStudioRelayGroups(t)
	router := gin.New()
	router.Use(ImageStudioBridgeAuth())
	router.POST("/write", ImageStudioWriteOriginGuard(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	noOrigin := imageStudioBridgeRequest(http.MethodPost, "/write", token)
	noOriginResponse := httptest.NewRecorder()
	router.ServeHTTP(noOriginResponse, noOrigin)
	assert.Equal(t, http.StatusNoContent, noOriginResponse.Code)

	foreignOrigin := imageStudioBridgeRequest(http.MethodPost, "/write", token)
	foreignOrigin.Header.Set("Origin", "https://attacker.example.com")
	foreignResponse := httptest.NewRecorder()
	router.ServeHTTP(foreignResponse, foreignOrigin)
	assert.Equal(t, http.StatusForbidden, foreignResponse.Code)
}
