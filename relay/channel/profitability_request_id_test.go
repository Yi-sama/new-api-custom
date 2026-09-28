package channel

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDoRequestCapturesSub2APIRequestID(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Client-Request-ID", "sub-request-42")
		w.Header().Set("X-Request-ID", "provider-request-42")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	request, err := http.NewRequest(http.MethodPost, upstream.URL, strings.NewReader("{}"))
	require.NoError(t, err)
	response, err := DoRequest(ctx, request, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, "client:sub-request-42", ctx.GetString(common.UpstreamRequestIdKey))
}
