package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func studioJobError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"success": false, "message": message})
}

// readStudioJobRequest also canonicalizes multipart files for idempotency:
// a browser's new multipart boundary is not a new billable operation.
func readStudioJobRequest(c *gin.Context, kind string) (map[string]any, string, error) {
	fields := make(map[string]any)
	canonical := make(map[string]any)
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		form, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return nil, "", err
		}
		defer form.RemoveAll()
		for key, values := range form.Value {
			if len(values) != 1 {
				return nil, "", fmt.Errorf("duplicate field %q", key)
			}
			fields[key] = values[0]
		}
		canonical["fields"] = form.Value
		files := make(map[string][]string)
		for key, headers := range form.File {
			for _, header := range headers {
				file, err := header.Open()
				if err != nil {
					return nil, "", err
				}
				digest := sha256.New()
				_, err = io.Copy(digest, file)
				file.Close()
				if err != nil {
					return nil, "", err
				}
				files[key] = append(files[key], header.Filename+":"+header.Header.Get("Content-Type")+":"+hex.EncodeToString(digest.Sum(nil)))
			}
		}
		canonical["files"] = files
	} else if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		if err := common.UnmarshalBodyReusable(c, &fields); err != nil {
			return nil, "", err
		}
		canonical["fields"] = fields
	} else {
		return nil, "", errors.New("expected JSON or multipart form data")
	}
	if fields["stream"] == true || fields["stream"] == "true" {
		return nil, "", errors.New("studio jobs require a non-streaming result")
	}
	if group, ok := fields["group"]; ok && group != common.GetContextKeyString(c, constant.ContextKeyTokenGroup) {
		return nil, "", errors.New("body group must match the selected group")
	}
	modelName, _ := fields["model"].(string)
	if modelName == "" {
		return nil, "", errors.New("model is required")
	}
	// Providers may read these same parameters from metadata instead of the
	// standard fields. Validate both representations, including form JSON.
	effective := make(map[string]any, len(fields))
	for key, value := range fields {
		effective[key] = value
	}
	for _, key := range []string{"metadata", "parameters"} {
		if raw, exists := fields[key]; exists {
			var nested map[string]any
			if text, ok := raw.(string); ok {
				if err := common.UnmarshalJsonStr(text, &nested); err != nil {
					return nil, "", fmt.Errorf("invalid %s", key)
				}
			} else if object, ok := raw.(map[string]any); ok {
				nested = object
			} else {
				return nil, "", fmt.Errorf("%s must be an object", key)
			}
			for name, value := range nested {
				if current, exists := effective[name]; exists && fmt.Sprint(current) != fmt.Sprint(value) {
					return nil, "", fmt.Errorf("conflicting %s parameter", name)
				}
				effective[name] = value
			}
		}
	}
	for alias, name := range map[string]string{
		"resolution_name": "resolution", "aspectRatio": "aspect_ratio", "ratio": "aspect_ratio",
		"durationSeconds": "duration", "generateAudio": "generate_audio",
	} {
		if value, exists := effective[alias]; exists {
			if current, exists := effective[name]; exists && fmt.Sprint(current) != fmt.Sprint(value) {
				return nil, "", fmt.Errorf("conflicting %s parameter", name)
			}
			effective[name] = value
		}
	}
	values := service.VideoStudioParameterValues{}
	for key, target := range map[string]*string{
		"size": &values.Size, "aspect_ratio": &values.AspectRatio, "quality": &values.Quality,
		"resolution": &values.Resolution, "mode": &values.Mode,
	} {
		if value, exists := effective[key]; exists {
			text, valid := value.(string)
			if !valid {
				return nil, "", fmt.Errorf("%s must be a string", key)
			}
			*target = text
		}
	}
	for key, target := range map[string]**int{"n": &values.Count, "seconds": &values.Duration, "duration": &values.Duration, "fps": &values.FPS} {
		if value, exists := effective[key]; exists {
			number, err := strconv.Atoi(fmt.Sprint(value))
			if err != nil || number <= 0 {
				return nil, "", fmt.Errorf("%s must be a positive integer", key)
			}
			if *target != nil && **target != number {
				return nil, "", errors.New("conflicting duration fields")
			}
			*target = &number
		}
	}
	if values.Count != nil && *values.Count > dto.MaxImageN {
		return nil, "", fmt.Errorf("n exceeds %d", dto.MaxImageN)
	}
	if values.Duration != nil && *values.Duration > relaycommon.MaxTaskDurationSeconds {
		return nil, "", fmt.Errorf("duration exceeds %d", relaycommon.MaxTaskDurationSeconds)
	}
	for key, target := range map[string]**bool{"generate_audio": &values.GenerateAudio, "watermark": &values.Watermark} {
		if value, exists := effective[key]; exists {
			boolean, err := strconv.ParseBool(fmt.Sprint(value))
			if err != nil {
				return nil, "", fmt.Errorf("%s must be a boolean", key)
			}
			*target = &boolean
		}
	}
	if err := service.ValidateVideoStudioParameters(common.GetContextKeyString(c, constant.ContextKeyTokenGroup), modelName, kind, values); err != nil {
		return nil, "", err
	}
	canonical["group"] = common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	canonical["operation"] = c.GetString("studio_operation")
	data, err := common.Marshal(canonical)
	if err != nil {
		return nil, "", err
	}
	hash := sha256.Sum256(data)
	return fields, hex.EncodeToString(hash[:]), nil
}

