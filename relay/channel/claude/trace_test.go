package claude_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type traceLogBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *traceLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *traceLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func TestClaudeRelayTraceObservesWireWithoutChangingIt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	const start = `{"type":"message_start","message":{"id":"msg_trace_fixture","type":"message","role":"assistant","model":"claude-test","content":[],"usage":{"input_tokens":5,"output_tokens":0}}}`
	const block = `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`
	const text = `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"synthetic-private-answer"}}`
	const blockStop = `{"type":"content_block_stop","index":0}`
	const delta = `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":3}}`
	const stop = `{"type":"message_stop"}`

	for _, test := range []struct {
		name      string
		events    []string
		interrupt bool
		complete  bool
		gateFirst bool
	}{
		{"complete", []string{start, block, text, blockStop, delta, stop}, false, true, false},
		{"delayed first packet", []string{start, block, text, blockStop, delta, stop}, false, true, true},
		{"clean EOF without message_stop", []string{start, block, text, blockStop, delta}, false, false, false},
		{"connection lost mid-message", []string{start, block, text}, true, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var log traceLogBuffer
			common.LogWriterMu.Lock()
			oldWriter, oldErrorWriter := gin.DefaultWriter, gin.DefaultErrorWriter
			gin.DefaultWriter, gin.DefaultErrorWriter = &log, &log
			common.LogWriterMu.Unlock()
			t.Cleanup(func() {
				common.LogWriterMu.Lock()
				gin.DefaultWriter, gin.DefaultErrorWriter = oldWriter, oldErrorWriter
				common.LogWriterMu.Unlock()
			})

			received := make(chan struct{})
			release := make(chan struct{})
			if !test.gateFirst {
				close(release)
			}
			var releaseOnce sync.Once
			if test.gateFirst {
				t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				close(received)
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("X-Request-Id", "req_trace_fixture")
				for _, event := range test.events {
					_, _ = io.WriteString(w, "data: "+event+"\n\n")
					w.(http.Flusher).Flush()
				}
				if test.interrupt {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						_ = conn.Close()
					}
				}
			}))
			defer upstream.Close()

			router := gin.New()
			router.Use(middleware.RequestId())
			handlerErr := make(chan error, 1)
			router.POST("/v1/messages", func(c *gin.Context) {
				info := &relaycommon.RelayInfo{
					RelayFormat: types.RelayFormatClaude,
					IsStream:    true,
					DisablePing: true,
					ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 54, UpstreamModelName: "claude-test"},
				}
				req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, upstream.URL, strings.NewReader("synthetic-private-prompt"))
				if err != nil {
					handlerErr <- err
					return
				}
				resp, err := channel.DoRequest(c, req, info)
				if err != nil {
					handlerErr <- err
					return
				}
				_, apiErr := claude.ClaudeStreamHandler(c, resp, info)
				if apiErr != nil {
					handlerErr <- apiErr
				} else {
					handlerErr <- nil
				}
			})
			gateway := httptest.NewServer(router)
			defer gateway.Close()

			type result struct {
				status int
				body   string
				err    error
			}
			done := make(chan result, 1)
			go func() {
				req, err := http.NewRequest(http.MethodPost, gateway.URL+"/v1/messages", nil)
				if err != nil {
					done <- result{err: err}
					return
				}
				req.Header.Set("X-Client-Request-Id", "3ae6da3c-aed7-4c25-9892-8fe47efef0b5")
				resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
				if err != nil {
					done <- result{err: err}
					return
				}
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				done <- result{status: resp.StatusCode, body: string(body), err: err}
			}()
			select {
			case <-received:
			case <-time.After(5 * time.Second):
				t.Fatal("synthetic upstream was not reached")
			}
			if test.gateFirst {
				select {
				case unexpected := <-done:
					t.Fatalf("request ended before upstream was released: %v", unexpected.err)
				default:
				}
				releaseOnce.Do(func() { close(release) })
			}
			var got result
			select {
			case got = <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("synthetic relay did not finish")
			}
			require.NoError(t, got.err)
			require.NoError(t, <-handlerErr)
			assert.Equal(t, http.StatusOK, got.status)
			assert.Contains(t, got.body, "synthetic-private-answer")
			assert.Equal(t, test.complete, strings.Contains(got.body, `"type":"message_stop"`),
				"observation must not add or remove terminal events")

			var snapshot map[string]any
			count := 0
			for _, line := range strings.Split(log.String(), "\n") {
				if _, data, ok := strings.Cut(line, "relay_trace "); ok {
					require.NoError(t, common.UnmarshalJsonStr(strings.TrimSpace(data), &snapshot))
					count++
				}
			}
			require.Equal(t, 1, count, "one summary per inbound request")
			assert.Equal(t, "3ae6da3c-aed7-4c25-9892-8fe47efef0b5", snapshot["client_request_id"])
			assert.Greater(t, snapshot["downstream_bytes_written"], float64(0))
			assert.Contains(t, snapshot, "first_flush_return_ms")
			attempts, ok := snapshot["attempts"].([]any)
			require.True(t, ok)
			require.Len(t, attempts, 1)
			attempt := attempts[0].(map[string]any)
			assert.Equal(t, "msg_trace_fixture", attempt["message_id"])
			assert.Equal(t, "req_trace_fixture", attempt["upstream_request_id"])
			assert.Equal(t, test.complete, attempt["message_stop_seen"])
			for _, phase := range []string{"upstream_start_ms", "request_written_ms", "first_response_byte_ms", "response_headers_ms", "first_body_read_ms", "first_event_ms", "first_text_ms", "body_close_ms", "stream_handler_end_ms"} {
				assert.Contains(t, attempt, phase)
			}
			if test.interrupt {
				assert.Equal(t, "unexpected_eof", attempt["body_read_error"])
			} else {
				assert.Contains(t, attempt, "body_eof_ms")
			}
			data, err := common.Marshal(snapshot)
			require.NoError(t, err)
			assert.NotContains(t, string(data), "synthetic-private-prompt")
			assert.NotContains(t, string(data), "synthetic-private-answer")
		})
	}
}
