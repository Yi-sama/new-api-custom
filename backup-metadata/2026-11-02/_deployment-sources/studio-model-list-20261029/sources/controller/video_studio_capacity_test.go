package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeStudioCapacityTest(t *testing.T, path, group, imageModel string, slots int) {
	t.Helper()
	data, err := common.Marshal(studioImageCapacity{
		UpdatedAt: time.Now().Unix(), Group: group, Model: imageModel, Slots: slots,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path+".next", data, 0600))
	require.NoError(t, os.Rename(path+".next", path))
}

func TestVideoStudioCapacityValidation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "capacity.json")
	t.Setenv("VIDEO_STUDIO_IMAGE_CAPACITY_FILE", file)
	t.Setenv("VIDEO_STUDIO_IMAGE_CONCURRENCY", "auto")
	_, err := readStudioImageCapacity()
	require.Error(t, err)
	for _, body := range []string{
		`{`,
		`{"updated_at":1,"group":"images","model":"gpt-image-1","slots":24}`,
		fmt.Sprintf(`{"updated_at":%d,"slots":24}`, time.Now().Unix()),
		fmt.Sprintf(`{"updated_at":%d,"group":"images","model":"gpt-image-1","slots":-1}`, time.Now().Unix()),
		fmt.Sprintf(`{"updated_at":%d,"group":"images","model":"gpt-image-1","slots":24}`, time.Now().Unix()+3600),
	} {
		require.NoError(t, os.WriteFile(file, []byte(body), 0600))
		_, err := readStudioImageCapacity()
		assert.Error(t, err)
	}
	writeStudioCapacityTest(t, file, "images", "gpt-image-1", 0)
	capacity, err := readStudioImageCapacity()
	require.NoError(t, err)
	assert.Zero(t, capacity.Slots, "zero eligible capacity must be representable without submitting work")
	t.Setenv("VIDEO_STUDIO_IMAGE_CONCURRENCY", "24")
	capacity, err = readStudioImageCapacity()
	require.NoError(t, err)
	assert.Equal(t, 24, capacity.Slots, "operator limits must not retain the old eight-slot ceiling")
	for _, value := range []string{"0", "-1", "invalid"} {
		t.Setenv("VIDEO_STUDIO_IMAGE_CONCURRENCY", value)
		_, err := readStudioImageCapacity()
		assert.Error(t, err)
	}
}

func TestVideoStudioPublicationPauseLeavesNewTasksUnclaimed(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	createStudioJobChannel(t, "http://127.0.0.1:1", "gpt-image-1", constant.ChannelTypeOpenAI)
	pause := filepath.Join(t.TempDir(), "paused")
	t.Setenv("VIDEO_STUDIO_PAUSE_FILE", pause)
	require.NoError(t, os.WriteFile(pause, nil, 0600))
	job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/images/generations",
		`{"model":"gpt-image-1","prompt":"publication paused","n":1}`)
	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	require.NoError(t, runVideoStudioJobs(context.Background(), "paused-publication"))
	persisted, err := model.FindStudioJob(userID, job.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StudioJobQueued, persisted.Status)
	quota, err := model.GetUserQuota(userID, false)
	require.NoError(t, err)
	assert.Equal(t, 10_000_000, quota)
}

