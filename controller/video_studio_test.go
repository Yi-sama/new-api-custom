package controller

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateVideoStudioSessionRequiresConfiguredDist(t *testing.T) {
	t.Setenv("VIDEO_STUDIO_DIST_DIR", "")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/video-studio/session", nil)
	CreateVideoStudioSession(c)
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestVideoStudioBridgeUsesSyntheticTokenAndRejectsOtherStudioTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("VIDEO_STUDIO_DIST_DIR", t.TempDir())
	identity, session := setupImageStudioSession(t, common.RoleCommonUser)
	videoToken, _, err := service.IssueVideoStudioToken(identity, time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)
	imageToken, _, err := service.IssueImageStudioToken(identity, "user", time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)

	router := gin.New()
	router.Use(VideoStudioBridgeAuth())
	router.GET("/context", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": c.GetInt("id"), "token_id": c.GetInt("token_id"), "token_key": c.GetString("token_key")})
	})
	request := httptest.NewRequest(http.MethodGet, "/context", nil)
	request.Header.Set("Authorization", "Bearer "+videoToken)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `"id":`)
	assert.Contains(t, response.Body.String(), `"token_id":0`)
	assert.Contains(t, response.Body.String(), `"token_key":""`)

	request = httptest.NewRequest(http.MethodGet, "/context", nil)
	request.Header.Set("Authorization", "Bearer "+imageToken)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestVideoStudioSyntheticTokenPreConsumesWallet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("VIDEO_STUDIO_DIST_DIR", t.TempDir())
	identity, session := setupImageStudioSession(t, common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", identity.UserID).Updates(map[string]any{
		"quota": 1000, "setting": `{"billing_preference":"wallet_only"}`,
	}).Error)
	token, _, err := service.IssueVideoStudioToken(identity, time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)

	router := gin.New()
	router.Use(VideoStudioBridgeAuth())
	router.POST("/v1/videos", func(c *gin.Context) {
		prepareVideoStudioRelay(c, 0)
		info, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
		require.NoError(t, err)
		info.ForcePreConsume = true
		require.Zero(t, info.TokenId)
		require.True(t, info.IsPlayground)
		require.Nil(t, service.PreConsumeBilling(c, 200, info))
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	quota, err := model.GetUserQuota(identity.UserID, true)
	require.NoError(t, err)
	assert.Equal(t, 800, quota)
}

func TestVideoStudioTaskFetchIsOwnerScoped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("VIDEO_STUDIO_DIST_DIR", t.TempDir())
	identity, session := setupImageStudioSession(t, common.RoleCommonUser)
	ownerToken, _, err := service.IssueVideoStudioToken(identity, time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)
	require.NoError(t, model.DB.AutoMigrate(&model.Task{}))
	require.NoError(t, model.DB.Create(&model.Task{TaskID: "task-owner-only", UserId: identity.UserID, Platform: "not-a-platform"}).Error)
	other := model.User{Username: "video-other-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "video-other-code", AuthVersion: 1}
	require.NoError(t, model.DB.Create(&other).Error)
	otherSession := &model.UserSession{SID: "video-other-session", UserID: other.Id, Version: 1, UserAuthVersion: 1, Status: model.UserSessionStatusActive, RefreshHash: "unused", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, model.CreateUserSession(otherSession))
	otherToken, _, err := service.IssueVideoStudioToken(service.AuthIdentity{UserID: other.Id, SessionID: otherSession.SID, UserAuthVersion: 1, SessionVersion: 1}, time.Unix(otherSession.ExpiresAt, 0))
	require.NoError(t, err)

	router := gin.New()
	router.Use(VideoStudioBridgeAuth())
	router.GET("/api/video-studio/v1/videos/:task_id", VideoStudioNormalizeRelayPath(), VideoStudioRelayTaskFetch)
	request := httptest.NewRequest(http.MethodGet, "/api/video-studio/v1/videos/task-owner-only", nil)
	request.Header.Set("Authorization", "Bearer "+otherToken)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "task_not_exist")

	request = httptest.NewRequest(http.MethodGet, "/api/video-studio/v1/videos/task-owner-only", nil)
	request.Header.Set("Authorization", "Bearer "+ownerToken)
	request.Header.Set(videoStudioGroupHeader, "removed-group")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.NotContains(t, response.Body.String(), "task_not_exist")
}

