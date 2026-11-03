package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func studioEmberMJBody(c *gin.Context, fields map[string]any, modelName string) (io.Reader, string, string, error) {
	if _, exists := fields["mask"]; exists {
		return nil, "", "", errors.New("MJ does not support masks")
	}
	if fields["tile"] == true || fields["tile"] == "true" {
		return nil, "", "", errors.New("MJ does not support tile")
	}
	size, _ := fields["size"].(string)
	ratio, _ := fields["aspect_ratio"].(string)
	if strings.Contains(size, ":") {
		ratio, size = size, ""
	}
	if size == "" {
		size = "1K"
		if fields["hd"] == true {
			size = "2K"
		}
	}
	size = strings.ToUpper(size)
	if size != "1K" && size != "2K" {
		return nil, "", "", errors.New("MJ size must be 1K or 2K")
	}
	if ratio == "" {
		ratio = "1:1"
	}
	parts := strings.Split(ratio, ":")
	if len(parts) != 2 {
		return nil, "", "", errors.New("MJ aspect_ratio must be a positive ratio")
	}
	for _, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value <= 0 {
			return nil, "", "", errors.New("MJ aspect_ratio must be a positive ratio")
		}
	}
	if n, exists := fields["n"]; exists && fmt.Sprint(n) != "1" {
		return nil, "", "", errors.New("MJ requires n=1 (one task returns four images)")
	}
	payload := map[string]any{"model": modelName, "prompt": fields["prompt"], "n": 1, "size": size, "aspect_ratio": ratio}
	var reference io.Reader
	var closeReference io.Closer
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		form, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return nil, "", "", err
		}
		defer form.RemoveAll()
		count := 0
		for key, files := range form.File {
			if key != "image" && key != "image[]" {
				return nil, "", "", errors.New("MJ only accepts a single reference image, without a mask")
			}
			count += len(files)
		}
		if count != 1 {
			return nil, "", "", errors.New("MJ editing requires exactly one reference image")
		}
		for _, files := range form.File {
			file, err := files[0].Open()
			if err != nil {
				return nil, "", "", err
			}
			reference, closeReference = file, file
		}
	} else {
		references := []any{}
		if image, exists := fields["image"]; exists {
			references = append(references, image)
		}
		if images, exists := fields["images"]; exists {
			list, ok := images.([]any)
			if !ok {
				return nil, "", "", errors.New("MJ images must be an array")
			}
			references = append(references, list...)
		}
		if len(references) > 1 {
			return nil, "", "", errors.New("MJ supports only one reference image")
		}
		if len(references) == 1 {
			image, ok := references[0].(string)
			parts := strings.SplitN(image, ",", 2)
			if !ok || len(parts) != 2 || !strings.HasPrefix(parts[0], "data:image/") || !strings.HasSuffix(parts[0], ";base64") {
				return nil, "", "", errors.New("MJ reference must be an image Data URI")
			}
			reference = base64.NewDecoder(base64.StdEncoding, strings.NewReader(parts[1]))
		}
	}
	if closeReference != nil {
		defer closeReference.Close()
	}
	if reference == nil {
		body, err := common.Marshal(payload)
		return bytes.NewReader(body), "application/json", "/images/generations", err
	}
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for _, key := range []string{"model", "prompt", "n", "size", "aspect_ratio"} {
		if err := writer.WriteField(key, fmt.Sprint(payload[key])); err != nil {
			return nil, "", "", err
		}
	}
	part, err := writer.CreateFormFile("image[]", "reference.png")
	if err == nil {
		limit := int64(constant.MaxFileDownloadMB) * 1024 * 1024
		var count int64
		count, err = io.Copy(part, io.LimitReader(reference, limit+1))
		if err == nil && count > limit {
			err = errors.New("MJ reference image exceeds download size limit")
		}
	}
	if err != nil {
		return nil, "", "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", "", err
	}
	return &buffer, writer.FormDataContentType(), "/images/edits", nil
}

