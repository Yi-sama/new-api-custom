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
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeStudioCapacityTest(t *testing.T, path, group, imageModel string, slots int, models ...string) {
	t.Helper()
	data, err := common.Marshal(studioImageCapacity{
		UpdatedAt: time.Now().Unix(), Group: group, Model: imageModel, Models: models, Slots: slots,
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
		fmt.Sprintf(`{"updated_at":%d,"group":"images","model":"gpt-image-2.5-web","models":null,"slots":5}`, time.Now().Unix()),
		fmt.Sprintf(`{"updated_at":%d,"group":"images","model":"gpt-image-2.5-web","models":[],"slots":5}`, time.Now().Unix()),
		fmt.Sprintf(`{"updated_at":%d,"group":"images","model":"gpt-image-2.5-web","models":["gpt-image-2"],"slots":5}`, time.Now().Unix()),
		fmt.Sprintf(`{"updated_at":%d,"group":"images","model":"gpt-image-2.5-web","models":["gpt-image-2.5-web","gpt-image-2.5-web"],"slots":5}`, time.Now().Unix()),
		fmt.Sprintf(`{"updated_at":%d,"group":"images","model":"gpt-image-2.5-web","models":["gpt-image-2.5-web"," "],"slots":5}`, time.Now().Unix()),
	} {
		require.NoError(t, os.WriteFile(file, []byte(body), 0600))
		_, err := readStudioImageCapacity()
		assert.Error(t, err)
	}
	writeStudioCapacityTest(t, file, "images", "gpt-image-1", 0)
	capacity, err := readStudioImageCapacity()
	require.NoError(t, err)
	assert.Zero(t, capacity.Slots, "zero eligible capacity must be representable without submitting work")
	writeStudioCapacityTest(t, file, "images", "gpt-image-2.5-web", 5,
		"gpt-image-2.5-web", "gpt-image-2", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst")
	capacity, err = readStudioImageCapacity()
	require.NoError(t, err)
	assert.Equal(t, []string{"gpt-image-2.5-web", "gpt-image-2", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst"}, capacity.Models)
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
		} else {
			assert.Equal(t, model.StudioJobQueued, persisted.Status)
			var charged int64
			require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("request_id = ? AND type = ?", job.ID, model.LogTypeConsume).Count(&charged).Error)
			assert.Zero(t, charged, "capacity shrink must leave never-started jobs uncharged")
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
	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
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
	require.Error(t, runVideoStudioJobs(context.Background(), "capacity-still-unavailable"))
	assert.Equal(t, 1, calls, "missing capacity must not replay a completed request")
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

func TestVideoStudioAutoCapacitySharesOnePoolAcrossImageModels(t *testing.T) {
	router, token, userID := setupStudioJobsTest(t)
	previousRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousRatios)) })
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"sora-2":1,"gpt-image-1":1,"gpt-image-2":1,"gpt-image-2.5-web":1,"gpt-image-2.5-flare":1,"gpt-image-2.5-sunburst":1}`))
	file := filepath.Join(t.TempDir(), "capacity.json")
	t.Setenv("VIDEO_STUDIO_IMAGE_CONCURRENCY", "auto")
	t.Setenv("VIDEO_STUDIO_IMAGE_CAPACITY_FILE", file)
	writeStudioCapacityTest(t, file, "default", "gpt-image-2.5-web", 5,
		"gpt-image-2.5-web", "gpt-image-2", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst")
	entered := make(chan string, 6)
	release := make(chan struct{}, 6)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct{ Model string }
		if !assert.NoError(t, common.DecodeJson(r.Body, &request)) {
			return
		}
		entered <- request.Model
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
	}))
	t.Cleanup(upstream.Close)
	for _, imageModel := range []string{"gpt-image-2", "gpt-image-2.5-web"} {
		createStudioJobChannel(t, upstream.URL, imageModel, constant.ChannelTypeOpenAI)
	}
	jobs := make([]model.StudioJob, 0, 6)
	for index := 0; index < 6; index++ {
		imageModel := "gpt-image-2"
		if index%2 == 1 {
			imageModel = "gpt-image-2.5-web"
		}
		job, response := submitStudioJobTest(t, router, token, common.GetUUID(), "/jobs/images/generations",
			fmt.Sprintf(`{"model":%q,"prompt":"shared-%d","n":1}`, imageModel, index))
		require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
		jobs = append(jobs, job)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		done <- runVideoStudioJobs(ctx, "shared-model-pool")
	}()
	t.Cleanup(func() { cancel(); close(release); awaitStudioEvent(t, exited) })
	firstFive := make([]string, 0, 5)
	for range 5 {
		firstFive = append(firstFive, awaitStudioEvent(t, entered))
	}
	assert.Contains(t, firstFive, "gpt-image-2")
	assert.Contains(t, firstFive, "gpt-image-2.5-web")
	var queued int64
	require.NoError(t, model.DB.Model(&model.StudioJob{}).Where("status = ?", model.StudioJobQueued).Count(&queued).Error)
	assert.Equal(t, int64(1), queued, "five shared slots must leave the sixth listed model queued")
	release <- struct{}{}
	assert.Contains(t, []string{"gpt-image-2", "gpt-image-2.5-web"}, awaitStudioEvent(t, entered))
	for range 5 {
		release <- struct{}{}
	}
	require.NoError(t, awaitStudioEvent(t, done))
	for _, job := range jobs {
		persisted, err := model.FindStudioJob(userID, job.ID)
		require.NoError(t, err)
		assert.Equal(t, model.StudioJobSucceeded, persisted.Status, persisted.Error)
		var charged int64
		require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("request_id = ? AND type = ?", job.ID, model.LogTypeConsume).Count(&charged).Error)
		assert.Equal(t, int64(1), charged)
	}
}
