package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestQQRestoreRoutesExistAndRejectAnonymousAccess(t *testing.T) {
	t.Setenv("QQ_BENEFIT_BOT_SECRET", strings.Repeat("test-secret-", 4))
	engine := gin.New()
	SetApiRouter(engine)
	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/qq-benefits/bot/start"},
		{"POST", "/api/qq-benefits/bot/status"},
		{"POST", "/api/qq-benefits/bot/confirm"},
		{"GET", "/api/user/qq-binding"},
		{"POST", "/api/user/qq-binding/code"},
		{"GET", "/api/status-images/bot?kind=channel"},
		{"GET", "/api/status-images/bot?kind=sub"},
		{"GET", "/api/status-images/overview"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
			request.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(response, request)
			assert.Equal(t, http.StatusUnauthorized, response.Code)
		})
	}
}