func validateStudioJobModel(userGroup, group, name, kind string) error {
	catalog, err := service.GetVideoStudioCatalog(userGroup)
	if err != nil {
		return err
	}
	for _, entry := range catalog {
		if entry.ID != group {
			continue
		}
		for _, candidate := range entry.Models {
			if candidate.ID == name {
				for _, capability := range candidate.Capabilities {
					if capability == kind {
						return nil
					}
				}
			}
		}
	}
	return errors.New("model is not available for this generation type and group")
}

func VideoStudioParameterAuthorization(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, _, err := readStudioJobRequest(c, kind); err != nil {
			studioJobError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.Next()
	}
}

func CreateVideoStudioJob(kind, operation string) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer common.CleanupBodyStorage(c)
		requestID, err := uuid.Parse(c.GetHeader("Idempotency-Key"))
		if err != nil {
			studioJobError(c, http.StatusBadRequest, "Idempotency-Key must be a UUID")
			return
		}
		c.Set("studio_operation", operation)
		fields, fingerprint, err := readStudioJobRequest(c, kind)
		if err != nil {
			status := http.StatusBadRequest
			if common.IsRequestBodyTooLargeError(err) {
				status = http.StatusRequestEntityTooLarge
			}
			studioJobError(c, status, err.Error())
			return
		}
		userID := c.GetInt("id")
		existing, err := model.FindStudioJobRequest(userID, requestID.String())
		if err == nil {
			if existing.Fingerprint != fingerprint {
				studioJobError(c, http.StatusConflict, "this request ID already belongs to a different submission")
				return
			}
			common.ApiSuccess(c, existing)
			return
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			studioJobError(c, http.StatusInternalServerError, "could not look up the submission")
			return
		}
		group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		modelName := fields["model"].(string)
		if err := validateStudioJobModel(common.GetContextKeyString(c, constant.ContextKeyUserGroup), group, modelName, kind); err != nil {
			studioJobError(c, http.StatusBadRequest, err.Error())
			return
		}
		parameters := make(map[string]any)
		for _, key := range []string{"n", "size", "quality", "aspect_ratio", "seconds", "duration", "resolution", "mode", "generate_audio", "watermark", "fps"} {
			if value, ok := fields[key]; ok {
				parameters[key] = value
			}
		}
		parameterJSON, err := common.Marshal(parameters)
		if err != nil {
			studioJobError(c, http.StatusBadRequest, "invalid parameters")
			return
		}
		prompt, _ := fields["prompt"].(string)
		now := common.GetTimestamp()
		job := &model.StudioJob{
			ID: "studio_" + common.GetUUID(), UserID: userID, RequestID: requestID.String(),
			Kind: kind, Operation: operation, Group: group, Model: modelName, Prompt: prompt,
			Parameters: parameterJSON, Status: model.StudioJobQueued, CreatedAt: now, UpdatedAt: now,
			ContentType: c.GetHeader("Content-Type"), Fingerprint: fingerprint,
		}
		if kind == "video" {
			job.TaskID = model.GenerateTaskID()
		}
		body, err := common.GetBodyStorage(c)
		if err == nil {
			err = service.StoreStudioFile(job.ID, "request.bin", body, 0)
		}
		if err != nil {
			studioJobError(c, http.StatusInternalServerError, "could not persist the submission")
			return
		}
		stored, created, err := model.CreateStudioJob(job)
		if err != nil || !created {
			_ = os.Remove(service.VideoStudioJobPath(job.ID, "request.bin"))
		}
		if err != nil {
			studioJobError(c, http.StatusInternalServerError, "could not persist the submission")
			return
		}
		if stored.Fingerprint != fingerprint {
			studioJobError(c, http.StatusConflict, "this request ID already belongs to a different submission")
			return
		}
		if _, _, err := service.EnqueueSystemTask(model.SystemTaskTypeVideoStudio, nil); err != nil {
			// The durable job remains queued; the existing scheduler picks it up.
			common.SysError("wake studio jobs: " + err.Error())
		}
		c.JSON(http.StatusAccepted, gin.H{"success": true, "data": stored})
	}
}

