package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const studioVideoTerminalModelError = `{"error":{"code":"model_not_found","message":"No available channel for this model under the current group (request id: 202611111516012014442748268d9d6Pmkgvc2g)","type":"new_api_error"}}`
const studioVideoTerminalReceipt = `{"code":"fail_to_fetch_task","message":"{\"error\":{\"code\":\"model_not_found\",\"message\":\"No available channel for this model under the current group (request id: 202611111516012014442748268d9d6Pmkgvc2g)\",\"type\":\"new_api_error\"}}","data":null}`

func TestStudioVideoTerminalCapturedResponseError(t *testing.T) {
	t.Setenv("VIDEO_STUDIO_DATA_DIR", t.TempDir())
	for _, test := range []struct {
		name, kind, body, marker, wantReason string
		status                               int
	}{
		{"initial quoted receipt", "video", studioVideoTerminalReceipt, "503", "selected video model is unavailable upstream", 503},
		{"saved quoted receipt", "video", studioVideoTerminalReceipt, "503", "selected video model is unavailable upstream", 0},
		{"legacy quoted receipt", "video", studioVideoTerminalReceipt, "", "selected video model is unavailable upstream", 0},
		{"unknown nested rejection", "video", `{"code":"fail_to_fetch_task","message":"{\"error\":{\"code\":\"unknown_upstream_error\",\"message\":\"private details\"}}"}`, "503", "", 0},
		{"nested quota with server error", "video", `{"code":"fail_to_fetch_task","message":"{\"code\":\"insufficient_user_quota\"}"}`, "503", "", 0},
		{"transport error", "video", `{"code":"do_request_failed","message":"model_not_found: connection reset"}`, "503", "", 0},
		{"unstructured message", "video", `{"code":"fail_to_fetch_task","message":"model_not_found: gateway timeout"}`, "503", "", 0},
		{"unwrapped server error", "video", studioVideoTerminalModelError, "503", "", 0},
		{"other gateway status", "video", studioVideoTerminalReceipt, "502", "", 0},
		{"successful status", "video", studioVideoTerminalReceipt, "200", "", 0},
		{"invalid status marker", "video", studioVideoTerminalReceipt, "unknown", "", 0},
		{"image server error", "image", studioVideoTerminalReceipt, "503", "", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			job := &model.StudioJob{ID: "studio_" + common.GetUUID(), Kind: test.kind}
			require.NoError(t, service.StoreStudioFile(job.ID, "response.bin", strings.NewReader(test.body), 0))
			if test.marker != "" {
				require.NoError(t, service.StoreStudioFile(job.ID, "relay-status", strings.NewReader(test.marker), 0))
			}
			reason, rejected := studioCapturedResponseError(job, test.status)
			assert.True(t, rejected)
			assert.Equal(t, test.wantReason, reason)
		})
	}
}

func TestStudioVideoTerminalSubmissionNoReplay(t *testing.T) {
	for _, test := range []struct {
		name, body, wantStatus, wantReason string
	}{
		{"nested model unavailable", studioVideoTerminalModelError, model.StudioJobFailed, "selected video model is unavailable upstream"},
		{"ambiguous structured server error", `{"error":{"code":"unknown_upstream_error","message":"private details"}}`, model.StudioJobUncertain, "upstream submission could not be confirmed; do not automatically resubmit"},
		{"ambiguous gateway response", `gateway timeout`, model.StudioJobUncertain, "upstream submission could not be confirmed; do not automatically resubmit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			router, token, userID := setupStudioJobsTest(t)
			t.Setenv("VIDEO_STUDIO_PAUSE_FILE", "")
			t.Setenv("VIDEO_STUDIO_IMAGE_CONCURRENCY", "1")
			require.NoError(t, model.DB.AutoMigrate(&model.ChannelStatusCheck{}))
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = io.WriteString(w, test.body)
			}))
			defer upstream.Close()
			createStudioJobChannel(t, upstream.URL, "sora-2", constant.ChannelTypeSora)
			job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/videos",
				`{"model":"sora-2","prompt":"offline terminal rejection fixture","seconds":"4"}`)
			require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
			require.NoError(t, runVideoStudioJobs(context.Background(), "terminal-submission"))
			got, err := model.FindStudioJob(userID, job.ID)
			require.NoError(t, err)
			assert.Equal(t, test.wantStatus, got.Status)
			assert.Equal(t, test.wantReason, got.Error)
			assert.Equal(t, job.TaskID, got.TaskID)
			assert.NotContains(t, got.Error, "202611111516012014442748268d9d6Pmkgvc2g")
			marker, err := os.ReadFile(service.VideoStudioJobPath(job.ID, "relay-status"))
			require.NoError(t, err)
			assert.Equal(t, "503", string(marker))
			captured, err := os.ReadFile(service.VideoStudioJobPath(job.ID, "response.bin"))
			require.NoError(t, err)
			if test.wantStatus == model.StudioJobFailed {
				assert.Equal(t, studioVideoTerminalReceipt, string(captured))
			}
			_, exists, err := model.GetByTaskId(userID, got.TaskID)
			require.NoError(t, err)
			assert.False(t, exists, "a rejected submission has no persisted native task")
			require.NoError(t, model.DB.Model(&model.StudioJob{}).Where("id = ?", job.ID).
				Update("updated_at", common.GetTimestamp()-301).Error)
			require.NoError(t, runVideoStudioJobs(context.Background(), "terminal-no-replay"))
			got, err = model.FindStudioJob(userID, job.ID)
			require.NoError(t, err)
			assert.Equal(t, test.wantStatus, got.Status)
			assert.Equal(t, test.wantReason, got.Error)
			assert.Equal(t, int32(1), calls.Load(), "submission and reconciliation must never replay the request")
			quota, err := model.GetUserQuota(userID, false)
			require.NoError(t, err)
			assert.Equal(t, 10_000_000, quota, "a rejected submission must not charge the user")
		})
	}
}

