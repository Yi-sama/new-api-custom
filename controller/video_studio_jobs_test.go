package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupStudioJobsTest(t *testing.T) (*gin.Engine, string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("VIDEO_STUDIO_DIST_DIR", t.TempDir())
	t.Setenv("VIDEO_STUDIO_DATA_DIR", t.TempDir())
	initModelListColumnNames(t)
	identity, session := setupImageStudioSession(t, common.RoleCommonUser)
	configureVideoStudioRelayGroups(t)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"sora-2":1,"gpt-image-1":1}`))
	previousCache, previousRetry := common.MemoryCacheEnabled, common.RetryTimes
	common.MemoryCacheEnabled, common.RetryTimes = false, 3
	t.Cleanup(func() { common.MemoryCacheEnabled, common.RetryTimes = previousCache, previousRetry })
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Task{}, &model.Model{}, &model.StudioJob{}, &model.SystemTask{}, &model.SystemTaskLock{}))
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", identity.UserID).Updates(map[string]any{
		"quota": 10_000_000, "setting": `{"billing_preference":"wallet_only"}`,
	}).Error)
	token, _, err := service.IssueVideoStudioToken(identity, time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)
	router := gin.New()
	router.Use(VideoStudioBridgeAuth())
	router.POST("/jobs/images/generations", VideoStudioGroupAuthorization(), CreateVideoStudioJob("image", "images/generations"))
	router.POST("/jobs/images/edits", VideoStudioGroupAuthorization(), CreateVideoStudioJob("image", "images/edits"))
	router.POST("/jobs/videos", VideoStudioGroupAuthorization(), CreateVideoStudioJob("video", "videos"))
	router.GET("/jobs", ListVideoStudioJobs)
	router.GET("/jobs/:id", GetVideoStudioJob)
	router.GET("/jobs/:id/result", GetVideoStudioJobResult)
	router.GET("/jobs/:id/assets/:asset", GetVideoStudioJobResult)
	return router, token, identity.UserID
}

func createStudioJobChannel(t *testing.T, url, modelName string, channelType int) {
	t.Helper()
	channel := &model.Channel{Type: channelType, Key: "local-dummy-key", Status: common.ChannelStatusEnabled, Name: "job-test", Group: "default", Models: modelName, BaseURL: &url}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "default", Model: modelName, ChannelId: channel.Id, Enabled: true}).Error)
}

func submitStudioJobTest(t *testing.T, router *gin.Engine, token, key, path, body string) (model.StudioJob, *httptest.ResponseRecorder) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Idempotency-Key", key)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var envelope struct {
		Data model.StudioJob `json:"data"`
	}
	if response.Code < 300 {
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &envelope))
	}
	return envelope.Data, response
}

func TestVideoStudioJobSurvivesDisconnectAndBillsOnce(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	var calls atomic.Int32
	png := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jfZkAAAAASUVORK5CYII="
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/v1/images/generations", r.URL.Path)
		assert.Equal(t, "Bearer local-dummy-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"created":1,"data":[{"b64_json":%q}]}`, png)
	}))
	defer upstream.Close()
	createStudioJobChannel(t, upstream.URL, "gpt-image-1", constant.ChannelTypeOpenAI)
	requestContext, disconnect := context.WithCancel(context.Background())
	key := common.GetUUID()
	body := `{"model":"gpt-image-1","prompt":"synthetic offline test","n":1,"size":"1024x1024"}`
	request := httptest.NewRequest(http.MethodPost, "/jobs/images/generations", strings.NewReader(body)).WithContext(requestContext)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Idempotency-Key", key)
	request.Header.Set("Content-Type", "application/json")
	receipt := httptest.NewRecorder()
	router.ServeHTTP(receipt, request)
	require.Equal(t, http.StatusAccepted, receipt.Code, receipt.Body.String())
	assert.Zero(t, calls.Load(), "acceptance must not wait for or execute upstream")
	disconnect()
	var envelope struct{ Data model.StudioJob }
	require.NoError(t, common.Unmarshal(receipt.Body.Bytes(), &envelope))
	job := envelope.Data
	require.NoError(t, runVideoStudioJobs(context.Background(), "test-run"))
	persisted, err := model.FindStudioJob(userID, job.ID)
	require.NoError(t, err)
	require.Equal(t, model.StudioJobSucceeded, persisted.Status, persisted.Error)
	quota, err := model.GetUserQuota(userID, false)
	require.NoError(t, err)
	assert.Less(t, quota, 10_000_000)

	duplicate, response := submitStudioJobTest(t, router, token, key, "/jobs/images/generations", body)
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, job.ID, duplicate.ID)
	require.NoError(t, runVideoStudioJobs(context.Background(), "after-restart"))
	assert.Equal(t, int32(1), calls.Load())
	quotaAgain, err := model.GetUserQuota(userID, false)
	require.NoError(t, err)
	assert.Equal(t, quota, quotaAgain)

	result := httptest.NewRecorder()
	get := httptest.NewRequest(http.MethodGet, "/jobs/"+job.ID+"/result", nil)
	get.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(result, get)
	require.Equal(t, http.StatusOK, result.Code, result.Body.String())
	assert.Contains(t, result.Body.String(), "/api/video-studio/jobs/"+job.ID+"/assets/0")
	assert.NotContains(t, result.Body.String(), "local-dummy-key")
	get = httptest.NewRequest(http.MethodGet, "/jobs/"+job.ID+"/assets/0", nil)
	get.Header.Set("Authorization", "Bearer "+token)
	result = httptest.NewRecorder()
	router.ServeHTTP(result, get)
	require.Equal(t, http.StatusOK, result.Code)
	expected, err := base64.StdEncoding.DecodeString(png)
	require.NoError(t, err)
	assert.Equal(t, expected, result.Body.Bytes())
	assert.Equal(t, "private, no-store", result.Header().Get("Cache-Control"))

	otherToken, _ := createVideoStudioRelayUser(t, "job-other-user")
	for _, path := range []string{"/jobs/" + job.ID, "/jobs/" + job.ID + "/result", "/jobs/" + job.ID + "/assets/0"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+otherToken)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		assert.Equal(t, http.StatusNotFound, response.Code)
	}
	_, response = submitStudioJobTest(t, router, token, key, "/jobs/images/generations", strings.Replace(body, "offline", "changed", 1))
	assert.Equal(t, http.StatusConflict, response.Code)
}

