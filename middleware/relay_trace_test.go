package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingRelayWriter struct {
	gin.ResponseWriter
}

func (w *failingRelayWriter) Write(p []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestRelayTracePreservesDownstreamWriteFailure(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	trace := common.NewRelayTrace(c)
	writer := &relayTraceWriter{ResponseWriter: &failingRelayWriter{ResponseWriter: c.Writer}, trace: trace}
	n, err := writer.Write([]byte("not delivered"))
	assert.Zero(t, n)
	require.ErrorIs(t, err, io.ErrClosedPipe)
	assert.Equal(t, "closed", trace.Snapshot()["downstream_write_error"])
	assert.NotContains(t, trace.Snapshot(), "first_write_return_ms")
}

func TestRelayTracePreserves503AndExistingRequestHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestId())
	const body = `{"error":{"message":"original message","type":"upstream_error","code":"upstream_service_unavailable"}}`
	router.POST("/v1/messages", func(c *gin.Context) {
		c.Data(http.StatusServiceUnavailable, "application/json", []byte(body))
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/messages", nil))
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Equal(t, body, recorder.Body.String())
	assert.NotEmpty(t, recorder.Header().Get(common.RequestIdKey))
	assert.Empty(t, recorder.Header().Get("X-Should-Retry"))
}