func submitStudioEmberMJ(c *gin.Context, job *model.StudioJob) {
	fields, _, err := readStudioJobRequest(c, "image")
	if err != nil {
		studioJobError(c, http.StatusBadRequest, err.Error())
		return
	}
	prepareVideoStudioRelay(c, 0)
	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		studioJobError(c, http.StatusInternalServerError, "could not initialize MJ task")
		return
	}
	info.InitChannelMeta(c)
	info.OriginModelName, info.UpstreamModelName = job.Model, job.Model
	info.ForcePreConsume = true
	info.Action = constant.TaskActionGenerate
	if err := helper.ModelMappedHelper(c, info, nil); err != nil {
		studioJobError(c, http.StatusBadRequest, "could not map MJ model")
		return
	}
	body, contentType, path, err := studioEmberMJBody(c, fields, info.UpstreamModelName)
	if err != nil {
		studioJobError(c, http.StatusBadRequest, err.Error())
		return
	}
	price, err := helper.ModelPriceHelperPerCall(c, info)
	if err != nil {
		studioJobError(c, http.StatusBadRequest, "MJ model price is not configured")
		return
	}
	price.AddOtherRatio("images", 4)
	price.Quota, info.QuotaClamp = common.QuotaFromFloatChecked(price.ApplyOtherRatiosToFloat(float64(price.Quota)))
	info.PriceData = price
	if apiErr := service.PreConsumeBilling(c, price.Quota, info); apiErr != nil {
		studioJobError(c, apiErr.StatusCode, "could not reserve MJ task quota")
		return
	}
	persisted := false
	defer func() {
		if !persisted && info.Billing != nil {
			info.Billing.Refund(c)
		}
	}()
	upstreamID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(job.ID)).String()
	task := &model.Task{
		TaskID: job.TaskID, Platform: constant.TaskPlatformEmberMJ, UserId: job.UserID, Group: job.Group,
		ChannelId: info.ChannelId, Quota: price.Quota, Status: model.TaskStatusSubmitted, Progress: "10%",
		SubmitTime: common.GetTimestamp(), Action: info.Action,
		Properties: model.Properties{OriginModelName: job.Model, UpstreamModelName: info.UpstreamModelName},
		PrivateData: model.TaskPrivateData{
			Key: info.ApiKey, UpstreamBaseURL: info.ChannelBaseUrl, UpstreamProxy: info.ChannelSetting.Proxy, UpstreamTaskID: upstreamID,
			BillingSource: info.BillingSource, SubscriptionId: info.SubscriptionId, NodeName: common.NodeName,
			BillingContext: &model.TaskBillingContext{OriginModelName: job.Model, ModelPrice: price.ModelPrice,
				RatioInfo: &price.GroupRatioInfo, GroupRatio: price.GroupRatioInfo.GroupRatio,
				OtherRatios: price.OtherRatios(), PerCallBilling: true},
		},
	}
	// Save the route and UUID before POST. Recovery can only query this task.
	if err := task.Insert(); err != nil {
		studioJobError(c, http.StatusInternalServerError, "could not persist MJ task")
		return
	}
	persisted = true
	if err := service.SettleBilling(c, info, price.Quota); err != nil {
		common.SysError("MJ billing settlement " + job.ID + ": " + err.Error())
		studioJobError(c, http.StatusInternalServerError, "MJ task billing needs verification")
		return
	}
	service.LogTaskConsumption(c, info)
	common.SetContextKey(c, constant.ContextKeyUpstreamRequestAttempted, true)
	response, requestErr := service.EmberMJRequest(c.Request.Context(), http.MethodPost, info.ChannelBaseUrl, path, info.ApiKey, info.ChannelSetting.Proxy, body, contentType, upstreamID)
	if requestErr != nil {
		common.SysError("MJ submission unconfirmed " + job.ID + "; query original UUID")
	} else {
		response.Body.Close()
		switch response.StatusCode {
		case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusUnprocessableEntity:
			previous := task.Status
			task.Status, task.FailReason, task.Progress = model.TaskStatusFailure, "MJ submission rejected", "100%"
			won, updateErr := task.UpdateWithStatus(previous)
			if updateErr != nil {
				common.SysError("MJ rejection persistence " + job.ID + ": " + updateErr.Error())
			} else if won {
				service.RefundTaskQuota(c.Request.Context(), task, task.FailReason)
			}
		}
	}
	c.JSON(http.StatusAccepted, gin.H{"id": task.TaskID, "status": "submitted"})
}

func isStudioEmberMJTask(job *model.StudioJob) bool {
	if job.TaskID == "" {
		return false
	}
	task, exists, err := model.GetByTaskId(job.UserID, job.TaskID)
	return err == nil && exists && task.Platform == constant.TaskPlatformEmberMJ
}

func syncStudioEmberMJResult(ctx context.Context, job *model.StudioJob) error {
	job.Status, job.Error = model.StudioJobSubmitted, ""
	task, exists, err := model.GetByTaskId(job.UserID, job.TaskID)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("MJ task record is unavailable")
	}
	if task.Status == model.TaskStatusFailure {
		job.Status, job.Error = model.StudioJobFailed, task.FailReason
		return nil
	}
	if task.Status != model.TaskStatusSuccess {
		return nil
	}
	adaptor := &service.EmberMJAdaptor{}
	if _, err := adaptor.ParseTaskResult(task.Data); err != nil {
		return err
	}
	var result service.EmberMJResult
	if err := common.Unmarshal(task.Data, &result); err != nil {
		return err
	}
	data := make([]map[string]any, 4)
	for _, image := range result.Result.Data {
		index := image.Index - 1
		name := fmt.Sprintf("image-%d", index)
		if stat, err := os.Stat(service.VideoStudioJobPath(job.ID, name)); err != nil || stat.Size() == 0 {
			// Construct the documented URL; never send credentials to a URL from provider JSON.
			response, err := service.EmberMJRequest(ctx, http.MethodGet, task.PrivateData.UpstreamBaseURL,
				"/mj/tasks/"+task.GetUpstreamTaskID()+"/images/"+strconv.Itoa(image.Index), task.PrivateData.Key, task.PrivateData.UpstreamProxy, nil, "", "")
			if err != nil {
				return err
			}
			if response.StatusCode != http.StatusOK {
				response.Body.Close()
				return fmt.Errorf("MJ image download returned HTTP %d", response.StatusCode)
			}
			err = service.StoreStudioFile(job.ID, name, response.Body, int64(constant.MaxFileDownloadMB)*1024*1024)
			response.Body.Close()
			if err != nil {
				return err
			}
		}
		data[index] = map[string]any{"url": videoStudioPrefix + "/jobs/" + job.ID + "/assets/" + strconv.Itoa(index)}
	}
	body, err := common.Marshal(map[string]any{"data": data})
	if err != nil {
		return err
	}
	if err := service.StoreStudioFile(job.ID, "result.json", bytes.NewReader(body), 0); err != nil {
		return err
	}
	job.Status, job.Error = model.StudioJobSucceeded, ""
	return nil
}