func TestVideoStudioImageEditJobPreservesReferenceAndBillsOnce(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	const png = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jfZkAAAAASUVORK5CYII="
	reference, err := base64.StdEncoding.DecodeString(png)
	require.NoError(t, err)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/v1/images/edits", r.URL.Path)
		require.NoError(t, r.ParseMultipartForm(1<<20))
		defer r.MultipartForm.RemoveAll()
		assert.Equal(t, "1", r.FormValue("n"))
		assert.Equal(t, "1024x1024", r.FormValue("size"))
		image, _, err := r.FormFile("image")
		require.NoError(t, err)
		got, err := io.ReadAll(image)
		require.NoError(t, err)
		require.NoError(t, image.Close())
		assert.Equal(t, reference, got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"created":1,"data":[{"b64_json":%q}]}`, png)
	}))
	defer upstream.Close()
	createStudioJobChannel(t, upstream.URL, "gpt-image-1", constant.ChannelTypeOpenAI)
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for key, value := range map[string]string{
		"model": "gpt-image-1", "prompt": "edit the reference", "n": "1", "size": "1024x1024",
	} {
		require.NoError(t, form.WriteField(key, value))
	}
	image, err := form.CreateFormFile("image", "reference.png")
	require.NoError(t, err)
	_, err = image.Write(reference)
	require.NoError(t, err)
	require.NoError(t, form.Close())
	request := httptest.NewRequest(http.MethodPost, "/jobs/images/edits", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Idempotency-Key", common.GetUUID())
	receipt := httptest.NewRecorder()
	router.ServeHTTP(receipt, request)
	require.Equal(t, http.StatusAccepted, receipt.Code, receipt.Body.String())
	var envelope struct{ Data model.StudioJob }
	require.NoError(t, common.Unmarshal(receipt.Body.Bytes(), &envelope))
	require.NoError(t, runVideoStudioJobs(context.Background(), "edit-regression"))
	job, err := model.FindStudioJob(userID, envelope.Data.ID)
	require.NoError(t, err)
	require.Equal(t, model.StudioJobSucceeded, job.Status, job.Error)
	result, err := os.ReadFile(service.VideoStudioJobPath(job.ID, "image-0"))
	require.NoError(t, err)
	assert.Equal(t, reference, result)
	quota, err := model.GetUserQuota(userID, false)
	require.NoError(t, err)
	assert.Less(t, quota, 10_000_000)
	require.NoError(t, runVideoStudioJobs(context.Background(), "edit-after-restart"))
	assert.Equal(t, int32(1), calls.Load())
	quotaAgain, err := model.GetUserQuota(userID, false)
	require.NoError(t, err)
	assert.Equal(t, quota, quotaAgain)
}

func TestVideoStudioJobMultipartIdempotencyAndNativeRecovery(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"upstream-native","object":"video","model":"sora-2","status":"queued"}`))
	}))
	defer upstream.Close()
	createStudioJobChannel(t, upstream.URL, "sora-2", constant.ChannelTypeSora)
	key := common.GetUUID()
	var job model.StudioJob
	for index := 0; index < 2; index++ {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		require.NoError(t, form.WriteField("model", "sora-2"))
		require.NoError(t, form.WriteField("prompt", "multipart boundary test"))
		require.NoError(t, form.WriteField("seconds", "4"))
		file, err := form.CreateFormFile("input_reference", "frame.png")
		require.NoError(t, err)
		_, err = file.Write([]byte("synthetic-image"))
		require.NoError(t, err)
		require.NoError(t, form.Close())
		request := httptest.NewRequest(http.MethodPost, "/jobs/videos", &body)
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Idempotency-Key", key)
		request.Header.Set("Content-Type", form.FormDataContentType())
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Less(t, response.Code, 300, response.Body.String())
		var envelope struct{ Data model.StudioJob }
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &envelope))
		if index == 0 {
			job = envelope.Data
		} else {
			assert.Equal(t, job.ID, envelope.Data.ID)
		}
	}
	require.NoError(t, runVideoStudioJobs(context.Background(), "first"))
	persisted, err := model.FindStudioJob(userID, job.ID)
	require.NoError(t, err)
	require.Equal(t, model.StudioJobSubmitted, persisted.Status, persisted.Error)
	task, exists, err := model.GetByTaskId(userID, persisted.TaskID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, "upstream-native", task.PrivateData.UpstreamTaskID)
	require.NoError(t, runVideoStudioJobs(context.Background(), "recovery"))
	assert.Equal(t, int32(1), calls.Load())
}

