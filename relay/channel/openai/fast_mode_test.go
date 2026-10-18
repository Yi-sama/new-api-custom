package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenFastModeRoundTrip(t *testing.T) {
	service.InitHttpClient()
	timeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = timeout })
	for _, tc := range []struct {
		name      string
		stream    bool
		responses bool
		tier      string
		trusted   bool
		marker    bool
	}{
		{"responses JSON", false, true, "priority", false, false},
		{"responses SSE downgrade", true, true, "default", false, false},
		{"chat JSON downgrade", false, false, "default", false, false},
		{"chat SSE", true, false, "priority", false, false},
		{"trusted OAuth responses JSON", false, true, "default", true, true},
		{"trusted OAuth responses SSE", true, true, "default", true, true},
		{"trusted OAuth chat JSON", false, false, "default", true, true},
		{"trusted OAuth chat SSE", true, false, "default", true, true},
		{"untrusted upstream marker", true, true, "default", false, true},
		{"client marker cannot select protocol", true, true, "default", true, false},
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
				w.Header().Set("Content-Type", "application/json")
				data := `{"id":"chat-test","object":"chat.completion","model":"gpt-6.1-sol","service_tier":"` + tc.tier + `","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`
				if tc.responses {
					data = `{"id":"resp-test","object":"response","model":"gpt-6.1-sol","status":"completed","service_tier":"` + tc.tier + `","output":[],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}`
				}
				if tc.stream {
					w.Header().Set("Content-Type", "text/event-stream")
					if tc.responses {
						data = `{"type":"response.completed","response":` + data + `}`
					} else {
						data = strings.ReplaceAll(data, `"message":`, `"delta":`)
					}
					data = "data: " + data + "\n\ndata: [DONE]\n\n"
				}
				_, _ = io.WriteString(w, data)
			}))
			defer upstream.Close()
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			path := "/v1/chat/completions"
			mode := relayconstant.RelayModeChatCompletions
			format := types.RelayFormatOpenAI
			if tc.responses {
				path, mode, format = "/v1/responses", relayconstant.RelayModeResponses, types.RelayFormatOpenAIResponses
			}
			ctx.Request = httptest.NewRequest(http.MethodPost, path, nil)
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Request.Header.Set("X-Sub2API-Fast-Protocol", "codex-oauth-priority-v1")
			info := &relaycommon.RelayInfo{
				FastMode: true, IsStream: tc.stream, RelayMode: mode, RelayFormat: format,
				RequestURLPath: path, OriginModelName: "gpt-6.1-sol",
				ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI,
					ChannelSetting: dto.ChannelSettings{TrustSub2APIFastProtocol: tc.trusted},
					ChannelBaseUrl: upstream.URL, UpstreamModelName: "gpt-6.1-sol"},
			}
			info.PriceData.AddOtherRatio("fast_mode", 1.5)
			adaptor := &Adaptor{}
			adaptor.Init(info)
			resp, err := channel.DoApiRequest(adaptor, ctx, info, strings.NewReader(`{"model":"gpt-6.1-sol","input":"hello","service_tier":"flex"}`))
			require.NoError(t, err)
			assert.JSONEq(t, `{"model":"gpt-6.1-sol","input":"hello","service_tier":"priority"}`, <-requests)
			usage, apiErr := adaptor.DoResponse(ctx, resp, info)
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			assert.Equal(t, tc.tier, info.FastModeTier)
			expected := 1.5
			if tc.tier == "default" && !(tc.trusted && tc.marker) {
				expected = 1
			}
			assert.Equal(t, expected, info.PriceData.OtherRatios()["fast_mode"])
			assert.Contains(t, recorder.Body.String(), tc.tier)
		})
	}
}
