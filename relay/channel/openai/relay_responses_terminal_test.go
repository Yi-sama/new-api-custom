package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOaiResponsesStreamHandlerTerminatesAtTerminalEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	for _, tc := range []struct {
		event  string
		failed bool
	}{
		{event: "response.completed"},
		{event: "response.done"},
		{event: "response.failed", failed: true},
		{event: "response.incomplete", failed: true},
		{event: "response.cancelled", failed: true},
		{event: "response.canceled", failed: true},
	} {
		t.Run(tc.event, func(t *testing.T) {
			response := map[string]any{
				"id":     "resp_terminal",
				"status": "completed",
				"usage":  map[string]any{"input_tokens": 3, "output_tokens": 2, "total_tokens": 5},
			}
			if tc.failed {
				response["status"] = strings.TrimPrefix(tc.event, "response.")
				response["error"] = map[string]any{"code": "server_error", "message": "upstream overloaded"}
			}
			payload, err := common.Marshal(map[string]any{
				"type":     tc.event,
				"response": response,
			})
			require.NoError(t, err)
			reader, writer := io.Pipe()
			t.Cleanup(func() {
				_ = writer.Close()
				_ = reader.Close()
			})
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			c.Set(common.RequestIdKey, "responses-terminal-test")
			info := &relaycommon.RelayInfo{
				OriginModelName: "gpt-5.1",
				DisablePing:     true,
				ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.1"},
			}
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       reader,
			}
			var usage *dto.Usage
			var apiErr *types.NewAPIError
			done := make(chan struct{})
			go func() {
				defer close(done)
				usage, apiErr = OaiResponsesStreamHandler(c, info, resp)
			}()
			_, err = io.WriteString(writer, "data: "+string(payload)+"\n\n")
			require.NoError(t, err)

			returnedBeforeEOF := false
			select {
			case <-done:
				returnedBeforeEOF = true
			case <-time.After(time.Second):
				_ = writer.Close()
				<-done
			}
			require.True(t, returnedBeforeEOF, "terminal event must end the relay without waiting for upstream EOF")
			require.Nil(t, apiErr, "a forwarded terminal must not cause generic JSON or a gateway replay")
			require.NotNil(t, info.StreamStatus)
			assert.Equal(t, tc.failed, info.StreamStatus.HasErrors())
			assert.Equal(t, 1, strings.Count(recorder.Body.String(), "data: "))
			assert.Contains(t, recorder.Body.String(), "data: "+string(payload)+"\n\n")
			if !tc.failed {
				require.NotNil(t, usage)
				assert.Equal(t, 3, usage.PromptTokens)
				assert.Equal(t, 2, usage.CompletionTokens)
			}
		})
	}
}