func TestStudioVideoTerminalReconciliation(t *testing.T) {
	for _, test := range []struct {
		name, body, initialStatus, initialError, wantStatus, wantReason string
		nativeTask                                                      bool
	}{
		{"submitted receipt", studioVideoTerminalReceipt, model.StudioJobSubmitted, "", model.StudioJobFailed, "selected video model is unavailable upstream", false},
		{"uncertain receipt", studioVideoTerminalReceipt, model.StudioJobUncertain, "original uncertain reason", model.StudioJobFailed, "selected video model is unavailable upstream", false},
		{"unknown nested rejection", `{"code":"fail_to_fetch_task","message":"{\"error\":{\"code\":\"unknown_upstream_error\",\"message\":\"private details\"}}"}`, model.StudioJobUncertain, "original uncertain reason", model.StudioJobUncertain, "original uncertain reason", false},
		{"transport error", `{"code":"do_request_failed","message":"connection reset"}`, model.StudioJobUncertain, "original uncertain reason", model.StudioJobUncertain, "original uncertain reason", false},
		{"gateway response", `gateway timeout`, model.StudioJobUncertain, "original uncertain reason", model.StudioJobUncertain, "original uncertain reason", false},
		{"persisted task takes precedence", studioVideoTerminalReceipt, model.StudioJobSubmitted, "", model.StudioJobSubmitted, "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, userID := setupStudioJobsTest(t)
			t.Setenv("VIDEO_STUDIO_PAUSE_FILE", "")
			t.Setenv("VIDEO_STUDIO_IMAGE_CONCURRENCY", "1")
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				http.Error(w, "unexpected generation request", http.StatusInternalServerError)
			}))
			defer upstream.Close()
			createStudioJobChannel(t, upstream.URL, "sora-2", constant.ChannelTypeSora)
			job := &model.StudioJob{
				ID: "studio_" + common.GetUUID(), UserID: userID, RequestID: common.GetUUID(),
				Kind: "video", Operation: "videos", Group: "default", Model: "sora-2", ContentType: "application/json",
				TaskID: model.GenerateTaskID(), Status: test.initialStatus, Error: test.initialError,
				RunnerTask: "old-terminal-run", UpdatedAt: common.GetTimestamp() - 301,
			}
			require.NoError(t, model.DB.Create(job).Error)
			if test.nativeTask {
				require.NoError(t, model.DB.Create(&model.Task{
					TaskID: job.TaskID, UserId: userID, Status: model.TaskStatusQueued,
					PrivateData: model.TaskPrivateData{UpstreamTaskID: "accepted-native-task"},
				}).Error)
			}
			require.NoError(t, service.StoreStudioFile(job.ID, "request.bin",
				strings.NewReader(`{"model":"sora-2","prompt":"must not replay","seconds":"4"}`), 0))
			require.NoError(t, service.StoreStudioFile(job.ID, "response.bin", strings.NewReader(test.body), 0))
			require.NoError(t, service.StoreStudioFile(job.ID, "relay-status", strings.NewReader("503"), 0))
			for _, run := range []string{"terminal-reconcile", "terminal-reconcile-again"} {
				require.NoError(t, runVideoStudioJobs(context.Background(), run))
				got, err := model.FindStudioJob(userID, job.ID)
				require.NoError(t, err)
				assert.Equal(t, test.wantStatus, got.Status)
				assert.Equal(t, test.wantReason, got.Error)
				assert.Equal(t, job.TaskID, got.TaskID)
			}
			assert.Zero(t, calls.Load(), "evidence-only reconciliation must not submit or charge again")
			native, exists, err := model.GetByTaskId(userID, job.TaskID)
			require.NoError(t, err)
			require.Equal(t, test.nativeTask, exists)
			if exists {
				assert.Equal(t, model.TaskStatus(model.TaskStatusQueued), native.Status)
				assert.Equal(t, "accepted-native-task", native.PrivateData.UpstreamTaskID)
			}
			captured, err := os.ReadFile(service.VideoStudioJobPath(job.ID, "response.bin"))
			require.NoError(t, err)
			assert.Equal(t, test.body, string(captured), "reconciliation must preserve the captured evidence")
			quota, err := model.GetUserQuota(userID, false)
			require.NoError(t, err)
			assert.Equal(t, 10_000_000, quota)
		})
	}
}
