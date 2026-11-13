package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

const OCMJProtocolHeader = "X-OC-Image-Protocol"

func UsesOCMJProtocol(baseURL, modelName string, headers map[string]interface{}) bool {
	name := strings.ToLower(modelName)
	if !strings.HasPrefix(name, "midjourney") && !strings.HasPrefix(name, "mj-") {
		return false
	}
	for key, value := range headers {
		if strings.EqualFold(key, OCMJProtocolHeader) && value == "mj-async" {
			return true
		}
	}
	u, err := url.Parse(baseURL)
	return err == nil && strings.EqualFold(u.Hostname(), "api.osodm.com")
}

type OCMJResult struct {
	TaskID   string          `json:"task_id"`
	Status   string          `json:"status"`
	Progress string          `json:"progress"`
	Result   json.RawMessage `json:"result"`
}

func ValidOCMJTaskID(id string) bool {
	if len(id) == 0 || len(id) > 191 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func OCMJRequest(ctx context.Context, method, baseURL, path, key, proxy string, body io.Reader, requestID string) (*http.Response, error) {
	return mjRequest(ctx, method, baseURL, path, key, proxy, body, "application/json", requestID, "")
}

type OCMJAdaptor struct{}

func (*OCMJAdaptor) Init(*relaycommon.RelayInfo) {}

func (*OCMJAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	id, _ := body["task_id"].(string)
	if !ValidOCMJTaskID(id) {
		return nil, errors.New("MJ upstream task ID is unavailable")
	}
	ctx, _ := body["context"].(context.Context)
	if ctx == nil {
		ctx = context.Background()
	}
	response, err := OCMJRequest(ctx, http.MethodGet, baseURL, "/images/midjourney/tasks/"+id, key, proxy, nil, "")
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("MJ task query returned HTTP %d; retain original task", response.StatusCode)
	}
	return response, nil
}

func (*OCMJAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	var result OCMJResult
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if !ValidOCMJTaskID(result.TaskID) {
		return nil, errors.New("MJ response has no valid upstream task ID")
	}
	info := &relaycommon.TaskInfo{TaskID: result.TaskID}
	switch strings.ToUpper(result.Status) {
	case string(model.TaskStatusNotStart), model.TaskStatusSubmitted:
		info.Status = model.TaskStatusSubmitted
	case model.TaskStatusQueued:
		info.Status = model.TaskStatusQueued
	case model.TaskStatusInProgress:
		info.Status = model.TaskStatusInProgress
	case model.TaskStatusSuccess:
		var images struct {
			Data []struct {
				URL    string `json:"url"`
				Base64 string `json:"b64_json"`
			} `json:"data"`
		}
		if err := common.Unmarshal(result.Result, &images); err != nil {
			return nil, err
		}
		if len(images.Data) != 4 {
			return nil, errors.New("MJ completed task must contain four images")
		}
		for _, image := range images.Data {
			if image.URL == "" && image.Base64 == "" {
				return nil, errors.New("MJ completed task contains an empty image")
			}
		}
		info.Status = model.TaskStatusSuccess
	case model.TaskStatusFailure:
		info.Status, info.Reason = model.TaskStatusFailure, "MJ generation failed"
	default:
		return nil, errors.New("unrecognized MJ task state; retain original task")
	}
	return info, nil
}

func (*OCMJAdaptor) AdjustBillingOnComplete(*model.Task, *relaycommon.TaskInfo) int { return 0 }
