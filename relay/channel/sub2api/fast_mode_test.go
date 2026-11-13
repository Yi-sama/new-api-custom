package sub2api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFastModeResponsesRoundTrip(t *testing.T) {
	previousGroups := setting.FastModeAllowedGroups2JSONString()
	t.Cleanup(func() { require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(previousGroups)) })
	require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(`["gpt-paid"]`))
	service.InitHttpClient()
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = previousTimeout })

	for _, tc := range []struct {
		name    string
		stream  bool
		enabled bool
		tier    string
		trusted bool
		marker  bool
		ratio   float64
	}{
		{"priority JSON", false, true, "priority", false, false, 1.5},
		{"trusted OAuth SSE", true, true, "default", true, true, 1.5},
		{"untrusted downgrade SSE", true, true, "default", false, true, 1},
		{"ordinary JSON", false, false, "default", true, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := make(chan string, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				requests <- string(body)
				if tc.marker {
					w.Header().Set("X-Sub2API-Fast-Protocol", "codex-oauth-priority-v1")
				}
				data := `{"id":"resp-fast-test","object":"response","model":"gpt-6.1-sol","status":"completed","service_tier":"` + tc.tier + `","output":[],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}`
				w.Header().Set("Content-Type", "application/json")
				if tc.stream {
					w.Header().Set("Content-Type", "text/event-stream")
					data = "data: " + `{"type":"response.completed","response":` + data + "}\n\ndata: [DONE]\n\n"
				}
				_, _ = io.WriteString(w, data)
			}))
			defer upstream.Close()

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			ctx.Request.Header.Set("Content-Type", "application/json")
			info := &relaycommon.RelayInfo{
				TokenFastMode: tc.enabled, UsingGroup: "gpt-paid", IsStream: tc.stream,
				RelayMode: relayconstant.RelayModeResponses, RelayFormat: types.RelayFormatOpenAIResponses,
				RequestURLPath: "/v1/responses", OriginModelName: "gpt-6.1-sol", StartTime: time.Now(),
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType: constant.ChannelTypeSub2API, ChannelBaseUrl: upstream.URL,
					UpstreamModelName: "gpt-6.1-sol",
					ChannelSetting:    dto.ChannelSettings{TrustSub2APIFastProtocol: tc.trusted},
				},
			}
			info.ConfigureFastMode()
			require.Equal(t, tc.enabled, info.FastMode)
			if info.FastMode {
				info.PriceData.AddOtherRatio("fast_mode", relaycommon.FastModeRatio)
			}
			body, err := relaycommon.RemoveDisabledFields(
				[]byte(`{"model":"gpt-6.1-sol","input":"hello","service_tier":"priority"}`),
				dto.ChannelOtherSettings{}, false,
			)
			require.NoError(t, err)
			adaptor := &Adaptor{}
			adaptor.Init(info)
			result, err := adaptor.DoRequest(ctx, info, strings.NewReader(string(body)))
			require.NoError(t, err)
			response, ok := result.(*http.Response)
			require.True(t, ok)
			expected := `{"model":"gpt-6.1-sol","input":"hello"}`
			if tc.enabled {
				expected = `{"model":"gpt-6.1-sol","input":"hello","service_tier":"priority"}`
			}
			assert.JSONEq(t, expected, <-requests)
			usage, apiErr := adaptor.DoResponse(ctx, response, info)
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			assert.Equal(t, tc.ratio, info.FastModeBillingRatio())

			log := service.GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 1, 0, 1)
			if tc.enabled {
				assert.Equal(t, true, log["fast_mode"])
				assert.Equal(t, "priority", log["service_tier_requested"])
				assert.Equal(t, tc.tier, log["service_tier"])
				assert.Equal(t, tc.ratio, log["fast_mode_ratio"])
			} else {
				assert.NotContains(t, log, "fast_mode")
				assert.NotContains(t, log, "service_tier_requested")
			}
		})
	}
}
