package sora

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGwayMultipartReferenceUploadAndJSONForwarding(t *testing.T) {
	before := ratio_setting.ModelPrice2JSONString()
	oldDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() {
		model.DB = oldDB
		require.NoError(t, sqlDB.Close())
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(before))
	})
	require.NoError(t, db.AutoMigrate(&model.Model{}))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"demo":0.3,"demo-720p":0.45}`))
	require.NoError(t, db.Create(&model.Model{
		ModelName: "demo", Status: 1,
		Parameters: `{"provider":"gway","video":{"currency":"CNY","billing_unit":"second","durations":[5,30],"resolutions":["720p"]}}`,
	}).Error)
	for _, tc := range []struct {
		name                string
		seconds             string
		duplicate           bool
		uploadSuccess       bool
		wantValidationError bool
	}{
		{"three references", "30", false, true, false},
		{"upload business rejection", "30", false, false, false},
		{"duration checked before upload", "31", false, true, true},
		{"duplicate scalar rejected", "30", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uploads := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				uploads++
				assert.Equal(t, "/v1/media/upload", r.URL.Path)
				assert.Equal(t, "Bearer mock-key", r.Header.Get("Authorization"))
				require.NoError(t, r.ParseMultipartForm(1<<20))
				defer r.MultipartForm.RemoveAll()
				assert.Equal(t, "image", r.FormValue("kind"))
				files := r.MultipartForm.File["file"]
				require.Len(t, files, 3)
				for i, header := range files {
					assert.Equal(t, strconv.Itoa(i)+".png", header.Filename)
					file, err := header.Open()
					require.NoError(t, err)
					content, err := io.ReadAll(file)
					require.NoError(t, err)
					file.Close()
					assert.Equal(t, "image-"+strconv.Itoa(i), string(content))
				}
				w.Header().Set("Content-Type", "application/json")
				if !tc.uploadSuccess {
					io.WriteString(w, `{"success":false,"message":"storage quota exhausted"}`)
					return
				}
				io.WriteString(w, `{"success":true,"data":{"items":[{"url":"https://media.example/0.png"},{"url":"https://media.example/1.png"},{"url":"https://media.example/2.png"}]}}`)
			}))
			defer server.Close()
			var source bytes.Buffer
			form := multipart.NewWriter(&source)
			require.NoError(t, form.WriteField("model", "demo"))
			require.NoError(t, form.WriteField("prompt", "use image 1 then 2 then 3"))
			require.NoError(t, form.WriteField("seconds", tc.seconds))
			if tc.duplicate {
				require.NoError(t, form.WriteField("seconds", "5"))
			}
			for i := 0; i < 3; i++ {
				part, err := form.CreateFormFile("input_reference", strconv.Itoa(i)+".png")
				require.NoError(t, err)
				_, err = io.WriteString(part, "image-"+strconv.Itoa(i))
				require.NoError(t, err)
			}
			require.NoError(t, form.Close())
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", &source)
			c.Request.Header.Set("Content-Type", form.FormDataContentType())
			t.Cleanup(func() { common.CleanupBodyStorage(c) })
			info := &relaycommon.RelayInfo{
				ChannelMeta:   &relaycommon.ChannelMeta{ChannelBaseUrl: "https://www.gway.dev", ApiKey: "mock-key", UpstreamModelName: "mapped-demo"},
				TaskRelayInfo: &relaycommon.TaskRelayInfo{},
			}
			a := &TaskAdaptor{}
			a.Init(info)
			taskErr := a.ValidateRequestAndSetAction(c, info)
			if tc.wantValidationError {
				require.NotNil(t, taskErr)
				assert.True(t, taskErr.LocalError)
				assert.Zero(t, uploads)
				return
			}
			require.Nil(t, taskErr)
			outbound := httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
			require.NoError(t, a.BuildRequestHeader(c, outbound, info))
			assert.Equal(t, "application/json", outbound.Header.Get("Content-Type"))
			a.baseURL = server.URL
			body, err := a.BuildRequestBody(c, info)
			assert.Equal(t, 1, uploads)
			if !tc.uploadSuccess {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "storage quota exhausted")
				assert.Contains(t, err.Error(), "video was not submitted")
				assert.Nil(t, body)
				return
			}
			require.NoError(t, err)
			var sent map[string]any
			require.NoError(t, common.DecodeJson(body, &sent))
			assert.Equal(t, []any{"https://media.example/0.png", "https://media.example/1.png", "https://media.example/2.png"}, sent["images"])
			assert.Equal(t, "mapped-demo", sent["model"])
			assert.Equal(t, float64(30), sent["duration"])
			assert.NotContains(t, sent, "input_reference")
			assert.NotContains(t, sent, "seconds")
			assert.False(t, strings.Contains(outbound.Header.Get("Content-Type"), "multipart"))
		})
	}
}

func TestGwayBillingUnitsAndForwardedRequest(t *testing.T) {
	before := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(before)) })
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"demo":0.3,"demo-480p":0.3,"demo-720p":0.45}`))
	for _, unit := range []string{"request", "second"} {
		for _, seconds := range []int{5, 15} {
			fields := map[string]any{"model": "demo", "prompt": "test", "seconds": seconds, "resolution": "720p", "watermark": false}
			profile := gwayVideoProfile{Currency: "CNY", BillingUnit: unit, Durations: []int{5, 15}, Resolutions: []string{"480p", "720p"}}
			billing, err := normalizeGwayVideo(fields, profile)
			require.NoError(t, err)
			quantity := 1
			if unit == "second" {
				quantity = seconds
			}
			assert.Equal(t, quantity, billing.Quantity)
			assert.Equal(t, 0.45, billing.UnitPrice)
			body, err := buildGwayVideoBody(billing, "upstream-demo")
			require.NoError(t, err)
			data, err := io.ReadAll(body)
			require.NoError(t, err)
			var sent map[string]any
			require.NoError(t, common.Unmarshal(data, &sent))
			assert.Equal(t, float64(seconds), sent["duration"])
			assert.Equal(t, "upstream-demo", sent["model"])
			assert.Equal(t, false, sent["watermark"])
			assert.NotContains(t, sent, "seconds")
		}
	}
}

