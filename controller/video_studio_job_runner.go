package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type videoStudioJobsHandler struct{}

func (videoStudioJobsHandler) Type() string { return model.SystemTaskTypeVideoStudio }
func (videoStudioJobsHandler) Enabled() bool {
	return VideoStudioEnabled() && model.HasPendingStudioJobs()
}
func (videoStudioJobsHandler) Interval() time.Duration { return 15 * time.Second }
func (videoStudioJobsHandler) NewPayload() any         { return nil }

func (videoStudioJobsHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	err := runVideoStudioJobs(ctx, task.TaskID)
	status := model.SystemTaskStatusSucceeded
	if err != nil {
		status = model.SystemTaskStatusFailed
	}
	finishSystemTaskHandler(task, runnerID, status, nil, err)
}

func runVideoStudioJobs(ctx context.Context, runnerTask string) error {
	if path := os.Getenv("VIDEO_STUDIO_PAUSE_FILE"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	jobs, err := model.UnfinishedStudioJobs()
	if err != nil {
		return err
	}
	// The existing per-type lease guarantees that a new run cannot resubmit a
	// still-running job. An abandoned upstream request is never executed again.
	for i := range jobs {
		job := &jobs[i]
		if err := ctx.Err(); err != nil {
			return err
		}
		if job.Kind == "video" && job.TaskID != "" {
			task, exists, err := model.GetByTaskId(job.UserID, job.TaskID)
			if err != nil {
				return err
			}
			if exists {
				job.Status = model.StudioJobSubmitted
				if err := syncStudioVideoResult(ctx, job, task); err != nil {
					// A media download can be retried without submitting or charging.
					job.Error = "video generated; result storage is pending"
					common.SysError("studio video result " + job.ID + ": " + err.Error())
				}
			} else {
				job.Status, job.Error = model.StudioJobUncertain, "submission was interrupted; do not submit it again until checked"
			}
		} else if job.Kind == "image" {
			_, completedErr := os.Stat(service.VideoStudioJobPath(job.ID, "relay-complete"))
			response, responseErr := os.Open(service.VideoStudioJobPath(job.ID, "response.bin"))
			// A complete captured response can be recovered even if the process
			// stopped before writing its completion marker. Never run the relay.
			if responseErr == nil {
				job.Status, job.Error = model.StudioJobSubmitted, "image generated; result storage is pending"
				err := storeStudioImageResult(job, response)
				response.Close()
				if err == nil {
					job.Status, job.Error = model.StudioJobSucceeded, ""
				} else {
					common.SysError("studio image result " + job.ID + ": " + err.Error())
					if completedErr != nil {
						job.Status, job.Error = model.StudioJobUncertain, "submission was interrupted; do not submit it again until checked"
					}
				}
			} else {
				job.Status, job.Error = model.StudioJobUncertain, "submission was interrupted; do not submit it again until checked"
			}
		}
		if err := model.SaveStudioJobState(job); err != nil {
			return err
		}
	}
	return runQueuedStudioJobs(ctx, runnerTask)
}

type studioImageCapacity struct {
	UpdatedAt        int64    `json:"updated_at"`
	Group            string   `json:"group"`
	Model            string   `json:"model"`
	Models           []string `json:"models,omitempty"`
	Slots            int      `json:"slots"`
	modelsConfigured bool
}

func (capacity *studioImageCapacity) UnmarshalJSON(data []byte) error {
	var decoded struct {
		UpdatedAt int64    `json:"updated_at"`
		Group     string   `json:"group"`
		Model     string   `json:"model"`
		Models    []string `json:"models"`
		Slots     int      `json:"slots"`
	}
	if err := common.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := common.Unmarshal(data, &fields); err != nil {
		return err
	}
	_, modelsConfigured := fields["models"]
	*capacity = studioImageCapacity{
		UpdatedAt: decoded.UpdatedAt, Group: decoded.Group, Model: decoded.Model,
		Models: decoded.Models, Slots: decoded.Slots, modelsConfigured: modelsConfigured,
	}
	return nil
}

func readStudioImageCapacity() (studioImageCapacity, error) {
	value := os.Getenv("VIDEO_STUDIO_IMAGE_CONCURRENCY")
	if value != "auto" {
		if value == "" {
			value = "1"
		}
		slots, err := strconv.Atoi(value)
		if err != nil || slots < 1 {
			return studioImageCapacity{}, errors.New("VIDEO_STUDIO_IMAGE_CONCURRENCY must be auto or a positive integer")
		}
		return studioImageCapacity{Slots: slots}, nil
	}
	file, err := os.Open(os.Getenv("VIDEO_STUDIO_IMAGE_CAPACITY_FILE"))
	if err != nil {
		return studioImageCapacity{}, fmt.Errorf("studio image capacity unavailable: %w", err)
	}
	defer file.Close()
	var capacity studioImageCapacity
	if err := common.DecodeJson(io.LimitReader(file, 16384), &capacity); err != nil {
		return studioImageCapacity{}, fmt.Errorf("invalid studio image capacity: %w", err)
	}
	now := time.Now().Unix()
	if capacity.Group == "" || capacity.Model == "" || capacity.Slots < 0 ||
		capacity.UpdatedAt < now-60 || capacity.UpdatedAt > now+5 {
		return studioImageCapacity{}, errors.New("studio image capacity is invalid or stale; new images remain queued")
	}
	if capacity.modelsConfigured {
		if len(capacity.Models) == 0 {
			return studioImageCapacity{}, errors.New("studio image capacity models must be a nonempty array")
		}
		models := make(map[string]struct{}, len(capacity.Models))
		for _, imageModel := range capacity.Models {
			if imageModel == "" || imageModel != strings.TrimSpace(imageModel) {
				return studioImageCapacity{}, errors.New("studio image capacity models must not contain blank entries")
			}
			if _, exists := models[imageModel]; exists {
				return studioImageCapacity{}, errors.New("studio image capacity models must not contain duplicates")
			}
			models[imageModel] = struct{}{}
		}
		if _, exists := models[capacity.Model]; !exists {
			return studioImageCapacity{}, errors.New("studio image capacity models must include model")
		}
	}
	return capacity, nil
}

func runQueuedStudioJobs(ctx context.Context, runnerTask string) (runErr error) {
	completed := make(chan *model.StudioJob)
	active := make(map[string]model.StudioJobFilter)
	refresh := time.NewTicker(5 * time.Second)
	defer refresh.Stop()
	inflight := 0
	// Do not finish the system task or release its lease until every request
	// has stopped and its final state has been saved, even on cancellation.
	defer func() {
		for inflight > 0 {
			job := <-completed
			inflight--
			if err := model.SaveStudioJobState(job); err != nil {
				runErr = errors.Join(runErr, err)
			}
		}
	}()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if path := os.Getenv("VIDEO_STUDIO_PAUSE_FILE"); path != "" {
			if _, err := os.Stat(path); err == nil {
				return nil
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		// Save completed work before claiming more, so a storage failure stops
		// further submissions while the deferred drain preserves in-flight work.
	drain:
		for {
			select {
			case job := <-completed:
				inflight--
				delete(active, job.ID)
				if err := model.SaveStudioJobState(job); err != nil {
					return err
				}
			default:
				break drain
			}
		}
		capacity, capacityErr := readStudioImageCapacity()
		type studioLane struct {
			filter model.StudioJobFilter
			slots  int
		}
		limits := []studioLane{{filter: model.StudioJobFilter{Kind: "video"}, slots: 1}}
		if capacityErr == nil {
			imageRoute := model.StudioJobFilter{Kind: "image", Group: capacity.Group, Model: capacity.Model, Models: capacity.Models}
			limits = append(limits, studioLane{imageRoute, capacity.Slots})
			if capacity.Group != "" {
				imageRoute.ExcludeRoute = true
				limits = append(limits, studioLane{imageRoute, 1})
			}
		}
		for _, limit := range limits {
			count := 0
			for _, route := range active {
				if limit.filter.Matches(route) {
					count++
				}
			}
			for count < limit.slots {
				if err := ctx.Err(); err != nil {
					return err
				}
				job, err := model.ClaimStudioJob(runnerTask, limit.filter)
				if err != nil {
					return err
				}
				if job == nil {
					break
				}
				if err := ctx.Err(); err != nil {
					job.Status, job.Error = model.StudioJobQueued, ""
					return errors.Join(err, model.SaveStudioJobState(job))
				}
				active[job.ID] = model.StudioJobFilter{Kind: job.Kind, Group: job.Group, Model: job.Model}
				count++
				inflight++
				go func() {
					defer func() {
						if recovered := recover(); recovered != nil {
							job.Status, job.Error = model.StudioJobUncertain, "generation was interrupted; do not automatically resubmit"
							common.SysError(fmt.Sprintf("studio job %s interrupted: %v", job.ID, recovered))
						}
						completed <- job
					}()
					executeStudioJob(ctx, job)
				}()
			}
		}
		if inflight == 0 {
			return capacityErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-refresh.C:
		case job := <-completed:
			inflight--
			delete(active, job.ID)
			if err := model.SaveStudioJobState(job); err != nil {
				return err
			}
		}
	}
}

// studioResponseFile captures relay output without a browser connection or an
// in-memory video buffer. Only authenticated job endpoints expose final media.
type studioResponseFile struct {
	file   *os.File
	header http.Header
	status int
	err    error
	limit  int64
	size   int64
}

func (w *studioResponseFile) Header() http.Header { return w.header }
func (w *studioResponseFile) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *studioResponseFile) Write(p []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	if w.limit > 0 && int64(len(p)) > w.limit-w.size {
		w.err = errors.New("video exceeds the configured download limit")
		return 0, w.err
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	if err != nil {
		w.err = err
	}
	return n, err
}
func (w *studioResponseFile) Flush() {}

func executeStudioJob(ctx context.Context, job *model.StudioJob) {
	if ctx.Err() != nil {
		job.Status, job.Error = model.StudioJobQueued, ""
		return
	}
	job.Status, job.Error = model.StudioJobFailed, "generation could not be started"
	input, err := os.Open(service.VideoStudioJobPath(job.ID, "request.bin"))
	if err != nil {
		common.SysError("studio input " + job.ID + ": " + err.Error())
		return
	}
	defer input.Close()
	output, err := os.OpenFile(service.VideoStudioJobPath(job.ID, "response.bin"), os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		common.SysError("studio output " + job.ID + ": " + err.Error())
		return
	}
	defer output.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/"+job.Operation, input)
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", job.ContentType)
	if stat, err := input.Stat(); err == nil {
		request.ContentLength = stat.Size()
	}
	writer := &studioResponseFile{file: output, header: make(http.Header), limit: int64(constant.MaxFileDownloadMB) * 1024 * 1024}
	attempted := false
	router := gin.New()
	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		common.SysError(fmt.Sprintf("studio job %s interrupted: %v", job.ID, recovered))
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
	router.POST("/v1/"+job.Operation, func(c *gin.Context) {
		defer common.CleanupBodyStorage(c)
		defer service.CleanupFileSources(c)
		defer func() {
			if c.Request.MultipartForm != nil {
				_ = c.Request.MultipartForm.RemoveAll()
			}
		}()
		defer func() { attempted = common.GetContextKeyBool(c, constant.ContextKeyUpstreamRequestAttempted) }()
		user, err := model.GetUserCache(job.UserID)
		if err != nil || user.Status != common.UserStatusEnabled || !service.IsUserSelectableGroup(user.Group, job.Group) {
			studioJobError(c, http.StatusForbidden, "user or group is no longer available")
			return
		}
		user.WriteContext(c)
		c.Set("id", job.UserID)
		if err := middleware.SetupContextForToken(c, &model.Token{UserId: job.UserID, Name: "video-studio", Group: job.Group}); err != nil {
			studioJobError(c, http.StatusForbidden, "could not authorize the task")
			return
		}
		common.SetContextKey(c, constant.ContextKeyUsingGroup, job.Group)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, job.Group)
		common.SetContextKey(c, constant.ContextKeyStudioJobID, job.ID)
		common.SetContextKey(c, constant.ContextKeyStudioNativeTaskID, job.TaskID)
		c.Set(common.RequestIdKey, job.ID)
		c.Set("studio_operation", job.Operation)
		if _, _, err := readStudioJobRequest(c, job.Kind); err != nil {
			studioJobError(c, http.StatusBadRequest, err.Error())
			return
		}
		if err := validateStudioJobModel(user.Group, job.Group, job.Model, job.Kind); err != nil {
			studioJobError(c, http.StatusBadRequest, err.Error())
			return
		}
		if ctx.Err() != nil {
			studioJobError(c, http.StatusServiceUnavailable, "task runner stopped before submission")
			return
		}
		c.Next()
	}, middleware.Distribute(), func(c *gin.Context) {
		if job.Kind == "video" {
			VideoStudioRelayTask(c)
		} else {
			VideoStudioRelay(c, types.RelayFormatOpenAIImage)
		}
	})
	router.ServeHTTP(writer, request)
	if !attempted && ctx.Err() != nil {
		job.Status, job.Error = model.StudioJobQueued, ""
		return
	}
	if err := output.Sync(); err != nil {
		writer.err = err
	}
	if writer.err != nil || writer.status < 200 || writer.status >= 300 {
		job.Error = "generation request was rejected"
		if attempted {
			job.Status, job.Error = model.StudioJobUncertain, "upstream submission could not be confirmed; do not automatically resubmit"
			if job.Kind == "image" && writer.err == nil && writer.status == http.StatusBadRequest {
				var rejection struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if _, err := output.Seek(0, io.SeekStart); err == nil &&
					common.DecodeJson(io.LimitReader(output, 16384), &rejection) == nil &&
					rejection.Error.Code == "content_policy_violation" {
					job.Status, job.Error = model.StudioJobFailed, "image generation was rejected by upstream content policy"
				}
			}
		}
		return
	}
	if job.Kind == "video" {
		_, exists, err := model.GetByTaskId(job.UserID, job.TaskID)
		if err != nil || !exists {
			job.Status, job.Error = model.StudioJobUncertain, "video submission needs verification"
			return
		}
		job.Status, job.Error = model.StudioJobSubmitted, ""
		return
	}
	if err = service.StoreStudioFile(job.ID, "relay-complete", strings.NewReader("complete"), 0); err != nil {
		job.Status, job.Error = model.StudioJobUncertain, "image response received but result storage needs verification"
		return
	}
	if _, err = output.Seek(0, io.SeekStart); err == nil {
		err = storeStudioImageResult(job, output)
	}
	if err != nil {
		common.SysError("studio image result " + job.ID + ": " + err.Error())
		job.Status, job.Error = model.StudioJobSubmitted, "image generated; result storage is pending"
		return
	}
	job.Status, job.Error = model.StudioJobSucceeded, ""
}

func storeStudioImageResult(job *model.StudioJob, source io.Reader) error {
	var response struct {
		Data []struct {
			URL           string `json:"url"`
			Base64        string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt,omitempty"`
		} `json:"data"`
	}
	if err := common.DecodeJson(source, &response); err != nil {
		return err
	}
	if len(response.Data) == 0 || len(response.Data) > dto.MaxImageN {
		return errors.New("invalid image count in generation response")
	}
	limit := int64(constant.MaxFileDownloadMB) * 1024 * 1024
	for i := range response.Data {
		item := &response.Data[i]
		name := fmt.Sprintf("image-%d", i)
		if item.Base64 != "" {
			if err := service.StoreStudioFile(job.ID, name, base64.NewDecoder(base64.StdEncoding, strings.NewReader(item.Base64)), limit); err != nil {
				return err
			}
		} else if strings.HasPrefix(item.URL, "data:image/") {
			parts := strings.SplitN(item.URL, ",", 2)
			if len(parts) != 2 || !strings.HasSuffix(parts[0], ";base64") {
				return errors.New("unsupported image data URL")
			}
			if err := service.StoreStudioFile(job.ID, name, base64.NewDecoder(base64.StdEncoding, strings.NewReader(parts[1])), limit); err != nil {
				return err
			}
		} else {
			response, err := service.DoDownloadRequest(item.URL, "video studio image result")
			if err != nil {
				return err
			}
			if response.StatusCode != http.StatusOK {
				response.Body.Close()
				return fmt.Errorf("image download returned %d", response.StatusCode)
			}
			err = service.StoreStudioFile(job.ID, name, response.Body, limit)
			response.Body.Close()
			if err != nil {
				return err
			}
		}
		item.URL = videoStudioPrefix + "/jobs/" + job.ID + "/assets/" + fmt.Sprint(i)
		item.Base64 = ""
	}
	data, err := common.Marshal(response)
	if err != nil {
		return err
	}
	return service.StoreStudioFile(job.ID, "result.json", bytes.NewReader(data), 0)
}

func syncStudioVideoResult(ctx context.Context, job *model.StudioJob, task *model.Task) error {
	if task.Status == model.TaskStatusFailure {
		job.Status, job.Error = model.StudioJobFailed, "video generation failed"
		return nil
	}
	if task.Status != model.TaskStatusSuccess {
		return nil
	}
	file, err := os.CreateTemp(service.VideoStudioJobPath(job.ID, ""), ".video-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	writer := &studioResponseFile{file: file, header: make(http.Header), limit: int64(constant.MaxFileDownloadMB) * 1024 * 1024}
	router := gin.New()
	router.GET("/videos/:task_id/content", func(c *gin.Context) {
		c.Set("id", job.UserID)
		VideoProxy(c)
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "/videos/"+job.TaskID+"/content", nil)
	if err != nil {
		return err
	}
	router.ServeHTTP(writer, request)
	if writer.err != nil {
		return writer.err
	}
	if writer.status != http.StatusOK {
		return fmt.Errorf("video content returned %d", writer.status)
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return errors.New("empty video result")
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), service.VideoStudioJobPath(job.ID, "video")); err != nil {
		return err
	}
	job.Status, job.Error = model.StudioJobSucceeded, ""
	return nil
}
