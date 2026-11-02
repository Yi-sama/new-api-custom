package claude

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeStreamHandlerDoesNotBillEmptyStreams(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 5
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	for _, test := range []struct {
		name     string
		body     string
		canceled bool
	}{
		{"empty upstream EOF", "", false},
		{"heartbeat only", "event: ping\ndata: {\"type\":\"ping\"}\n\n", false},
		{"client canceled before first event", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			if test.canceled {
				requestCtx, cancel := context.WithCancel(ctx.Request.Context())
				cancel()
				ctx.Request = ctx.Request.WithContext(requestCtx)
			}
			info := &relaycommon.RelayInfo{
				RelayFormat: types.RelayFormatClaude,
				ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-opus-5-5"},
			}
			info.SetEstimatePromptTokens(7582912)
			response := &http.Response{Body: io.NopCloser(strings.NewReader(test.body)), Header: make(http.Header)}

			usage, err := ClaudeStreamHandler(ctx, response, info)

			require.NotNil(t, err)
			assert.Equal(t, types.ErrorCodeEmptyResponse, err.GetErrorCode())
			assert.Equal(t, http.StatusBadGateway, err.StatusCode)
			assert.True(t, types.IsSkipRetryError(err))
			assert.Nil(t, usage, "an empty stream must not become estimated billable input")
		})
	}
}

func TestClaudeStreamHandlerPreservesReportedInputWithoutOutput(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 5
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	body := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"model\":\"claude-opus-5-5\",\"usage\":{\"input_tokens\":33,\"output_tokens\":0}}}\n\n"
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-opus-5-5"},
	}
	info.SetEstimatePromptTokens(7582912)

	usage, err := ClaudeStreamHandler(ctx, &http.Response{Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, info)

	require.Nil(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, 33, usage.PromptTokens)
	assert.Equal(t, 0, usage.CompletionTokens)
}