func configureVideoStudioRelayGroups(t *testing.T) {
	t.Helper()
	originalGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalModelRatios := ratio_setting.ModelRatio2JSONString()
	originalSelfUseMode := operation_setting.SelfUseModeEnabled
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"sora-2":1}`))
	operation_setting.SelfUseModeEnabled = true
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalModelRatios))
		operation_setting.SelfUseModeEnabled = originalSelfUseMode
	})
}

func videoStudioRelayTestRouter() *gin.Engine {
	router := gin.New()
	router.Use(VideoStudioBridgeAuth())
	router.GET("/api/video-studio/v1/models", VideoStudioGroupAuthorization(), GetVideoStudioModels)
	router.POST("/api/video-studio/v1/videos", VideoStudioGroupAuthorization(), VideoStudioNormalizeRelayPath(), middleware.Distribute(), VideoStudioVideoModelAuthorization(), VideoStudioRelayTask)
	router.GET("/api/video-studio/v1/videos/:task_id", VideoStudioNormalizeRelayPath(), VideoStudioRelayTaskFetch)
	router.GET("/api/video-studio/v1/videos/:task_id/content", VideoStudioVideoProxy)
	return router
}

func videoStudioMultipartVideoRequest(t *testing.T, token string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "sora-2"))
	require.NoError(t, writer.WriteField("prompt", "a relay integration test"))
	require.NoError(t, writer.WriteField("seconds", "4"))
	part, err := writer.CreateFormFile("input_reference", "frame.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("not-a-real-png"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(http.MethodPost, "/api/video-studio/v1/videos", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Specific-Channel-Id", "999")
	return request
}

func createVideoStudioRelayUser(t *testing.T, username string) (string, int) {
	t.Helper()
	user := &model.User{Username: username, Password: "unused-password-hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: username + "-aff", AuthVersion: 1}
	require.NoError(t, model.DB.Create(user).Error)
	session := &model.UserSession{SID: username + "-session", UserID: user.Id, Version: 1, UserAuthVersion: 1, Status: model.UserSessionStatusActive, RefreshHash: "unused", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, model.CreateUserSession(session))
	token, _, err := service.IssueVideoStudioToken(service.AuthIdentity{UserID: user.Id, SessionID: session.SID, UserAuthVersion: 1, SessionVersion: 1}, time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)
	return token, user.Id
}

func TestVideoStudioVideoRelayBillsWalletAndScopesTaskAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("VIDEO_STUDIO_DIST_DIR", t.TempDir())
	initModelListColumnNames(t)
	identity, session := setupImageStudioSession(t, common.RoleCommonUser)
	configureVideoStudioRelayGroups(t)
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCache })
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Task{}, &model.Model{}))
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", identity.UserID).Updates(map[string]any{
		"quota": 10_000_000, "setting": `{"billing_preference":"wallet_only"}`,
	}).Error)
	ownerToken, _, err := service.IssueVideoStudioToken(identity, time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)
	otherToken, _ := createVideoStudioRelayUser(t, "video-relay-other-user")

	var upstreamAuthorization, upstreamGroup, upstreamPath, upstreamFileName string
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamAuthorization = request.Header.Get("Authorization")
		upstreamGroup = request.Header.Get(videoStudioGroupHeader)
		upstreamPath = request.URL.Path
		require.NoError(t, request.ParseMultipartForm(1<<20))
		file, header, err := request.FormFile("input_reference")
		require.NoError(t, err)
		_ = file.Close()
		upstreamFileName = header.Filename
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"upstream-task-1","object":"video","model":"sora-2","status":"queued","progress":0,"created_at":123}`))
	}))
	t.Cleanup(upstream.Close)
	channel := &model.Channel{Type: constant.ChannelTypeSora, Key: "upstream-video-key", Status: common.ChannelStatusEnabled, Name: "video-studio-relay", Group: "default", Models: "sora-2", BaseURL: &upstream.URL}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "default", Model: "sora-2", ChannelId: channel.Id, Enabled: true}).Error)

	response := httptest.NewRecorder()
	videoStudioRelayTestRouter().ServeHTTP(response, videoStudioMultipartVideoRequest(t, ownerToken))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, "/v1/videos", upstreamPath)
	assert.Equal(t, "Bearer upstream-video-key", upstreamAuthorization)
	assert.Empty(t, upstreamGroup)
	assert.Equal(t, "frame.png", upstreamFileName)
	assert.NotContains(t, upstreamAuthorization, ownerToken)

	var submitResponse struct {
		ID     string `json:"id"`
		Object string `json:"object"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &submitResponse))
	require.True(t, strings.HasPrefix(submitResponse.ID, "task_"))
	assert.Equal(t, "video", submitResponse.Object)
	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ?", submitResponse.ID).First(&task).Error)
	assert.Equal(t, identity.UserID, task.UserId)
	assert.Equal(t, "upstream-task-1", task.PrivateData.UpstreamTaskID)
	assert.Zero(t, task.PrivateData.TokenId)
	require.Positive(t, task.Quota)
	quota, err := model.GetUserQuota(identity.UserID, false)
	require.NoError(t, err)
	assert.Equal(t, 10_000_000-task.Quota, quota)

	ownerPoll := httptest.NewRecorder()
	ownerRequest := httptest.NewRequest(http.MethodGet, "/api/video-studio/v1/videos/"+submitResponse.ID, nil)
	ownerRequest.Header.Set("Authorization", "Bearer "+ownerToken)
	videoStudioRelayTestRouter().ServeHTTP(ownerPoll, ownerRequest)
	require.Equal(t, http.StatusOK, ownerPoll.Code, ownerPoll.Body.String())
	assert.Contains(t, ownerPoll.Body.String(), `"id":"`+submitResponse.ID+`"`)
	assert.Contains(t, ownerPoll.Body.String(), `"object":"video"`)

	otherPoll := httptest.NewRecorder()
	otherRequest := httptest.NewRequest(http.MethodGet, "/api/video-studio/v1/videos/"+submitResponse.ID, nil)
	otherRequest.Header.Set("Authorization", "Bearer "+otherToken)
	videoStudioRelayTestRouter().ServeHTTP(otherPoll, otherRequest)
	assert.Equal(t, http.StatusBadRequest, otherPoll.Code)
	assert.Contains(t, otherPoll.Body.String(), "task_not_exist")

	otherContent := httptest.NewRecorder()
	otherContentRequest := httptest.NewRequest(http.MethodGet, "/api/video-studio/v1/videos/"+submitResponse.ID+"/content", nil)
	otherContentRequest.Header.Set("Authorization", "Bearer "+otherToken)
	videoStudioRelayTestRouter().ServeHTTP(otherContent, otherContentRequest)
	assert.Equal(t, http.StatusNotFound, otherContent.Code)
	assert.Contains(t, otherContent.Body.String(), "Task not found")
}

func TestVideoStudioVideoRelayRefundsWalletWhenUpstreamSubmissionFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("VIDEO_STUDIO_DIST_DIR", t.TempDir())
	initModelListColumnNames(t)
	identity, session := setupImageStudioSession(t, common.RoleCommonUser)
	configureVideoStudioRelayGroups(t)
	originalMemoryCache, originalRetries := common.MemoryCacheEnabled, common.RetryTimes
	common.MemoryCacheEnabled = false
	common.RetryTimes = 0
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCache
		common.RetryTimes = originalRetries
	})
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Task{}, &model.Model{}))
	const initialQuota = 10_000_000
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", identity.UserID).Updates(map[string]any{
		"quota": initialQuota, "setting": `{"billing_preference":"wallet_only"}`,
	}).Error)
	token, _, err := service.IssueVideoStudioToken(identity, time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "upstream submission failed", http.StatusInternalServerError)
	}))
	t.Cleanup(upstream.Close)
	channel := &model.Channel{Type: constant.ChannelTypeSora, Key: "upstream-video-key", Status: common.ChannelStatusEnabled, Name: "video-studio-refund", Group: "default", Models: "sora-2", BaseURL: &upstream.URL}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "default", Model: "sora-2", ChannelId: channel.Id, Enabled: true}).Error)

	response := httptest.NewRecorder()
	videoStudioRelayTestRouter().ServeHTTP(response, videoStudioMultipartVideoRequest(t, token))
	assert.Equal(t, http.StatusInternalServerError, response.Code)
	require.Eventually(t, func() bool {
		quota, quotaErr := model.GetUserQuota(identity.UserID, false)
		return quotaErr == nil && quota == initialQuota
	}, time.Second, 10*time.Millisecond)
	var taskCount int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&taskCount).Error)
	assert.Zero(t, taskCount)
}

func TestVideoStudioCatalogFiltersGroupsAndModelsByCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("VIDEO_STUDIO_DIST_DIR", t.TempDir())
	initModelListColumnNames(t)
	identity, session := setupImageStudioSession(t, common.RoleCommonUser)
	originalGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
	})
	unicodeGroup := "video-\u4e2d\u6587+\u7ec4"
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","video-\u4e2d\u6587+\u7ec4":"Unicode","llm-only":"LLM only"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"video-\u4e2d\u6587+\u7ec4":2,"llm-only":1,"denied":1}`))
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Model{}))

	channels := []*model.Channel{
		{Type: constant.ChannelTypeOpenAI, Key: "llm-key", Status: common.ChannelStatusEnabled, Name: "llm", Group: "default", Models: "gpt-4o"},
		{Type: constant.ChannelTypeSora, Key: "video-key", Status: common.ChannelStatusEnabled, Name: "video", Group: "default", Models: "sora-main,disabled-video"},
		{Type: constant.ChannelTypeOpenAI, Key: "custom-key", Status: common.ChannelStatusEnabled, Name: "custom", Group: unicodeGroup, Models: "custom-video"},
		{Type: constant.ChannelTypeSora, Key: "disabled-key", Status: common.ChannelStatusManuallyDisabled, Name: "disabled", Group: unicodeGroup, Models: "disabled-channel-video"},
		{Type: constant.ChannelTypeOpenAI, Key: "llm-only-key", Status: common.ChannelStatusEnabled, Name: "llm-only", Group: "llm-only", Models: "gpt-4o-mini"},
	}
	for _, channel := range channels {
		require.NoError(t, model.DB.Create(channel).Error)
	}
	require.NoError(t, model.DB.Create(&[]model.Ability{
		{Group: "default", Model: "gpt-4o", ChannelId: channels[0].Id, Enabled: true},
		{Group: "default", Model: "sora-main", ChannelId: channels[1].Id, Enabled: true},
		{Group: "default", Model: "disabled-video", ChannelId: channels[1].Id, Enabled: true},
		{Group: unicodeGroup, Model: "sora-main", ChannelId: channels[1].Id, Enabled: true},
		{Group: unicodeGroup, Model: "custom-video", ChannelId: channels[2].Id, Enabled: true},
		{Group: unicodeGroup, Model: "disabled-channel-video", ChannelId: channels[3].Id, Enabled: true},
		{Group: "llm-only", Model: "gpt-4o-mini", ChannelId: channels[4].Id, Enabled: true},
	}).Error)
	require.NoError(t, model.DB.Create(&[]model.Model{
		{ModelName: "custom-video", Status: 1, Endpoints: `{"openai-video":"/v1/videos","openai":{"path":"/v1/audio/speech"}}`},
		{ModelName: "disabled-video", Status: 1, Endpoints: `{"openai-video":"/v1/videos"}`},
	}).Error)
	require.NoError(t, model.DB.Model(&model.Model{}).Where("model_name = ?", "disabled-video").Update("status", 0).Error)
	token, _, err := service.IssueVideoStudioToken(identity, time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)
	router := gin.New()
	router.Use(VideoStudioBridgeAuth())
	router.GET("/catalog", GetVideoStudioCatalog)
	router.GET("/v1/models", VideoStudioGroupAuthorization(), GetVideoStudioModels)

	request := httptest.NewRequest(http.MethodGet, "/catalog", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var catalog struct {
		Success bool `json:"success"`
		Data    struct {
			Groups []service.VideoStudioCatalogGroup `json:"groups"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &catalog))
	require.True(t, catalog.Success)
	require.Len(t, catalog.Data.Groups, 2)
	assert.Equal(t, "default", catalog.Data.Groups[0].ID)
	assert.Equal(t, []service.VideoStudioCatalogModel{{ID: "sora-main", Capabilities: []string{"video"}}}, catalog.Data.Groups[0].Models)
	assert.Equal(t, unicodeGroup, catalog.Data.Groups[1].ID)
	assert.Equal(t, 2.0, catalog.Data.Groups[1].Ratio)
	assert.Equal(t, []service.VideoStudioCatalogModel{
		{ID: "custom-video", Capabilities: []string{"video", "text", "audio"}},
		{ID: "sora-main", Capabilities: []string{"video"}},
	}, catalog.Data.Groups[1].Models)

	request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set(videoStudioGroupHeader, url.PathEscape(unicodeGroup))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `"id":"custom-video"`)
	assert.Contains(t, response.Body.String(), `"id":"sora-main"`)
	assert.NotContains(t, response.Body.String(), "gpt-4o")
}

func TestVideoStudioRejectsUnauthorizedOrMalformedSelectedGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("VIDEO_STUDIO_DIST_DIR", t.TempDir())
	identity, session := setupImageStudioSession(t, common.RoleCommonUser)
	configureVideoStudioRelayGroups(t)
	token, _, err := service.IssueVideoStudioToken(identity, time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)
	router := videoStudioRelayTestRouter()
	for _, group := range []string{"denied", "%zz"} {
		t.Run(group, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/video-studio/v1/models", nil)
			request.Header.Set("Authorization", "Bearer "+token)
			request.Header.Set(videoStudioGroupHeader, group)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assert.Equal(t, http.StatusForbidden, response.Code)

			submit := httptest.NewRequest(http.MethodPost, "/api/video-studio/v1/videos", strings.NewReader(`{"model":"sora-2","prompt":"must not reach relay"}`))
			submit.Header.Set("Content-Type", "application/json")
			submit.Header.Set("Authorization", "Bearer "+token)
			submit.Header.Set(videoStudioGroupHeader, group)
			submitResponse := httptest.NewRecorder()
			router.ServeHTTP(submitResponse, submit)
			assert.Equal(t, http.StatusForbidden, submitResponse.Code)
		})
	}
}

func TestVideoStudioRejectsNonVideoModelBeforeBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("VIDEO_STUDIO_DIST_DIR", t.TempDir())
	initModelListColumnNames(t)
	identity, session := setupImageStudioSession(t, common.RoleCommonUser)
	configureVideoStudioRelayGroups(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Task{}, &model.Model{}))
	const initialQuota = 10_000
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", identity.UserID).Updates(map[string]any{
		"quota": initialQuota, "setting": `{"billing_preference":"wallet_only"}`,
	}).Error)
	token, _, err := service.IssueVideoStudioToken(identity, time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI, Key: "llm-key", Status: common.ChannelStatusEnabled, Name: "llm-only", Group: "default", Models: "gpt-4o"}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "default", Model: "gpt-4o", ChannelId: channel.Id, Enabled: true}).Error)

	request := videoStudioMultipartVideoRequest(t, token)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-4o"))
	require.NoError(t, writer.WriteField("prompt", "must not bill"))
	require.NoError(t, writer.Close())
	request = httptest.NewRequest(http.MethodPost, "/api/video-studio/v1/videos", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	videoStudioRelayTestRouter().ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
	quota, err := model.GetUserQuota(identity.UserID, false)
	require.NoError(t, err)
	assert.Equal(t, initialQuota, quota)
	var tasks int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&tasks).Error)
	assert.Zero(t, tasks)
}

func TestVideoStudioSelectedGroupIsUsedForChargeAndTaskSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("VIDEO_STUDIO_DIST_DIR", t.TempDir())
	initModelListColumnNames(t)
	identity, session := setupImageStudioSession(t, common.RoleCommonUser)
	originalGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalModelRatios := ratio_setting.ModelRatio2JSONString()
	originalSelfUseMode := operation_setting.SelfUseModeEnabled
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","selected":"Selected"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"selected":2}`))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"sora-2":1}`))
	operation_setting.SelfUseModeEnabled = true
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalModelRatios))
		operation_setting.SelfUseModeEnabled = originalSelfUseMode
	})
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCache })
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Task{}, &model.Model{}))
	const initialQuota = 10_000_000
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", identity.UserID).Updates(map[string]any{
		"quota": initialQuota, "setting": `{"billing_preference":"wallet_only"}`,
	}).Error)
	token, _, err := service.IssueVideoStudioToken(identity, time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Empty(t, request.Header.Get(videoStudioGroupHeader))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"upstream-selected","object":"video","model":"sora-2","status":"queued","progress":0,"created_at":123}`))
	}))
	t.Cleanup(upstream.Close)
	channel := &model.Channel{Type: constant.ChannelTypeSora, Key: "selected-key", Status: common.ChannelStatusEnabled, Name: "selected", Group: "selected", Models: "sora-2", BaseURL: &upstream.URL}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "selected", Model: "sora-2", ChannelId: channel.Id, Enabled: true}).Error)

	request := videoStudioMultipartVideoRequest(t, token)
	request.Header.Set(videoStudioGroupHeader, "selected")
	response := httptest.NewRecorder()
	videoStudioRelayTestRouter().ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var submit struct {
		ID string `json:"id"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &submit))
	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ?", submit.ID).First(&task).Error)
	assert.Equal(t, "selected", task.Group)
	require.NotNil(t, task.PrivateData.BillingContext)
	assert.Equal(t, 2.0, task.PrivateData.BillingContext.GroupRatio)
	quota, err := model.GetUserQuota(identity.UserID, false)
	require.NoError(t, err)
	assert.Equal(t, initialQuota-task.Quota, quota)
}