func TestVideoStudioJobAmbiguousSubmissionNeverRetries(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "ambiguous upstream error", http.StatusBadGateway)
	}))
	defer upstream.Close()
	createStudioJobChannel(t, upstream.URL, "sora-2", constant.ChannelTypeSora)
	job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/videos", `{"model":"sora-2","prompt":"test","seconds":"4"}`)
	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	require.NoError(t, runVideoStudioJobs(context.Background(), "first"))
	persisted, err := model.FindStudioJob(userID, job.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StudioJobUncertain, persisted.Status)
	require.NoError(t, runVideoStudioJobs(context.Background(), "second"))
	assert.Equal(t, int32(1), calls.Load(), "configured relay retries must not replay an uncertain studio submission")
}

func TestVideoStudioImagePolicyRejectionFailsWithoutReplayOrCharge(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		code       string
		wantStatus string
		wantError  string
	}{
		{"policy rejection", http.StatusBadRequest, "content_policy_violation", model.StudioJobFailed,
			"image generation was rejected by upstream content policy"},
		{"moderation rejection", http.StatusBadRequest, "moderation_blocked", model.StudioJobFailed,
			"image generation was rejected by upstream content policy"},
		{"other bad request", http.StatusBadRequest, "unknown_upstream_error", model.StudioJobUncertain,
			"upstream submission could not be confirmed; do not automatically resubmit"},
		{"policy code with server error", http.StatusServiceUnavailable, "content_policy_violation", model.StudioJobUncertain,
			"upstream submission could not be confirmed; do not automatically resubmit"},
		{"moderation code with server error", http.StatusServiceUnavailable, "moderation_blocked", model.StudioJobUncertain,
			"upstream submission could not be confirmed; do not automatically resubmit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, token, userID := setupStudioJobsTest(t)
			require.NoError(t, model.DB.AutoMigrate(&model.ChannelStatusCheck{}))
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = fmt.Fprintf(w, `{"error":{"message":"private upstream policy details","type":"invalid_request_error","code":%q}}`, tt.code)
			}))
			defer upstream.Close()
			createStudioJobChannel(t, upstream.URL, "gpt-image-1", constant.ChannelTypeOpenAI)
			job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/images/generations",
				`{"model":"gpt-image-1","prompt":"policy fixture","n":1,"size":"1024x1024"}`)
			require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())

			require.NoError(t, runVideoStudioJobs(context.Background(), "policy-first"))
			persisted, err := model.FindStudioJob(userID, job.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, persisted.Status)
			assert.Equal(t, tt.wantError, persisted.Error)
			require.Eventually(t, func() bool {
				quota, err := model.GetUserQuota(userID, false)
				return err == nil && quota == 10_000_000
			}, time.Second, 10*time.Millisecond, "an unsuccessful relay must refund its reservation")
			captured, err := os.ReadFile(service.VideoStudioJobPath(job.ID, "response.bin"))
			require.NoError(t, err)
			if tt.wantStatus == model.StudioJobFailed {
				assert.Contains(t, string(captured), `"code":"content_policy_violation"`)
				assert.NotContains(t, string(captured), "private upstream")
			}

			require.NoError(t, runVideoStudioJobs(context.Background(), "policy-second"))
			assert.Equal(t, int32(1), calls.Load())
			quota, err := model.GetUserQuota(userID, false)
			require.NoError(t, err)
			assert.Equal(t, 10_000_000, quota)
			_, err = os.Stat(service.VideoStudioJobPath(job.ID, "result.json"))
			assert.True(t, os.IsNotExist(err))
		})
	}
}

