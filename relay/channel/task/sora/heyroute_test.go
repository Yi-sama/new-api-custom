package sora

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configureHeyRouteTestPrices(t *testing.T) {
	t.Helper()
	before := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(before)) })
	prices := map[string]float64{
		"grok-video":                   0.05,
		"grok-imagine-video":           0.25,
		"grok-imagine-video-480p":      0.25,
		"grok-imagine-video-720p":      0.35,
		"grok-imagine-video-1.5":       0.375,
		"grok-imagine-video-1.5-480p":  0.375,
		"grok-imagine-video-1.5-720p":  0.65625,
		"grok-imagine-video-1.5-1080p": 1.171875,
		"minimax-h3-original-768p":     0.175,
		"minimax-h3-original-1080p":    0.2625,
		"minimax-h3-original-cf-2k":    0.2625,
		"seedance-2.5":                 1.9125,
		"seedance-2.5-480p":            1.1475,
		"seedance-2.5-720p":            1.9125,
		"seedance-2.5-1080p":           4.9725,
	}
	data, err := common.Marshal(prices)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(data)))
}

func TestHeyRouteVideoCostAndForwardedParameters(t *testing.T) {
	configureHeyRouteTestPrices(t)
	for _, tc := range []struct {
		name       string
		model      string
		parameters map[string]any
		seconds    int
		resolution string
		unitPrice  float64
	}{
		{"grok", "grok-video", map[string]any{"seconds": 10}, 10, "", 0.05},
		{"grok480", "grok-imagine-video", map[string]any{"seconds": "8"}, 8, "480p", 0.25},
		{"grok720", "grok-imagine-video", map[string]any{"duration": "8", "resolution": "720p"}, 8, "720p", 0.35},
		{"grok1080capped", "grok-imagine-video", map[string]any{"seconds": 8, "resolution": "1080p"}, 8, "720p", 0.35},
		{"grokDefault", "grok-imagine-video", nil, 8, "480p", 0.25},
		{"grok15Default", "grok-imagine-video-1.5", nil, 8, "480p", 0.375},
		{"grok15_720", "grok-imagine-video-1.5", map[string]any{"seconds": 6, "resolution": "720p"}, 6, "720p", 0.65625},
		{"grok15_1080", "grok-imagine-video-1.5", map[string]any{"seconds": 6, "resolution": "1080p"}, 6, "1080p", 1.171875},
		{"minimax768", "minimax-h3-original-768p", nil, 4, "", 0.175},
		{"minimax1080", "minimax-h3-original-1080p", map[string]any{"seconds": 15, "size": "1792x1024"}, 15, "", 0.2625},
		{"minimax2k", "minimax-h3-original-cf-2k", map[string]any{"seconds": 8}, 8, "", 0.2625},
		{"seedance480", "seedance-2.5", map[string]any{"seconds": 8, "resolution": "480p"}, 8, "480p", 1.1475},
		{"seedanceDefault", "seedance-2.5", nil, 4, "720p", 1.9125},
		{"seedance1080", "seedance-2.5", map[string]any{"seconds": 6, "resolution": "1080p"}, 6, "1080p", 4.9725},
		{"seedanceExtend", "seedance-2.5", map[string]any{"seconds": 8, "resolution": "480p", "omni_reference_task_type": "extend"}, 8, "720p", 1.9125},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]any{"model": tc.model, "prompt": "test", "input_reference": []string{"https://example.com/frame.png"}}
			for k, v := range tc.parameters {
				fields[k] = v
			}
			payload, err := common.Marshal(fields)
			require.NoError(t, err)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(payload))
			c.Request.Header.Set("Content-Type", "application/json")
			defer common.CleanupBodyStorage(c)
			info := &relaycommon.RelayInfo{
				OriginModelName: tc.model,
				ChannelMeta:     &relaycommon.ChannelMeta{ChannelBaseUrl: "https://cn.heyroute.ai", UpstreamModelName: tc.model},
				TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			}
			a := &TaskAdaptor{}
			a.Init(info)
			require.Nil(t, a.ValidateRequestAndSetAction(c, info))
			basePrice, ok := ratio_setting.GetModelPrice(tc.model, false)
			require.True(t, ok)
			info.PriceData.ModelPrice = basePrice
			for name, ratio := range a.EstimateBilling(c, info) {
				info.PriceData.AddOtherRatio(name, ratio)
			}
			actual := common.QuotaFromFloat(info.PriceData.ApplyOtherRatiosToFloat(basePrice * common.QuotaPerUnit))
			expected := common.QuotaRound(tc.unitPrice * float64(tc.seconds) * common.QuotaPerUnit)
			assert.InDelta(t, expected, actual, 1, "quota rounding may differ by at most one internal quota unit")

			body, err := a.BuildRequestBody(c, info)
			require.NoError(t, err)
			sent, err := io.ReadAll(body)
			require.NoError(t, err)
			var forwarded map[string]any
			require.NoError(t, common.Unmarshal(sent, &forwarded))
			assert.Equal(t, tc.model, forwarded["model"])
			assert.Equal(t, tc.resolution, c.MustGet(heyRouteBillingKey).(heyRouteBilling).Resolution)
			assert.Equal(t, tc.seconds, c.MustGet(heyRouteBillingKey).(heyRouteBilling).Seconds)
			assert.NotContains(t, forwarded, "duration")
			assert.Equal(t, fields["input_reference"], []string{forwarded["input_reference"].([]any)[0].(string)})
		})
	}
}

func TestHeyRouteRejectsUnpriceableVideoRequests(t *testing.T) {
	configureHeyRouteTestPrices(t)
	for _, payload := range []string{
		`{"model":"grok-video","prompt":"test"}`,
		`{"model":"grok-video","prompt":"test","seconds":8}`,
		`{"model":"grok-imagine-video","prompt":"test","seconds":0}`,
		`{"model":"grok-imagine-video","prompt":"test","seconds":-1}`,
		`{"model":"grok-imagine-video","prompt":"test","seconds":1.5}`,
		`{"model":"grok-imagine-video","prompt":"test","seconds":"18446744073709551615"}`,
		`{"model":"grok-imagine-video","prompt":"test","seconds":8,"duration":10}`,
		`{"model":"grok-imagine-video","prompt":"test","resolution":"2k"}`,
		`{"model":"seedance-2.5","prompt":"test","seconds":31}`,
		`{"model":"seedance-2.5","prompt":"test","omni_reference_task_type":"edit"}`,
		`{"model":"seedance-2.5","prompt":"test","omni_reference_task_type":"extend"}`,
		`{"model":"seedance-2-0-pro-260801","prompt":"test","seconds":15}`,
	} {
		t.Run(payload, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(payload))
			c.Request.Header.Set("Content-Type", "application/json")
			defer common.CleanupBodyStorage(c)
			info := &relaycommon.RelayInfo{
				ChannelMeta:   &relaycommon.ChannelMeta{ChannelBaseUrl: "https://cn.heyroute.ai"},
				TaskRelayInfo: &relaycommon.TaskRelayInfo{},
			}
			a := &TaskAdaptor{}
			a.Init(info)
			taskErr := a.ValidateRequestAndSetAction(c, info)
			require.NotNil(t, taskErr)
			assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
			_, exists := c.Get(heyRouteBillingKey)
			assert.False(t, exists)
		})
	}
}
