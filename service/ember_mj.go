package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/google/uuid"
)

const EmberMJProtocolHeader = "X-Ember-Image-Protocol"

// The protocol belongs to the selected channel, not to every Midjourney model.
func UsesEmberMJProtocol(baseURL, modelName string, headers map[string]interface{}) bool {
	name := strings.ToLower(modelName)
	if !strings.HasPrefix(name, "midjourney") && !strings.HasPrefix(name, "mj-") {
		return false
	}
	for key, value := range headers {
		if strings.EqualFold(key, EmberMJProtocolHeader) && value == "mj-proxy" {
			return true
		}
	}
	u, err := url.Parse(baseURL)
	return err == nil && strings.EqualFold(u.Hostname(), "api.uuyue.top")
}

type EmberMJResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Result struct {
		Data []struct {
			Index  int    `json:"index"`
			URL    string `json:"url"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"data"`
	} `json:"result"`
}

// Never follow a provider redirect with the saved task credential.
func EmberMJRequest(ctx context.Context, method, baseURL, path, key, proxy string, body io.Reader, contentType, requestID string) (*http.Response, error) {
	return mjRequest(ctx, method, baseURL, path, key, proxy, body, contentType, requestID, "mj-proxy")
}

func mjRequest(ctx context.Context, method, baseURL, path, key, proxy string, body io.Reader, contentType, requestID, protocol string) (*http.Response, error) {
	target, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || target.Host == "" || (target.Scheme != "https" && target.Scheme != "http") || target.User != nil {
		return nil, errors.New("invalid MJ channel address")
	}
	target.Path = strings.TrimRight(target.Path, "/")
	if !strings.HasSuffix(target.Path, "/v1") {
		target.Path += "/v1"
	}
	target.Path += path
	target.RawQuery, target.Fragment = "", ""
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		if protocol != "" {
			req.Header.Set(EmberMJProtocolHeader, protocol)
		}
		req.Header.Set("Idempotency-Key", requestID)
		req.Header.Set("Content-Type", contentType)
		// A task submission must never be replayed by the HTTP transport.
		req.GetBody = nil
	}
	client, err := GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return copy.Do(req)
}

type EmberMJAdaptor struct{}

func (*EmberMJAdaptor) Init(*relaycommon.RelayInfo) {}

func (*EmberMJAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	id, _ := body["task_id"].(string)
	if _, err := uuid.Parse(id); err != nil {
		return nil, errors.New("invalid MJ task UUID")
	}
	ctx, _ := body["context"].(context.Context)
	if ctx == nil {
		ctx = context.Background()
	}
	response, err := EmberMJRequest(ctx, http.MethodGet, baseURL, "/mj/tasks/"+id, key, proxy, nil, "", "")
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("MJ task query returned HTTP %d; retain original task", response.StatusCode)
	}
	return response, nil
}

func (*EmberMJAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	var result EmberMJResult
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	info := &relaycommon.TaskInfo{TaskID: result.ID}
	switch result.Status {
	case "submitting", "pending", "uncertain":
		info.Status = model.TaskStatusSubmitted
	case "processing":
		info.Status = model.TaskStatusInProgress
	case "completed":
		if len(result.Result.Data) != 4 {
			return nil, errors.New("MJ completed task must contain four images")
		}
		seen := make(map[int]bool, 4)
		for _, image := range result.Result.Data {
			if image.Index < 1 || image.Index > 4 || seen[image.Index] {
				return nil, errors.New("invalid MJ image indexes")
			}
			seen[image.Index] = true
		}
		info.Status = model.TaskStatusSuccess
	case "failed":
		info.Status, info.Reason = model.TaskStatusFailure, "MJ generation failed"
	default:
		return nil, errors.New("unrecognized MJ task state; retain original task")
	}
	return info, nil
}

func (*EmberMJAdaptor) AdjustBillingOnComplete(*model.Task, *relaycommon.TaskInfo) int { return 0 }