func TestVideoStudioJobCrashRecoveryDoesNotResubmit(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	createStudioJobChannel(t, "http://127.0.0.1:1", "sora-2", constant.ChannelTypeSora)
	job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/videos", `{"model":"sora-2","prompt":"test"}`)
	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	claimed, err := model.ClaimStudioJob("crashed-run")
	require.NoError(t, err)
	require.Equal(t, job.ID, claimed.ID)
	require.NoError(t, runVideoStudioJobs(context.Background(), "new-run"))
	persisted, err := model.FindStudioJob(userID, job.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StudioJobUncertain, persisted.Status)
	_, err = os.Stat(service.VideoStudioJobPath(job.ID, "response.bin"))
	assert.True(t, os.IsNotExist(err), "recovery must not start an upstream request")
}

func TestVideoStudioJobValidatesParametersBeforeQueueing(t *testing.T) {
	router, token, _ := setupStudioJobsTest(t)
	createStudioJobChannel(t, "http://127.0.0.1:1", "sora-2", constant.ChannelTypeSora)
	require.NoError(t, model.DB.Create(&model.Model{
		ModelName: "sora-2", Status: 1, Parameters: `{"video":{"durations":[4],"resolutions":["720p"]}}`,
	}).Error)
	for _, body := range []string{
		`{"model":"sora-2","prompt":"test","seconds":"8"}`,
		`{"model":"sora-2","prompt":"test","metadata":{"duration":8}}`,
		`{"model":"sora-2","prompt":"test","resolution_name":"1080p"}`,
		`{"model":"sora-2","prompt":"test","seconds":"4","duration":8}`,
		`{"model":"sora-2","prompt":"test","parameters":{"n":1000000000}}`,
	} {
		_, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/videos", body)
		assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
	}
	var count int64
	require.NoError(t, model.DB.Model(&model.StudioJob{}).Count(&count).Error)
	assert.Zero(t, count)
	job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/videos", `{"model":"sora-2","prompt":"test","seconds":"4","resolution_name":"720p"}`)
	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	assert.Equal(t, model.StudioJobQueued, job.Status)
}

