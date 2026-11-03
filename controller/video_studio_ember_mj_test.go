package controller

import (
	"context"
	"fmt"
	"io"
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
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudioEmberMJDurableProtocol(t *testing.T) {
	oldLimit, oldDownload := constant.TaskQueryLimit, constant.MaxFileDownloadMB
	constant.TaskQueryLimit, constant.MaxFileDownloadMB = 1000, 64
	t.Cleanup(func() {
		constant.TaskQueryLimit, constant.MaxFileDownloadMB = oldLimit, oldDownload
	})
	for _, scenario := range []struct {
		name       string
		submitCode int
		reference  bool
		failTask   bool
	}{
		{"accepted", http.StatusAccepted, false, false},
		{"ambiguous submission recovers", http.StatusBadGateway, false, false},
		{"reference image", http.StatusAccepted, true, false},
		{"definitive rejection", http.StatusBadRequest, false, false},
		{"asynchronous failure", http.StatusAccepted, false, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			router, token, userID := setupStudioJobsTest(t)
			const modelName = "Midjourney-v8.2"
			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"Midjourney-v8.2":0.01}`))
			var submits, downloads, queryStage atomic.Int32
			var requestID string
			var upstream *httptest.Server
			upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer local-dummy-key", r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPost {
					submits.Add(1)
					requestID = r.Header.Get("Idempotency-Key")
					_, err := uuid.Parse(requestID)
					assert.NoError(t, err)
					assert.Equal(t, "mj-proxy", r.Header.Get(service.EmberMJProtocolHeader))
					if scenario.reference {
						assert.Equal(t, "/v1/images/edits", r.URL.Path)
						require.NoError(t, r.ParseMultipartForm(1024))
						assert.Equal(t, "2K", r.FormValue("size"))
						assert.Equal(t, "16:9", r.FormValue("aspect_ratio"))
						assert.Equal(t, "1", r.FormValue("n"))
						file, _, err := r.FormFile("image[]")
						require.NoError(t, err)
						b, err := io.ReadAll(file)
						file.Close()
						require.NoError(t, err)
						assert.Equal(t, []byte("reference"), b)
					} else {
						assert.Equal(t, "/v1/images/generations", r.URL.Path)
						var body map[string]any
						require.NoError(t, common.DecodeJson(r.Body, &body))
						assert.Equal(t, "2K", body["size"])
						assert.Equal(t, "16:9", body["aspect_ratio"])
						assert.Equal(t, float64(1), body["n"])
						assert.NotContains(t, body, "hd")
						assert.NotContains(t, body, "response_format")
					}
					w.WriteHeader(scenario.submitCode)
					fmt.Fprintf(w, `{"id":%q,"status":"submitting"}`, requestID)
					return
				}
				assert.Contains(t, r.URL.Path, "/v1/mj/tasks/"+requestID)
				if strings.Contains(r.URL.Path, "/images/") {
					downloads.Add(1)
					w.Header().Set("Content-Type", "image/png")
					_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nfixture"))
					return
				}
				switch queryStage.Load() {
				case 0:
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprint(w, `{"error":"not visible yet"}`)
					return
				case 1:
					fmt.Fprintf(w, `{"id":%q,"status":"uncertain"}`, requestID)
					return
				case 2:
					fmt.Fprintf(w, `{"id":%q,"status":"failed"}`, uuid.NewString())
					return
				}
				state := "completed"
				if scenario.failTask {
					state = "failed"
				}
				fmt.Fprintf(w, `{"id":%q,"status":%q,"result":{"data":[`, requestID, state)
				for i := 1; i <= 4; i++ {
					if i > 1 {
						fmt.Fprint(w, ",")
					}
					// Credentialed downloads must ignore these untrusted result URLs.
					fmt.Fprintf(w, `{"index":%d,"url":"https://untrusted.invalid/credential-sink"}`, i)
				}
				fmt.Fprint(w, `]}}`)
			}))
			defer upstream.Close()
			createStudioJobChannel(t, upstream.URL, modelName, constant.ChannelTypeOpenAI)
			require.NoError(t, model.DB.Create(&model.Model{
				ModelName: modelName, Status: 1, Endpoints: `{"image-generation":"/v1/images/generations"}`,
			}).Error)
			var channel model.Channel
			require.NoError(t, model.DB.Where("models = ?", modelName).First(&channel).Error)
			require.NoError(t, model.DB.Model(&channel).Update("header_override", `{"X-Ember-Image-Protocol":"mj-proxy"}`).Error)
			oldFactory := service.GetTaskAdaptorFunc
			service.GetTaskAdaptorFunc = func(constant.TaskPlatform) service.TaskPollingAdaptor { return &service.EmberMJAdaptor{} }
			t.Cleanup(func() { service.GetTaskAdaptorFunc = oldFactory })
			ref := ""
			if scenario.reference {
				ref = `,"image":"data:image/png;base64,cmVmZXJlbmNl"`
			}
			body := `{"model":"Midjourney-v8.2","prompt":"offline fixture","n":1,"size":"16:9","hd":true` + ref + `}`
			id := common.GetUUID()
			job, receipt := submitStudioJobTest(t, router, token, id, "/jobs/images/generations", body)
			require.Equal(t, http.StatusAccepted, receipt.Code, receipt.Body.String())
			require.NoError(t, runVideoStudioJobs(context.Background(), "ember-initial"))
			persisted, err := model.FindStudioJob(userID, job.ID)
			require.NoError(t, err)
			task, exists, err := model.GetByTaskId(userID, persisted.TaskID)
			require.NoError(t, err)
			if !exists {
				response, _ := os.ReadFile(service.VideoStudioJobPath(job.ID, "response.bin"))
				require.True(t, exists, "%s: %s", persisted.Error, response)
			}
			assert.Equal(t, constant.TaskPlatform(constant.TaskPlatformEmberMJ), task.Platform)
			assert.Equal(t, requestID, task.GetUpstreamTaskID())
			assert.NotEqual(t, id, requestID, "upstream UUID must be scoped to the local task, not a user-chosen cross-user UUID")
			reserved, err := model.GetUserQuota(userID, false)
			require.NoError(t, err)
			if scenario.submitCode != http.StatusBadRequest {
				require.Equal(t, model.StudioJobSubmitted, persisted.Status, persisted.Error)
				require.Less(t, reserved, 10_000_000)
				// A channel edit must not reroute an already accepted task.
				require.NoError(t, model.DB.Model(&channel).Updates(map[string]any{
					"key": "changed-key", "base_url": "https://untrusted.invalid",
				}).Error)
				for stage := int32(0); stage < 3; stage++ {
					queryStage.Store(stage)
					service.RunTaskPollingOnce(context.Background(), nil)
					require.NoError(t, runVideoStudioJobs(context.Background(), "ember-pending"))
					pending, err := model.FindStudioJob(userID, job.ID)
					require.NoError(t, err)
					assert.Equal(t, model.StudioJobSubmitted, pending.Status)
					quota, err := model.GetUserQuota(userID, false)
					require.NoError(t, err)
					assert.Equal(t, reserved, quota)
					assert.Zero(t, downloads.Load())
				}
				queryStage.Store(3)
				service.RunTaskPollingOnce(context.Background(), nil)
				require.NoError(t, runVideoStudioJobs(context.Background(), "ember-restart"))
			}
			persisted, err = model.FindStudioJob(userID, job.ID)
			require.NoError(t, err)
			finalQuota, err := model.GetUserQuota(userID, false)
			require.NoError(t, err)
			if scenario.failTask || scenario.submitCode == http.StatusBadRequest {
				assert.Equal(t, model.StudioJobFailed, persisted.Status, persisted.Error)
				assert.Equal(t, 10_000_000, finalQuota, "failed task refunds exactly once")
				assert.Zero(t, downloads.Load())
			} else {
				assert.Equal(t, model.StudioJobSucceeded, persisted.Status, persisted.Error)
				assert.Equal(t, reserved, finalQuota, "poll and download must not charge again")
				assert.Equal(t, int32(4), downloads.Load())
				result, err := os.ReadFile(service.VideoStudioJobPath(job.ID, "result.json"))
				require.NoError(t, err)
				var response struct{ Data []map[string]any }
				require.NoError(t, common.Unmarshal(result, &response))
				assert.Len(t, response.Data, 4)
				assert.NotContains(t, string(result), "local-dummy-key")
				assert.NotContains(t, string(result), "untrusted.invalid")
			}
			duplicate, receipt := submitStudioJobTest(t, router, token, id, "/jobs/images/generations", body)
			require.Equal(t, http.StatusOK, receipt.Code, receipt.Body.String())
			assert.Equal(t, job.ID, duplicate.ID)
			service.RunTaskPollingOnce(context.Background(), nil)
			require.NoError(t, runVideoStudioJobs(context.Background(), "ember-repeat"))
			assert.Equal(t, int32(1), submits.Load(), "never repeat a billable POST")
			quota, err := model.GetUserQuota(userID, false)
			require.NoError(t, err)
			assert.Equal(t, finalQuota, quota)
		})
	}
}
