package common

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayTraceKeepsAttemptsSeparateAndDoesNotLogSecrets(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	c.Request.Header.Set("X-Client-Request-Id", "sk-private-header-value")
	trace := NewRelayTrace(c)
	first := trace.BeginAttempt(54)
	first.Mark("response_headers_ms")
	first.RecordError("request_error", errors.New("private response body or api key"))
	second := trace.BeginAttempt(53)
	second.Mark("message_stop_ms")

	snapshot := trace.Snapshot()
	attempts, ok := snapshot["attempts"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, attempts, 2)
	assert.Equal(t, 54, attempts[0]["channel_id"])
	assert.NotContains(t, attempts[0], "message_stop_ms")
	assert.Equal(t, 53, attempts[1]["channel_id"])
	assert.NotContains(t, attempts[1], "request_error")
	assert.Same(t, second, trace.CurrentAttempt())
	data, err := Marshal(snapshot)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "sk-private-header-value")
	assert.NotContains(t, string(data), "private response body")
	assert.Contains(t, string(data), "sha256:")
	assert.Equal(t, "msg_011Synthetic", RelayTraceID("msg_011Synthetic"))
	assert.Equal(t, "3ae6da3c-aed7-4c25-9892-8fe47efef0b5", RelayTraceID("3ae6da3c-aed7-4c25-9892-8fe47efef0b5"))
}