func ListVideoStudioJobs(c *gin.Context) {
	if requestID := c.Query("request_id"); requestID != "" {
		id, err := uuid.Parse(requestID)
		if err != nil {
			studioJobError(c, http.StatusBadRequest, "invalid request ID")
			return
		}
		job, err := model.FindStudioJobRequest(c.GetInt("id"), id.String())
		jobs := make([]*model.StudioJob, 0)
		if err == nil {
			jobs = append(jobs, job)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			studioJobError(c, http.StatusInternalServerError, "could not load the submission")
			return
		}
		common.ApiSuccess(c, gin.H{"items": jobs, "total": len(jobs)})
		return
	}
	kind := c.Query("kind")
	if kind != "" && kind != "image" && kind != "video" {
		studioJobError(c, http.StatusBadRequest, "invalid job kind")
		return
	}
	page := common.GetPageQuery(c)
	if page.Page < 1 || page.PageSize < 1 || page.GetStartIdx() < 0 {
		studioJobError(c, http.StatusBadRequest, "invalid page")
		return
	}
	jobs, total, err := model.ListStudioJobs(c.GetInt("id"), kind, page)
	if err != nil {
		studioJobError(c, http.StatusInternalServerError, "could not load jobs")
		return
	}
	common.ApiSuccess(c, gin.H{"items": jobs, "total": total})
}

func GetVideoStudioJob(c *gin.Context) {
	job, err := model.FindStudioJob(c.GetInt("id"), c.Param("id"))
	if err != nil {
		studioJobError(c, http.StatusNotFound, "job not found")
		return
	}
	common.ApiSuccess(c, job)
}

func GetVideoStudioJobResult(c *gin.Context) {
	job, err := model.FindStudioJob(c.GetInt("id"), c.Param("id"))
	if err != nil {
		studioJobError(c, http.StatusNotFound, "job not found")
		return
	}
	if job.Status != model.StudioJobSucceeded {
		studioJobError(c, http.StatusConflict, "result is not ready")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	if asset := c.Param("asset"); asset != "" {
		index, err := strconv.Atoi(asset)
		if err != nil || index < 0 || index >= dto.MaxImageN || job.Kind != "image" {
			c.Status(http.StatusNotFound)
			return
		}
		c.File(service.VideoStudioJobPath(job.ID, fmt.Sprintf("image-%d", index)))
		return
	}
	if job.Kind == "video" {
		c.Header("Content-Type", "video/mp4")
		c.File(service.VideoStudioJobPath(job.ID, "video"))
		return
	}
	c.Header("Content-Type", "application/json")
	c.File(service.VideoStudioJobPath(job.ID, "result.json"))
}
