package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// This credential has no model, option, user-management or other API access.
func QQBenefitBotAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := os.Getenv("QQ_BENEFIT_BOT_SECRET")
		if len(secret) < 32 {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		stamp := c.GetHeader("X-QQ-Benefit-Time")
		seconds, err := strconv.ParseInt(stamp, 10, 64)
		now := time.Now().Unix()
		if err != nil || seconds < now-60 || seconds > now+60 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 4097))
		if err != nil || len(body) > 4096 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		supplied, err := hex.DecodeString(c.GetHeader("X-QQ-Benefit-Signature"))
		digest := hmac.New(sha256.New, []byte(secret))
		digest.Write([]byte(stamp + "\n" + c.Request.Method + "\n" + c.Request.URL.Path + "\n"))
		digest.Write(body)
		if err != nil || !hmac.Equal(supplied, digest.Sum(nil)) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}