func TestVideoStudioCapacityGrowsAboveEightAndShrinksWithoutReplay(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	file := filepath.Join(t.TempDir(), "capacity.json")
	t.Setenv("VIDEO_STUDIO_IMAGE_CONCURRENCY", "auto")
	t.Setenv("VIDEO_STUDIO_IMAGE_CAPACITY_FILE", file)
	writeStudioCapacityTest(t, file, "default", "gpt-image-1", 2)
	entered := make(chan struct{}, 15)
	release := make(chan struct{}, 15)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
	}))
	t.Cleanup(upstream.Close)
	createStudioJobChannel(t, upstream.URL, "gpt-image-1", constant.ChannelTypeOpenAI)
	var jobs []model.StudioJob
	for i := 0; i < 15; i++ {
		job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/images/generations",
			`{"model":"gpt-image-1","prompt":"dynamic capacity offline fixture","n":1}`)
		require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
		jobs = append(jobs, job)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		done <- runVideoStudioJobs(ctx, "capacity-growth")
	}()
	t.Cleanup(func() { cancel(); close(release); awaitStudioEvent(t, exited) })
	awaitStudioEvent(t, entered)
	awaitStudioEvent(t, entered)
	var queued int64
	require.NoError(t, model.DB.Model(&model.StudioJob{}).Where("status = ?", model.StudioJobQueued).Count(&queued).Error)
	assert.Equal(t, int64(13), queued)
	writeStudioCapacityTest(t, file, "default", "gpt-image-1", 12)
	// No upstream completion is needed to discover newly available capacity.
	for i := 0; i < 10; i++ {
		awaitStudioEvent(t, entered)
	}
	require.NoError(t, model.DB.Model(&model.StudioJob{}).Where("status = ?", model.StudioJobQueued).Count(&queued).Error)
	assert.Equal(t, int64(3), queued)
	writeStudioCapacityTest(t, file, "default", "gpt-image-1", 0)
	for i := 0; i < 12; i++ {
		release <- struct{}{}
	}
	require.NoError(t, awaitStudioEvent(t, done))
	assert.Empty(t, entered, "capacity shrink must stop new submissions, not cancel current work")
	require.NoError(t, model.DB.Model(&model.StudioJob{}).Where("status = ?", model.StudioJobQueued).Count(&queued).Error)
	assert.Equal(t, int64(3), queued)
	for _, job := range jobs {
		persisted, err := model.FindStudioJob(userID, job.ID)
		require.NoError(t, err)
		if persisted.Status == model.StudioJobSucceeded {
			var charged int64
			require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("request_id = ? AND type = ?", job.ID, model.LogTypeConsume).Count(&charged).Error)
			assert.Equal(t, int64(1), charged)
		}
	}
	writeStudioCapacityTest(t, file, "default", "gpt-image-1", 24)
	for i := 0; i < 3; i++ {
		release <- struct{}{}
	}
	require.NoError(t, runVideoStudioJobs(context.Background(), "capacity-restored"))
	assert.Len(t, entered, 3, "restoring capacity must submit only the never-started jobs")
}

func TestVideoStudioAutoCapacityIsScopedAndUnavailableCapacityKeepsImagesQueued(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	file := filepath.Join(t.TempDir(), "capacity.json")
	t.Setenv("VIDEO_STUDIO_IMAGE_CONCURRENCY", "auto")
	t.Setenv("VIDEO_STUDIO_IMAGE_CAPACITY_FILE", file)
	writeStudioCapacityTest(t, file, "different-group", "gpt-image-1", 24)
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var once sync.Once
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
	}))
	t.Cleanup(upstream.Close)
	createStudioJobChannel(t, upstream.URL, "gpt-image-1", constant.ChannelTypeOpenAI)
	var jobs []model.StudioJob
	for i := 0; i < 2; i++ {
		job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/images/generations",
			`{"model":"gpt-image-1","prompt":"other route","n":1}`)
		require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
		jobs = append(jobs, job)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		done <- runVideoStudioJobs(ctx, "capacity-scope")
	}()
	t.Cleanup(func() { cancel(); once.Do(func() { close(release) }); awaitStudioEvent(t, exited) })
	awaitStudioEvent(t, entered)
	var queued int64
	require.NoError(t, model.DB.Model(&model.StudioJob{}).Where("status = ?", model.StudioJobQueued).Count(&queued).Error)
	assert.Equal(t, int64(1), queued, "another route must not borrow this account pool's 24 slots")
	require.NoError(t, os.Remove(file))
	once.Do(func() { close(release) })
	require.Error(t, awaitStudioEvent(t, done), "missing capacity must pause image admission")
	assert.Empty(t, entered)
	for _, job := range jobs {
		persisted, err := model.FindStudioJob(userID, job.ID)
		require.NoError(t, err)
		if persisted.Status == model.StudioJobQueued {
			var charged int64
			require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("request_id = ? AND type = ?", job.ID, model.LogTypeConsume).Count(&charged).Error)
			assert.Zero(t, charged, "paused jobs must not pre-consume quota")
		}
	}
}
