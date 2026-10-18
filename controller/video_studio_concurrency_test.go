package controller

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func awaitStudioEvent[T any](t *testing.T, events <-chan T) T {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(10 * time.Second):
		t.Fatal("studio runner did not reach the expected barrier")
		var zero T
		return zero
	}
}

func TestVideoStudioImagesRunConcurrentlyWithinLimitAndBillOnce(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	t.Setenv("VIDEO_STUDIO_IMAGE_CONCURRENCY", "3")
	entered := make(chan string, 5)
	release := make(chan struct{}, 5)
	var active, peak, calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct{ Prompt string }
		if !assert.NoError(t, common.DecodeJson(r.Body, &request)) {
			return
		}
		calls.Add(1)
		count := active.Add(1)
		defer active.Add(-1)
		for previous := peak.Load(); count > previous; previous = peak.Load() {
			if peak.CompareAndSwap(previous, count) {
				break
			}
		}
		entered <- request.Prompt
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"data":[{"b64_json":%q}]}`, base64.StdEncoding.EncodeToString([]byte(request.Prompt)))
	}))
	t.Cleanup(upstream.Close)
	createStudioJobChannel(t, upstream.URL, "gpt-image-1", constant.ChannelTypeOpenAI)
	var jobs []model.StudioJob
	for i := 0; i < 5; i++ {
		job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/images/generations",
			fmt.Sprintf(`{"model":"gpt-image-1","prompt":"slot-%d","n":1,"size":"1024x1024"}`, i))
		require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
		jobs = append(jobs, job)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		done <- runVideoStudioJobs(ctx, "parallel-test")
	}()
	t.Cleanup(func() { cancel(); close(release); awaitStudioEvent(t, exited) })
	for i := 0; i < 3; i++ {
		awaitStudioEvent(t, entered)
	}
	assert.Equal(t, int32(3), active.Load())
	var queued int64
	require.NoError(t, model.DB.Model(&model.StudioJob{}).Where("status = ?", model.StudioJobQueued).Count(&queued).Error)
	assert.Equal(t, int64(2), queued, "slots at capacity must leave remaining jobs queued")
	release <- struct{}{}
	awaitStudioEvent(t, entered)
	assert.Equal(t, int32(4), calls.Load(), "a freed slot must admit the next job")
	for i := 0; i < 4; i++ {
		release <- struct{}{}
	}
	require.NoError(t, awaitStudioEvent(t, done))
	assert.Equal(t, int32(3), peak.Load())
	assert.Equal(t, int32(5), calls.Load())
	for _, job := range jobs {
		persisted, err := model.FindStudioJob(userID, job.ID)
		require.NoError(t, err)
		assert.Equal(t, model.StudioJobSucceeded, persisted.Status, persisted.Error)
		media, err := os.ReadFile(service.VideoStudioJobPath(job.ID, "image-0"))
		require.NoError(t, err)
		assert.Equal(t, job.Prompt, string(media), "parallel results must remain attached to their own job")
		var charged int64
		require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("request_id = ? AND type = ?", job.ID, model.LogTypeConsume).Count(&charged).Error)
		assert.Equal(t, int64(1), charged, "every parallel job must have exactly one consumption record")
	}
	quota, err := model.GetUserQuota(userID, false)
	require.NoError(t, err)
	assert.Less(t, quota, 10_000_000)
	for _, job := range jobs {
		duplicate, response := submitStudioJobTest(t, router, token, job.RequestID, "/jobs/images/generations",
			fmt.Sprintf(`{"model":"gpt-image-1","prompt":%q,"n":1,"size":"1024x1024"}`, job.Prompt))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Equal(t, job.ID, duplicate.ID)
	}
	require.NoError(t, runVideoStudioJobs(context.Background(), "second-pass"))
	quotaAgain, err := model.GetUserQuota(userID, false)
	require.NoError(t, err)
	assert.Equal(t, quota, quotaAgain)
	assert.Equal(t, int32(5), calls.Load())
}

func TestVideoStudioStateSaveFailureDrainsOtherRequestsAndRecoversWithoutReplay(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	t.Setenv("VIDEO_STUDIO_IMAGE_CONCURRENCY", "2")
	entered := make(chan struct{}, 3)
	release := make(chan struct{}, 3)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"c2F2ZWQ="}]}`))
	}))
	t.Cleanup(upstream.Close)
	createStudioJobChannel(t, upstream.URL, "gpt-image-1", constant.ChannelTypeOpenAI)
	var jobs []model.StudioJob
	for i := 0; i < 3; i++ {
		job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/images/generations",
			`{"model":"gpt-image-1","prompt":"storage error fixture","n":1}`)
		require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
		jobs = append(jobs, job)
	}
	saveFailure := errors.New("synthetic studio state write failure")
	failed := make(chan struct{}, 1)
	var injected atomic.Bool
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register("studio-save-failure", func(db *gorm.DB) {
		values, ok := db.Statement.Dest.(map[string]any)
		if ok && db.Statement.Table == "studio_jobs" && values["status"] == model.StudioJobSucceeded && injected.CompareAndSwap(false, true) {
			db.AddError(saveFailure)
			failed <- struct{}{}
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Update().Remove("studio-save-failure") })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		done <- runVideoStudioJobs(ctx, "save-failure")
	}()
	t.Cleanup(func() { cancel(); close(release); awaitStudioEvent(t, exited) })
	awaitStudioEvent(t, entered)
	awaitStudioEvent(t, entered)
	release <- struct{}{}
	awaitStudioEvent(t, failed)
	release <- struct{}{}
	require.ErrorIs(t, awaitStudioEvent(t, done), saveFailure)
	var queued, running, succeeded int
	for _, job := range jobs {
		persisted, err := model.FindStudioJob(userID, job.ID)
		require.NoError(t, err)
		switch persisted.Status {
		case model.StudioJobQueued:
			queued++
			require.NoError(t, model.DB.Delete(persisted).Error)
		case model.StudioJobRunning:
			running++
		case model.StudioJobSucceeded:
			succeeded++
		}
	}
	assert.Equal(t, 1, queued, "save failure must stop claiming new work")
	assert.Equal(t, 1, running, "failed state write leaves captured media available for recovery")
	assert.Equal(t, 1, succeeded, "the other in-flight job must still be saved")
	require.NoError(t, model.DB.Callback().Update().Remove("studio-save-failure"))
	require.NoError(t, runVideoStudioJobs(context.Background(), "recover-storage"))
	assert.Len(t, entered, 0, "recovery must not call upstream again")
	for _, job := range jobs {
		persisted, err := model.FindStudioJob(userID, job.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		require.NoError(t, err)
		assert.Equal(t, model.StudioJobSucceeded, persisted.Status)
	}
}

