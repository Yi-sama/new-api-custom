package middleware

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

type turnstileCheckResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
	Hostname   string   `json:"hostname"`
	Action     string   `json:"action"`
}

func TurnstileCheck() gin.HandlerFunc {
	return turnstileCheck(false)
}

// TurnstileCheckForPasswordLogin allows password login over plain HTTP so the
// login page remains usable when the browser cannot load the Turnstile widget.
func TurnstileCheckForPasswordLogin() gin.HandlerFunc {
	return turnstileCheck(true)
}

func turnstileCheck(skipOnPlainHTTP bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if common.TurnstileCheckEnabled && !(skipOnPlainHTTP && isPlainHTTPRequest(c)) {
			response := c.Query("turnstile")
			if response == "" {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "Turnstile token 为空",
				})
				c.Abort()
				return
			}
			rawRes, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", url.Values{
				"secret":   {common.TurnstileSecretKey},
				"response": {response},
				"remoteip": {c.ClientIP()},
			})
			if err != nil {
				common.SysLog(err.Error())
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				c.Abort()
				return
			}
			defer rawRes.Body.Close()
			var res turnstileCheckResponse
			err = common.DecodeJson(rawRes.Body, &res)
			if err != nil {
				common.SysLog(err.Error())
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				c.Abort()
				return
			}
			if !res.Success {
				common.SysLog(fmt.Sprintf(
					"Turnstile verification failed: error_codes=%v hostname=%q action=%q client_ip=%s",
					res.ErrorCodes,
					res.Hostname,
					res.Action,
					c.ClientIP(),
				))
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "Turnstile 校验失败，请刷新重试！",
				})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

func isPlainHTTPRequest(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return false
	}
	forwardedProto := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")))
	if forwardedProto != "" {
		return forwardedProto == "http"
	}
	return true
}
