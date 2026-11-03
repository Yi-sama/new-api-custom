package router

import (
	"errors"
	"fmt"
	"html"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// The desktop client shares website accounts and session policy, not its CAPTCHA flow.
func setAssistantAuthRoutes(api *gin.RouterGroup) {
	auth := api.Group("/assistant/auth")
	auth.Use(middleware.DisableCache(), middleware.AssistantAuthRateLimit())
	bodyLimit := middleware.AnonymousRequestBodyLimit()
	auth.GET("/options", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"password_login_enabled":     common.PasswordLoginEnabled,
				"register_enabled":           common.RegisterEnabled && common.PasswordRegisterEnabled,
				"email_verification_enabled": common.EmailVerificationEnabled,
			},
		})
	})
	auth.POST("/login", middleware.CriticalRateLimit(), bodyLimit, controller.Login)
	auth.POST("/login/2fa", middleware.CriticalRateLimit(), bodyLimit, controller.Verify2FALogin)
	auth.POST("/register", middleware.CriticalRateLimit(), bodyLimit, controller.Register)
	auth.GET("/verification", middleware.EmailVerificationRateLimit(), controller.SendEmailVerification)
	auth.GET("/password-reset/verification", middleware.AssistantPasswordResetVerificationRateLimit(),
		sendAssistantPasswordResetVerification)
	auth.POST("/password-reset", middleware.CriticalRateLimit(), bodyLimit, resetAssistantPassword)
}

func sendAssistantPasswordResetVerification(c *gin.Context) {
	email := model.NormalizeEmail(c.Query("email"))
	if err := common.Validate.Var(email, "required,email"); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	code, retryAfter, err := model.CreateAssistantPasswordReset(email)
	if err != nil {
		common.SysError("failed to prepare assistant password reset")
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "账号服务暂时不可用，请稍后重试。"})
		return
	}
	if retryAfter > 0 {
		c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"message": fmt.Sprintf("发送过于频繁，请等待 %d 秒后再试", retryAfter),
		})
		return
	}

	// SMTP is asynchronous so account existence is not exposed by mail latency.
	if code != "" {
		subject := fmt.Sprintf("%s密码重置验证码", common.SystemName)
		content := fmt.Sprintf("<p>您好，你正在进行%s密码重置。</p>"+
			"<p>您的验证码为: <strong>%s</strong></p>"+
			"<p>验证码 10 分钟内有效，如果不是本人操作，请忽略。</p>",
			html.EscapeString(common.SystemName), code)
		go func() {
			if err := common.SendEmail(subject, email, content); err != nil {
				common.SysError("failed to send assistant password reset email")
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "如果邮箱已注册，验证码将发送到邮箱。",
	})
}

type assistantPasswordResetRequest struct {
	Email            string `json:"email"`
	VerificationCode string `json:"verification_code"`
	Password         string `json:"password"`
}

func resetAssistantPassword(c *gin.Context) {
	var req assistantPasswordResetRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	req.Email = model.NormalizeEmail(req.Email)
	if common.Validate.Var(req.Email, "required,email") != nil ||
		len(req.VerificationCode) != 6 ||
		common.Validate.Var(req.Password, "required,min=8,max=20") != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	for _, digit := range req.VerificationCode {
		if digit < '0' || digit > '9' {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
	}
	if err := model.ResetAssistantPassword(req.Email, req.VerificationCode, req.Password); err != nil {
		if errors.Is(err, model.ErrAssistantPasswordResetInvalid) {
			common.ApiErrorI18n(c, i18n.MsgUserVerificationCodeError)
			return
		}
		common.SysError("failed to complete assistant password reset")
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "账号服务暂时不可用，请稍后重试。"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "密码已重置，请使用新密码登录。",
	})
}
