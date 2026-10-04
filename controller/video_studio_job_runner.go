package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		job, err := model.ClaimStudioJob(runnerTask)
		if err != nil || job == nil {
			return err
		}
		executeStudioJob(ctx, job)
		if err := model.SaveStudioJobState(job); err != nil {
			return err
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
		c.Next()
	}, middleware.Distribute(), func(c *gin.Context) {
		if job.Kind == "video" {
			VideoStudioRelayTask(c)
		} else {
			VideoStudioRelay(c, types.RelayFormatOpenAIImage)
		}
	})
	router.ServeHTTP(writer, request)
	if err := output.Sync(); err != nil {
		writer.err = err
	}
	if writer.err != nil || writer.status < 200 || writer.status >= 300 {
		job.Error = "generation request was rejected"
		if attempted {
			job.Status, job.Error = model.StudioJobUncertain, "upstream submission could not be confirmed; do not automatically resubmit"
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