func TestVideoStudioJobRecoversNativeVideoResultWithoutResubmission(t *testing.T) {
	_, _, userID := setupStudioJobsTest(t)
	channel := &model.Channel{Type: constant.ChannelTypeVidu, Key: "dummy", Status: common.ChannelStatusEnabled, Name: "saved-video"}
	require.NoError(t, model.DB.Create(channel).Error)
	media := []byte("synthetic-video-bytes")
	native := &model.Task{
		TaskID: "task-completed", UserId: userID, ChannelId: channel.Id, Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{ResultURL: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(media)},
	}
	require.NoError(t, model.DB.Create(native).Error)
	job := &model.StudioJob{
		ID: "studio_" + common.GetUUID(), UserID: userID, RequestID: common.GetUUID(), Kind: "video",
		TaskID: native.TaskID, Status: model.StudioJobRunning, RunnerTask: "old-run",
	}
	require.NoError(t, model.DB.Create(job).Error)
	require.NoError(t, service.StoreStudioFile(job.ID, "request.bin", strings.NewReader("already submitted"), 0))
	require.NoError(t, runVideoStudioJobs(context.Background(), "new-run"))
	persisted, err := model.FindStudioJob(userID, job.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StudioJobSucceeded, persisted.Status)
	result, err := os.ReadFile(service.VideoStudioJobPath(job.ID, "video"))
	require.NoError(t, err)
	assert.Equal(t, media, result)
}

func TestVideoStudioJobResumesCapturedImageWithoutCallingUpstream(t *testing.T) {
	_, _, userID := setupStudioJobsTest(t)
	job := &model.StudioJob{
		ID: "studio_" + common.GetUUID(), UserID: userID, RequestID: common.GetUUID(), Kind: "image",
		Status: model.StudioJobRunning, RunnerTask: "interrupted-after-response",
	}
	require.NoError(t, model.DB.Create(job).Error)
	data := []byte("persisted-image-bytes")
	response := fmt.Sprintf(`{"data":[{"b64_json":%q}]}`, base64.StdEncoding.EncodeToString(data))
	require.NoError(t, service.StoreStudioFile(job.ID, "response.bin", strings.NewReader(response), 0))
	require.NoError(t, runVideoStudioJobs(context.Background(), "new-run"))
	persisted, err := model.FindStudioJob(userID, job.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StudioJobSucceeded, persisted.Status)
	result, err := os.ReadFile(service.VideoStudioJobPath(job.ID, "image-0"))
	require.NoError(t, err)
	assert.Equal(t, data, result)
}

func TestVideoStudioJobUsesExistingDownloadLimitForCapturedResponse(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	previousLimit := constant.MaxFileDownloadMB
	constant.MaxFileDownloadMB = 1
	t.Cleanup(func() { constant.MaxFileDownloadMB = previousLimit })
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"data":[{"b64_json":%q}]}`, strings.Repeat("A", 2*1024*1024))
	}))
	defer upstream.Close()
	createStudioJobChannel(t, upstream.URL, "gpt-image-1", constant.ChannelTypeOpenAI)
	job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/images/generations", `{"model":"gpt-image-1","prompt":"oversized fixture"}`)
	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	require.NoError(t, runVideoStudioJobs(context.Background(), "first"))
	persisted, err := model.FindStudioJob(userID, job.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StudioJobUncertain, persisted.Status)
	require.NoError(t, runVideoStudioJobs(context.Background(), "second"))
	assert.Equal(t, int32(1), calls.Load())
	file, err := os.Stat(service.VideoStudioJobPath(job.ID, "response.bin"))
	require.NoError(t, err)
	assert.LessOrEqual(t, file.Size(), int64(1024*1024))
}
