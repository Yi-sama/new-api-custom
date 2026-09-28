package middleware

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTurnstileCheckResponseDecodesFailureDetails(t *testing.T) {
	var response turnstileCheckResponse
	err := common.Unmarshal([]byte(`{
		"success": false,
		"error-codes": ["timeout-or-duplicate"],
		"hostname": "api.yisama.asia",
		"action": "login"
	}`), &response)

	require.NoError(t, err)
	assert.False(t, response.Success)
	assert.Equal(t, []string{"timeout-or-duplicate"}, response.ErrorCodes)
	assert.Equal(t, "api.yisama.asia", response.Hostname)
	assert.Equal(t, "login", response.Action)
}

func TestIsPlainHTTPRequest(t *testing.T) {
	tests := []struct {
		name           string
		forwardedProto string
		tls            bool
		want           bool
	}{
		{name: "direct HTTP", want: true},
		{name: "proxied HTTP", forwardedProto: "http", want: true},
		{name: "proxied HTTPS", forwardedProto: "https", want: false},
		{name: "direct TLS", tls: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/user/login", nil)
			if tt.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req

			assert.Equal(t, tt.want, isPlainHTTPRequest(c))
		})
	}
}
