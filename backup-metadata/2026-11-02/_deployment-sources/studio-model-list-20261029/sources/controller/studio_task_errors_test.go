package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudioVideoLocalRejectionIsSpecificLoggedOnceAndFree(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	beforePrices := ratio_setting.ModelPrice2JSONString()
	oldErrorLog := constant.ErrorLogEnabled
	constant.ErrorLogEnabled = true
	t.Cleanup(func() {
		constant.ErrorLogEnabled = oldErrorLog
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(beforePrices))
	})
	require.NoError(t, model.DB.AutoMigrate(&model.Log{}))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"sora-2":0.3}`))
	createStudioJobChannel(t, "https://www.gway.dev", "sora-2", constant.ChannelTypeOpenAI)
	endpoints, err := common.Marshal(map[string]string{string(constant.EndpointTypeOpenAIVideo): "/v1/videos"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.Model{
		ModelName: "sora-2", Status: 1, Endpoints: string(endpoints),
		Parameters: `{"provider":"gway","video":{"currency":"CNY","billing_unit":"second","durations":[5,30],"resolutions":["720p"]}}`,
	}).Error)
	job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/videos",
		`{"model":"sora-2","prompt":"local rejection only","seconds":"30"}`)
	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	require.NoError(t, runVideoStudioJobs(context.Background(), "local-rejection"))
	persisted, err := model.FindStudioJob(userID, job.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StudioJobFailed, persisted.Status)
	assert.Contains(t, persisted.Error, "Gway video price is not configured")
	assert.Contains(t, persisted.Error, "invalid_gway_video_request")
	assert.Contains(t, persisted.Error, "HTTP 400")
	assert.Contains(t, persisted.Error, job.ID)
	require.NoError(t, runVideoStudioJobs(context.Background(), "no-local-replay"))
	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("request_id = ?", job.ID).Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Equal(t, model.LogTypeError, logs[0].Type)
	assert.Zero(t, logs[0].Quota)
	assert.Contains(t, logs[0].Content, "Gway video price is not configured")
	var other map[string]any
	require.NoError(t, common.UnmarshalJsonStr(logs[0].Other, &other))
	assert.Equal(t, true, other["local_error"])
	assert.Equal(t, "invalid_gway_video_request", other["error_code"])
	assert.Equal(t, job.ID, other["studio_job_id"])
	quota, err := model.GetUserQuota(userID, false)
	require.NoError(t, err)
	assert.Equal(t, 10_000_000, quota)
	var tasks int64
	require.NoError(t, model.DB.Model(&model.Task{}).Where("user_id = ?", userID).Count(&tasks).Error)
	assert.Zero(t, tasks)
	var channel model.Channel
	require.NoError(t, model.DB.First(&channel).Error)
	assert.Equal(t, common.ChannelStatusEnabled, channel.Status)
	get := httptest.NewRequest(http.MethodGet, "/jobs/"+job.ID, nil)
	get.Header.Set("Authorization", "Bearer "+token)
	view := httptest.NewRecorder()
	router.ServeHTTP(view, get)
	assert.Equal(t, http.StatusOK, view.Code)
	assert.Contains(t, view.Body.String(), "Gway video price is not configured")
	assert.Contains(t, view.Body.String(), job.ID)
}

func TestStudioTaskErrorRedactsCredentialsAndDoesNotDoubleLogRemoteError(t *testing.T) {
	_, _, userID := setupStudioJobsTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Log{}))
	oldErrorLog := constant.ErrorLogEnabled
	constant.ErrorLogEnabled = true
	t.Cleanup(func() { constant.ErrorLogEnabled = oldErrorLog })
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	c.Set("id", userID)
	c.Set(common.RequestIdKey, "remote-already-logged")
	respondTaskError(c, &dto.TaskError{
		Code: "fail_to_fetch_task", Message: "denied Bearer secret-value api_key=secret-key sk-abcd12345678",
		StatusCode: http.StatusForbidden,
	})
	var count int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("request_id = ?", "remote-already-logged").Count(&count).Error)
	assert.Zero(t, count)
	message := studioTaskFailureMessage(403, "denied", "denied Bearer secret-value api_key=secret-key sk-abcd12345678", "studio-reference")
	assert.Contains(t, message, "denied")
	assert.NotContains(t, message, "secret-value")
	assert.NotContains(t, message, "secret-key")
	assert.NotContains(t, message, "sk-abcd12345678")
}
