package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
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
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoStudioSoraAndOpenAIResultRecovery(t *testing.T) {
	for _, channelType := range []int{constant.ChannelTypeSora, constant.ChannelTypeOpenAI} {
		for _, resultKind := range []string{"media-url", "data-url", "missing-url", "self-proxy-url"} {
			t.Run(fmt.Sprintf("channel-%d/%s", channelType, resultKind), func(t *testing.T) {
				router, token, userID := setupStudioJobsTest(t)
				media := []byte("saved-video-result")
				var mediaCalls, contentCalls atomic.Int32
				contentFallback := resultKind == "missing-url" || resultKind == "self-proxy-url"
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, http.MethodGet, r.Method, "recovery must never resubmit a paid generation")
					switch r.URL.Path {
					case "/media/result.mp4":
						mediaCalls.Add(1)
						assert.Empty(t, r.Header.Get("Authorization"), "provider credentials must not be sent to a media URL")
					case "/v1/videos/upstream-completed/content":
						contentCalls.Add(1)
						assert.Equal(t, "Bearer local-dummy-key", r.Header.Get("Authorization"))
						if !contentFallback {
							http.Error(w, "content endpoint expired", http.StatusGone)
							return
						}
					default:
						http.NotFound(w, r)
						return
					}
					w.Header().Set("Content-Type", "video/mp4")
					_, _ = w.Write(media)
				}))
				t.Cleanup(upstream.Close)
				fetch := system_setting.GetFetchSetting()
				previous := *fetch
				_, port, err := net.SplitHostPort(upstream.Listener.Addr().String())
				require.NoError(t, err)
				fetch.AllowPrivateIp = true
				fetch.AllowedPorts = []string{port}
				t.Cleanup(func() { *fetch = previous })
				service.InitHttpClient()
				channel := &model.Channel{
					Type: channelType, Key: "local-dummy-key", BaseURL: &upstream.URL,
					Status: common.ChannelStatusEnabled, Name: "saved-video",
				}
				require.NoError(t, model.DB.Create(channel).Error)
				resultURL := ""
				switch resultKind {
				case "media-url":
					resultURL = upstream.URL + "/media/result.mp4"
				case "data-url":
					resultURL = "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(media)
				case "self-proxy-url":
					resultURL = "https://site.invalid/v1/videos/task-completed/content"
				}
				native := &model.Task{
					TaskID: "task-completed", UserId: userID, ChannelId: channel.Id, Status: model.TaskStatusSuccess,
					PrivateData: model.TaskPrivateData{UpstreamTaskID: "upstream-completed", ResultURL: resultURL},
				}
				require.NoError(t, model.DB.Create(native).Error)
				job := &model.StudioJob{
					ID: "studio_" + common.GetUUID(), UserID: userID, RequestID: common.GetUUID(),
					Kind: "video", TaskID: native.TaskID, Status: model.StudioJobSubmitted, RunnerTask: "old-run",
				}
				require.NoError(t, model.DB.Create(job).Error)
				require.NoError(t, service.StoreStudioFile(job.ID, "request.bin", strings.NewReader("already submitted"), 0))

				require.NoError(t, runVideoStudioJobs(context.Background(), "result-recovery"))
				persisted, err := model.FindStudioJob(userID, job.ID)
				require.NoError(t, err)
				require.Equal(t, model.StudioJobSucceeded, persisted.Status, persisted.Error)
				saved, err := os.ReadFile(service.VideoStudioJobPath(job.ID, "video"))
				require.NoError(t, err)
				assert.Equal(t, media, saved)
				wantMedia, wantContent := int32(0), int32(0)
				if resultKind == "media-url" {
					wantMedia = 1
				}
				if contentFallback {
					wantContent = 1
				}
				assert.Equal(t, wantMedia, mediaCalls.Load())
				assert.Equal(t, wantContent, contentCalls.Load())
				require.NoError(t, runVideoStudioJobs(context.Background(), "after-recovery"))
				assert.Equal(t, wantMedia, mediaCalls.Load(), "completed jobs must not be downloaded repeatedly")
				assert.Equal(t, wantContent, contentCalls.Load())
				quota, err := model.GetUserQuota(userID, false)
				require.NoError(t, err)
				assert.Equal(t, 10_000_000, quota, "media recovery must not charge the user again")

				request := httptest.NewRequest(http.MethodGet, "/jobs/"+job.ID+"/result", nil)
				request.Header.Set("Authorization", "Bearer "+token)
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				require.Equal(t, http.StatusOK, response.Code)
				assert.Equal(t, media, response.Body.Bytes())
				assert.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
				otherToken, _ := createVideoStudioRelayUser(t, "result-other-user")
				request = httptest.NewRequest(http.MethodGet, "/jobs/"+job.ID+"/result", nil)
				request.Header.Set("Authorization", "Bearer "+otherToken)
				response = httptest.NewRecorder()
				router.ServeHTTP(response, request)
				assert.Equal(t, http.StatusNotFound, response.Code)
			})
		}
	}
}