func TestVideoStudioCancellationSavesInflightJobsBeforeReturning(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	t.Setenv("VIDEO_STUDIO_IMAGE_CONCURRENCY", "2")
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		entered <- struct{}{}
		<-release
		http.Error(w, "ambiguous response", http.StatusBadGateway)
	}))
	t.Cleanup(upstream.Close)
	createStudioJobChannel(t, upstream.URL, "gpt-image-1", constant.ChannelTypeOpenAI)
	var jobs []model.StudioJob
	for i := 0; i < 3; i++ {
		job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/images/generations",
			`{"model":"gpt-image-1","prompt":"cancelled offline fixture","n":1}`)
		require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
		jobs = append(jobs, job)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		done <- runVideoStudioJobs(ctx, "cancelled-test")
	}()
	t.Cleanup(func() { cancel(); releaseOnce.Do(func() { close(release) }); awaitStudioEvent(t, exited) })
	for i := 0; i < 2; i++ {
		awaitStudioEvent(t, entered)
	}
	cancel()
	select {
	case <-done:
		t.Fatal("runner returned while upstream requests were still in flight")
	default:
	}
	releaseOnce.Do(func() { close(release) })
	require.ErrorIs(t, awaitStudioEvent(t, done), context.Canceled)
	var queued, uncertain int
	for _, job := range jobs {
		persisted, err := model.FindStudioJob(userID, job.ID)
		require.NoError(t, err)
		switch persisted.Status {
		case model.StudioJobQueued:
			queued++
			require.NoError(t, model.DB.Delete(persisted).Error)
		case model.StudioJobUncertain:
			uncertain++
		default:
			t.Errorf("in-flight state must be saved before return, got %s", persisted.Status)
		}
	}
	assert.Equal(t, 1, queued)
	assert.Equal(t, 2, uncertain)
	require.NoError(t, runVideoStudioJobs(context.Background(), "after-cancel"))
	assert.Equal(t, int32(2), calls.Load(), "interrupted requests must never be replayed")
}

