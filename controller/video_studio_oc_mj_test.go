package controller

import (
	"context"
	"fmt"
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
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudioOCMJDurableProtocol(t *testing.T) {
	oldLimit := constant.TaskQueryLimit
	constant.TaskQueryLimit = 1000
	t.Cleanup(func() { constant.TaskQueryLimit = oldLimit })
	fetchSetting := system_setting.GetFetchSetting()
	oldFetch := *fetchSetting
	fetchSetting.EnableSSRFProtection = false // Allow only the local fixture image server during this test.
	t.Cleanup(func() { *fetchSetting = oldFetch })
	for _, scenario := range []struct {
		name       string
		submitCode int
		missingID  bool
		failTask   bool
		disconnect bool
	}{
		{name: "four image completion", submitCode: http.StatusAccepted},
		{name: "asynchronous failure refunds once", submitCode: http.StatusAccepted, failTask: true},
		{name: "submission rejected", submitCode: http.StatusBadRequest},
		{name: "unavailable without receipt", submitCode: http.StatusServiceUnavailable},
		{name: "invalid acceptance without ID", submitCode: http.StatusAccepted, missingID: true},
		{name: "submission connection lost", disconnect: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			router, token, userID := setupStudioJobsTest(t)
			const modelName, upstreamID = "mj-v8.2", "mj-upstream-public-id"
			previousPrices := ratio_setting.ModelPrice2JSONString()
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"mj-v8.2":0.01}`))
			t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(previousPrices)) })
			var submits, queries, downloads atomic.Int32
			var stage atomic.Int32
			images := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				downloads.Add(1)
				assert.Empty(t, r.Header.Get("Authorization"), "public image downloads must not carry the channel credential")
				w.Header().Set("Content-Type", "image/png")
				fmt.Fprint(w, "\x89PNG\r\n\x1a\nfixture")
			}))
			defer images.Close()
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer local-dummy-key", r.Header.Get("Authorization"))
				assert.Empty(t, r.Header.Get(service.EmberMJProtocolHeader))
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPost {
					submits.Add(1)
					assert.Equal(t, "/v1/images/midjourney/generations", r.URL.Path)
					assert.NotEmpty(t, r.Header.Get("Idempotency-Key"))
					var body map[string]any
					require.NoError(t, common.DecodeJson(r.Body, &body))
					assert.Equal(t, float64(1), body["n"])
					assert.Equal(t, "16:9", body["size"])
					assert.Equal(t, "1", body["quality"])
					assert.Equal(t, true, body["hd"])
					assert.Equal(t, false, body["stream"])
					assert.Equal(t, false, body["raw"])
					assert.Equal(t, float64(0), body["seed"])
					assert.Equal(t, []any{"data:image/png;base64,cmVmZXJlbmNl", "https://reference.invalid/image.png"}, body["images"])
					if scenario.disconnect {
						connection, _, err := w.(http.Hijacker).Hijack()
						require.NoError(t, err)
						connection.Close()
						return
					}
					w.WriteHeader(scenario.submitCode)
					if scenario.submitCode >= 400 {
						fmt.Fprint(w, `{"error":{"message":"private-submit-diagnostic"}}`)
					} else if scenario.missingID {
						fmt.Fprint(w, `{}`)
					} else {
						fmt.Fprintf(w, `{"task_id":%q,"status":"SUBMITTED"}`, upstreamID)
					}
					return
				}
				queries.Add(1)
				assert.Equal(t, "/v1/images/midjourney/tasks/"+upstreamID, r.URL.Path)
				switch stage.Load() {
				case 0:
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprint(w, `{"error":{"code":"task_not_found"}}`)
				case 1:
					fmt.Fprintf(w, `{"task_id":%q,"status":"QUEUED"}`, upstreamID)
				case 2:
					fmt.Fprint(w, `{"task_id":"another-task","status":"FAILURE"}`)
				default:
					if scenario.failTask {
						fmt.Fprintf(w, `{"task_id":%q,"status":"FAILURE"}`, upstreamID)
					} else {
						fmt.Fprintf(w, `{"task_id":%q,"status":"SUCCESS","result":{"data":[`, upstreamID)
						for i := 0; i < 4; i++ {
							if i > 0 {
								fmt.Fprint(w, ",")
							}
							if i == 0 {
								fmt.Fprintf(w, `{"url":%q}`, images.URL+"/image")
							} else {
								fmt.Fprint(w, `{"b64_json":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jMfcAAAAASUVORK5CYII="}`)
							}
						}
						fmt.Fprint(w, `],"grid_url":"not-a-fifth-image"}}`)
					}
				}
			}))
			defer upstream.Close()
			createStudioJobChannel(t, upstream.URL, modelName, constant.ChannelTypeOpenAI)
			var channel model.Channel
			require.NoError(t, model.DB.Where("models = ?", modelName).First(&channel).Error)
			require.NoError(t, model.DB.Model(&channel).Update("header_override", `{"X-OC-Image-Protocol":"mj-async"}`).Error)
			require.NoError(t, model.DB.Create(&model.Model{ModelName: modelName, Status: 1, Endpoints: `{"image-generation":"/v1/images/generations"}`}).Error)
			oldFactory := service.GetTaskAdaptorFunc
			service.GetTaskAdaptorFunc = func(constant.TaskPlatform) service.TaskPollingAdaptor { return &service.OCMJAdaptor{} }
			t.Cleanup(func() { service.GetTaskAdaptorFunc = oldFactory })
			body := `{"model":"mj-v8.2","prompt":"offline fixture --ar 16:9","n":1,"size":"16:9","quality":"1","hd":true,"raw":false,"seed":0,"images":["data:image/png;base64,cmVmZXJlbmNl","https://reference.invalid/image.png"]}`
			requestID := common.GetUUID()
			job, receipt := submitStudioJobTest(t, router, token, requestID, "/jobs/images/generations", body)
			require.Equal(t, http.StatusAccepted, receipt.Code, receipt.Body.String())
			require.NoError(t, runVideoStudioJobs(context.Background(), "oc-initial"))
			persisted, err := model.FindStudioJob(userID, job.ID)
			require.NoError(t, err)
			task, exists, err := model.GetByTaskId(userID, persisted.TaskID)
			require.NoError(t, err)
			require.True(t, exists, persisted.Error)
			assert.Equal(t, constant.TaskPlatform(constant.TaskPlatformOCMJ), task.Platform)
			reserved, err := model.GetUserQuota(userID, false)
			require.NoError(t, err)
			accepted := scenario.submitCode == http.StatusAccepted && !scenario.missingID && !scenario.disconnect
			if scenario.submitCode != http.StatusBadRequest {
				assert.Equal(t, 9_980_000, reserved, "reserve exactly four images at $0.01 each")
			}
			switch {
			case accepted:
				assert.Equal(t, upstreamID, task.GetUpstreamTaskID())
				require.Equal(t, model.StudioJobSubmitted, persisted.Status, persisted.Error)
				require.Less(t, reserved, 10_000_000)
				require.NoError(t, model.DB.Model(&channel).Updates(map[string]any{
					"key": "changed-key", "base_url": "https://untrusted.invalid",
				}).Error)
				for current := int32(0); current < 3; current++ {
					stage.Store(current)
					service.RunTaskPollingOnce(context.Background(), nil)
					require.NoError(t, runVideoStudioJobs(context.Background(), "oc-pending"))
					pending, err := model.FindStudioJob(userID, job.ID)
					require.NoError(t, err)
					assert.Equal(t, model.StudioJobSubmitted, pending.Status, pending.Error)
					quota, err := model.GetUserQuota(userID, false)
					require.NoError(t, err)
					assert.Equal(t, reserved, quota)
				}
				stage.Store(3)
				service.RunTaskPollingOnce(context.Background(), nil)
				require.NoError(t, runVideoStudioJobs(context.Background(), "oc-restarted"))
			case scenario.submitCode == http.StatusBadRequest:
				assert.Equal(t, model.StudioJobFailed, persisted.Status)
				assert.Contains(t, persisted.Error, "HTTP 400")
				assert.Equal(t, 10_000_000, reserved)
				assert.Contains(t, task.PrivateData.UpstreamSubmitErrorBody, "private-submit-diagnostic")
			default:
				assert.Equal(t, model.StudioJobUncertain, persisted.Status, persisted.Error)
				assert.Empty(t, task.GetUpstreamTaskID(), "a local task ID must never be queried as an OC ID")
				service.RunTaskPollingOnce(context.Background(), nil)
				assert.Zero(t, queries.Load())
			}
			persisted, err = model.FindStudioJob(userID, job.ID)
			require.NoError(t, err)
			finalQuota, err := model.GetUserQuota(userID, false)
			require.NoError(t, err)
			if accepted && !scenario.failTask {
				require.Equal(t, model.StudioJobSucceeded, persisted.Status, persisted.Error)
				result, err := os.ReadFile(service.VideoStudioJobPath(job.ID, "result.json"))
				require.NoError(t, err)
				var response struct{ Data []map[string]any }
				require.NoError(t, common.Unmarshal(result, &response))
				require.Len(t, response.Data, 4)
				for i := range response.Data {
					assert.Equal(t, videoStudioPrefix+"/jobs/"+job.ID+"/assets/"+fmt.Sprint(i), response.Data[i]["url"])
					request := httptest.NewRequest(http.MethodGet, "/jobs/"+job.ID+"/assets/"+fmt.Sprint(i), nil)
					request.Header.Set("Authorization", "Bearer "+token)
					asset := httptest.NewRecorder()
					router.ServeHTTP(asset, request)
					assert.Equal(t, http.StatusOK, asset.Code)
					assert.True(t, strings.HasPrefix(asset.Body.String(), "\x89PNG\r\n\x1a\n"))
				}
				assert.Equal(t, reserved, finalQuota)
				assert.Equal(t, int32(1), downloads.Load())
			} else if scenario.failTask {
				assert.Equal(t, model.StudioJobFailed, persisted.Status)
				assert.Equal(t, 10_000_000, finalQuota)
			}
			duplicate, receipt := submitStudioJobTest(t, router, token, requestID, "/jobs/images/generations", body)
			require.Equal(t, http.StatusOK, receipt.Code)
			assert.Equal(t, job.ID, duplicate.ID)
			service.RunTaskPollingOnce(context.Background(), nil)
			require.NoError(t, runVideoStudioJobs(context.Background(), "oc-repeat"))
			assert.Equal(t, int32(1), submits.Load())
			quota, err := model.GetUserQuota(userID, false)
			require.NoError(t, err)
			assert.Equal(t, finalQuota, quota)
		})
	}
}

func TestStudioOCMJParameterBoundaries(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	for _, fields := range []map[string]any{
		{"prompt": "test", "n": 4},
		{"prompt": "test", "action": "upscale"},
		{"prompt": "test", "quality": "high"},
		{"prompt": "test", "images": []any{"https://a.invalid/a", "https://b.invalid/b"}},
		{"prompt": "test", "size": "2K", "hd": false},
	} {
		_, _, _, err := studioOCMJBody(c, fields, "Midjourney v8.2-test")
		require.Error(t, err)
	}
}