func TestGwayRejectsInvalidBillingParameters(t *testing.T) {
	before := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(before)) })
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"demo":0.3,"demo-720p":0.45}`))
	profile := gwayVideoProfile{Currency: "CNY", BillingUnit: "second", Durations: []int{5, 15}, Resolutions: []string{"720p"}}
	for _, payload := range []string{
		`{"model":"demo","prompt":"x","seconds":0}`,
		`{"model":"demo","prompt":"x","seconds":-1}`,
		`{"model":"demo","prompt":"x","seconds":1.5}`,
		`{"model":"demo","prompt":"x","duration":"18446744073709551615"}`,
		`{"model":"demo","prompt":"x","seconds":5,"duration":15}`,
		`{"model":"demo","prompt":"x","seconds":31}`,
		`{"model":"demo","prompt":"x","resolution":"1080p"}`,
		`{"model":"demo","prompt":"x","n":2}`,
		`{"model":"demo","prompt":"x","metadata":{"n":2}}`,
		`{"model":"demo","prompt":"x","seconds":5,"metadata":{"seconds":15}}`,
	} {
		var fields map[string]any
		require.NoError(t, common.UnmarshalJsonStr(payload, &fields))
		_, err := normalizeGwayVideo(fields, profile)
		require.Error(t, err, payload)
	}
	profile.RequireDuration = true
	_, err := normalizeGwayVideo(map[string]any{"model": "demo", "prompt": "x"}, profile)
	require.Error(t, err)
}

func TestGwayPollingPreservesResultURLAndFailures(t *testing.T) {
	a := &TaskAdaptor{}
	result, err := a.ParseTaskResult([]byte(`{"id":"provider-id","status":"completed","progress":100,"result_urls":["https://example.com/video.mp4"]}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, result.Status)
	assert.Equal(t, "https://example.com/video.mp4", result.Url)
	result, err = a.ParseTaskResult([]byte(`{"id":"provider-id","status":"failed","error":{"message":"provider rejected"}}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusFailure, result.Status)
	assert.Equal(t, "provider rejected", result.Reason)
}

func TestGwayWalletChargeAndFailureRefund(t *testing.T) {
	beforePrices := ratio_setting.ModelPrice2JSONString()
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldRedis, oldBatch, oldLog := common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = false, false, true
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = oldRedis, oldBatch, oldLog
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(beforePrices))
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Task{}, &model.Log{}, &model.Model{}))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"request-demo":0.6,"request-demo-720p":1.2,"second-demo":0.3,"second-demo-720p":0.45}`))
	for index, tc := range []struct {
		name    string
		unit    string
		seconds int
		charge  int
	}{
		{"request-demo", "request", 5, 600000},
		{"request-demo", "request", 15, 600000},
		{"second-demo", "second", 5, 1125000},
		{"second-demo", "second", 15, 3375000},
	} {
		id := index + 1810
		const initial = 10000000
		require.NoError(t, db.Create(&model.User{Id: id, Username: strconv.Itoa(id), AffCode: strconv.Itoa(id), Quota: initial}).Error)
		token := &model.Token{Id: id, UserId: id, Key: tc.name + strconv.Itoa(id), Name: "gway-test", RemainQuota: initial}
		require.NoError(t, db.Create(token).Error)
		require.NoError(t, db.Create(&model.Channel{Id: id, Name: tc.name, Key: "test"}).Error)
		require.NoError(t, db.Where("model_name = ?", tc.name).Delete(&model.Model{}).Error)
		parameters, err := common.Marshal(map[string]any{
			"provider": "gway", "video": gwayVideoProfile{
				Currency: "CNY", BillingUnit: tc.unit, Durations: []int{5, 15}, Resolutions: []string{"720p"},
			},
		})
		require.NoError(t, err)
		require.NoError(t, db.Create(&model.Model{ModelName: tc.name, Status: 1, Parameters: string(parameters)}).Error)
		payload, err := common.Marshal(map[string]any{"model": tc.name, "prompt": "test", "seconds": tc.seconds})
		require.NoError(t, err)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(payload))
		c.Request.Header.Set("Content-Type", "application/json")
		info := &relaycommon.RelayInfo{
			UserId: id, TokenId: id, TokenKey: token.Key, OriginModelName: tc.name, UsingGroup: "default", ForcePreConsume: true,
			UserSetting:   dto.UserSetting{BillingPreference: "wallet_only"},
			ChannelMeta:   &relaycommon.ChannelMeta{ChannelId: id, ChannelBaseUrl: "https://www.gway.dev"},
			TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		}
		a := &TaskAdaptor{}
		a.Init(info)
		require.Nil(t, a.ValidateRequestAndSetAction(c, info))
		info.PriceData, err = helper.ModelPriceHelperPerCall(c, info)
		require.NoError(t, err)
		for name, ratio := range a.EstimateBilling(c, info) {
			info.PriceData.AddOtherRatio(name, ratio)
		}
		info.PriceData.Quota = common.QuotaFromFloat(info.PriceData.ApplyOtherRatiosToFloat(float64(info.PriceData.Quota)))
		assert.InDelta(t, tc.charge, info.PriceData.Quota, 1)
		require.Nil(t, service.PreConsumeBilling(c, info.PriceData.Quota, info))
		require.NoError(t, service.SettleBilling(c, info, info.PriceData.Quota))
		service.LogTaskConsumption(c, info)
		var user model.User
		require.NoError(t, db.First(&user, id).Error)
		assert.InDelta(t, tc.charge, initial-user.Quota, 1)
		task := &model.Task{
			TaskID: token.Key, UserId: id, ChannelId: id, Quota: info.PriceData.Quota, Group: "default",
			PrivateData: model.TaskPrivateData{TokenId: id, BillingSource: service.BillingSourceWallet},
		}
		require.NoError(t, db.Create(task).Error)
		require.True(t, service.RefundTaskQuota(context.Background(), task, "mock upstream failure"))
		require.NoError(t, db.First(&user, id).Error)
		require.NoError(t, db.First(token, id).Error)
		assert.Equal(t, initial, user.Quota)
		assert.Equal(t, initial, token.RemainQuota)
		common.CleanupBodyStorage(c)
	}
}