func TestVideoStudioCancelledClaimDoesNotReachUpstream(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	t.Setenv("VIDEO_STUDIO_IMAGE_CONCURRENCY", "1")
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
	}))
	t.Cleanup(upstream.Close)
	createStudioJobChannel(t, upstream.URL, "gpt-image-1", constant.ChannelTypeOpenAI)
	job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/images/generations",
		`{"model":"gpt-image-1","prompt":"cancel during claim","n":1}`)
	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	claimRead := make(chan struct{}, 1)
	releaseClaim := make(chan struct{})
	var once sync.Once
	require.NoError(t, model.DB.Callback().Query().After("gorm:query").Register("cancel-during-claim", func(db *gorm.DB) {
		row, ok := db.Statement.Dest.(*model.StudioJob)
		if ok && row.ID == job.ID && row.Status == model.StudioJobQueued {
			claimRead <- struct{}{}
			<-releaseClaim
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Query().Remove("cancel-during-claim") })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		done <- runVideoStudioJobs(ctx, "claim-cancel")
	}()
	t.Cleanup(func() { cancel(); once.Do(func() { close(releaseClaim) }); awaitStudioEvent(t, exited) })
	awaitStudioEvent(t, claimRead)
	cancel()
	once.Do(func() { close(releaseClaim) })
	require.ErrorIs(t, awaitStudioEvent(t, done), context.Canceled)
	require.NoError(t, model.DB.Callback().Query().Remove("cancel-during-claim"))
	persisted, err := model.FindStudioJob(userID, job.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StudioJobQueued, persisted.Status, "a task never submitted upstream can safely remain queued")
	assert.Zero(t, calls.Load())
	quota, err := model.GetUserQuota(userID, false)
	require.NoError(t, err)
	assert.Equal(t, 10_000_000, quota)
}

func TestVideoStudioSuccessIsNotOverwrittenByLateRecovery(t *testing.T) {
	_, _, userID := setupStudioJobsTest(t)
	job := &model.StudioJob{
		ID: "studio_" + common.GetUUID(), UserID: userID, RequestID: common.GetUUID(),
		Kind: "image", Status: model.StudioJobSucceeded, RunnerTask: "previous-owner",
	}
	require.NoError(t, model.DB.Create(job).Error)
	require.NoError(t, service.StoreStudioFile(job.ID, "image-0", strings.NewReader("completed media"), 0))
	stale := *job
	stale.Status, stale.Error = model.StudioJobUncertain, "stale recovery snapshot"
	require.NoError(t, model.SaveStudioJobState(&stale))
	persisted, err := model.FindStudioJob(userID, job.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StudioJobSucceeded, persisted.Status)
	assert.Empty(t, persisted.Error)
	media, err := os.ReadFile(service.VideoStudioJobPath(job.ID, "image-0"))
	require.NoError(t, err)
	assert.Equal(t, "completed media", string(media))
}

func TestVideoStudioVideoSlotDoesNotBlockImageSlots(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	t.Setenv("VIDEO_STUDIO_IMAGE_CONCURRENCY", "2")
	entered := make(chan string, 4)
	release := make(chan struct{})
	var releaseOnce sync.Once
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- r.URL.Path
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/videos" {
			_, _ = w.Write([]byte(`{"id":"offline-video","object":"video","model":"sora-2","status":"queued"}`))
		} else {
			_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
		}
	}))
	t.Cleanup(upstream.Close)
	createStudioJobChannel(t, upstream.URL, "gpt-image-1", constant.ChannelTypeOpenAI)
	createStudioJobChannel(t, upstream.URL, "sora-2", constant.ChannelTypeSora)
	var videoJobs []model.StudioJob
	for i := 0; i < 2; i++ {
		job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/videos",
			`{"model":"sora-2","prompt":"offline video","seconds":"4"}`)
		require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
		videoJobs = append(videoJobs, job)
		require.NoError(t, model.DB.Model(&model.StudioJob{}).Where("id = ?", job.ID).Update("created_at", 1).Error)
	}
	for i := 0; i < 2; i++ {
		_, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/images/generations",
			`{"model":"gpt-image-1","prompt":"offline image","n":1}`)
		require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		done <- runVideoStudioJobs(ctx, "mixed-test")
	}()
	t.Cleanup(func() { cancel(); releaseOnce.Do(func() { close(release) }); awaitStudioEvent(t, exited) })
	var paths []string
	for i := 0; i < 3; i++ {
		paths = append(paths, awaitStudioEvent(t, entered))
	}
	assert.ElementsMatch(t, []string{"/v1/videos", "/v1/images/generations", "/v1/images/generations"}, paths)
	var queued int64
	require.NoError(t, model.DB.Model(&model.StudioJob{}).Where("kind = ? AND status = ?", "video", model.StudioJobQueued).Count(&queued).Error)
	assert.Equal(t, int64(1), queued, "video submissions must retain a single slot")
	releaseOnce.Do(func() { close(release) })
	require.NoError(t, awaitStudioEvent(t, done))
	for _, job := range videoJobs {
		persisted, err := model.FindStudioJob(userID, job.ID)
		require.NoError(t, err)
		assert.Equal(t, model.StudioJobSubmitted, persisted.Status, persisted.Error)
	}
}
