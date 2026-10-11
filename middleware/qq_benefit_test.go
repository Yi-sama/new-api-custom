package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestQQBenefitBotSignatureBindsBodyPathMethodAndTimestamp(t *testing.T) {
	secret := strings.Repeat("test-secret-", 4)
	t.Setenv("QQ_BENEFIT_BOT_SECRET", secret)
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	body := `{"qq":"100001","group_id":"1128511890"}`
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stamp + "\nPOST\n/api/qq-benefits/bot/status\n" + body))
	signature := hex.EncodeToString(mac.Sum(nil))
	router := gin.New()
	router.Any("/api/qq-benefits/bot/:action", QQBenefitBotAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	for _, tc := range []struct {
		name, method, path, body, stamp, signature string
		want                                       int
	}{
		{"valid", "POST", "/api/qq-benefits/bot/status", body, stamp, signature, 204},
		{"changed QQ", "POST", "/api/qq-benefits/bot/status", strings.ReplaceAll(body, "100001", "100002"), stamp, signature, 401},
		{"changed path", "POST", "/api/qq-benefits/bot/confirm", body, stamp, signature, 401},
		{"changed method", "GET", "/api/qq-benefits/bot/status", body, stamp, signature, 401},
		{"stale timestamp", "POST", "/api/qq-benefits/bot/status", body, "1", signature, 401},
		{"missing signature", "POST", "/api/qq-benefits/bot/status", body, stamp, "", 401},
		{"oversized body", "POST", "/api/qq-benefits/bot/status", strings.Repeat("x", 4097), stamp, signature, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("X-QQ-Benefit-Time", tc.stamp)
			request.Header.Set("X-QQ-Benefit-Signature", tc.signature)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assert.Equal(t, tc.want, response.Code)
		})
	}
}
