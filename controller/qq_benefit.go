package controller

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type qqBenefitBotInput struct {
	QQ        string `json:"qq"`
	GroupID   string `json:"group_id"`
	MessageID string `json:"message_id"`
	Code      string `json:"code"`
}

func qqBenefitEnabled() bool {
	return len(os.Getenv("QQ_BENEFIT_BOT_SECRET")) >= 32
}

func qqBenefitError(c *gin.Context, err error) {
	code := "QQ_BENEFIT_UNAVAILABLE"
	for _, known := range []error{
		model.ErrQQBenefitInvalid, model.ErrQQBenefitNeedStart, model.ErrQQBenefitAlreadyBound,
		model.ErrQQBenefitInvalidCode, model.ErrQQBenefitSession,
	} {
		if errors.Is(err, known) {
			code = known.Error()
			break
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": false, "code": code, "message": code})
}

func qqBenefitReadBotInput(c *gin.Context) (*qqBenefitBotInput, bool) {
	var input qqBenefitBotInput
	group := os.Getenv("QQ_BENEFIT_GROUP_ID")
	if group == "" {
		group = "1128511890"
	}
	if err := c.ShouldBindJSON(&input); err != nil || !model.ValidQQBenefitIdentity(input.QQ, input.GroupID) {
		qqBenefitError(c, model.ErrQQBenefitInvalid)
		return nil, false
	}
	for _, allowed := range strings.Split(group, ",") {
		if input.GroupID == strings.TrimSpace(allowed) {
			return &input, true
		}
	}
	qqBenefitError(c, model.ErrQQBenefitInvalid)
	return nil, false
}

func GetQQBenefitStatus(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	binding, err := model.GetQQBenefitBinding("", identity.UserID)
	if err != nil {
		qqBenefitError(c, err)
		return
	}
	data := gin.H{"enabled": qqBenefitEnabled(), "bound": binding != nil, "reward_usd": 1}
	if binding != nil {
		data["qq"] = binding.QQ
		data["reward_granted"] = true
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func CreateQQBenefitCode(c *gin.Context) {
	if !qqBenefitEnabled() {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var input struct {
		QQ string `json:"qq"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		qqBenefitError(c, model.ErrQQBenefitInvalid)
		return
	}
	code, expiresAt, err := model.CreateQQBenefitCode(identity.UserID, identity.SessionID, input.QQ)
	if err != nil {
		qqBenefitError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"code": code, "qq": input.QQ, "expires_at": expiresAt.Unix(), "reward_usd": 1,
	}})
}

func StartQQBenefit(c *gin.Context) {
	input, ok := qqBenefitReadBotInput(c)
	if !ok {
		return
	}
	request, bound, err := model.StartQQBenefit(input.QQ, input.GroupID, input.MessageID)
	if err != nil {
		qqBenefitError(c, err)
		return
	}
	data := gin.H{"bound": bound, "reward_granted": bound}
	if bound {
		binding, err := model.GetQQBenefitBinding(input.QQ, 0)
		if err != nil {
			qqBenefitError(c, err)
			return
		}
		if binding != nil {
			data["awarded_at"] = binding.AwardedAt.Unix()
		}
	}
	if request != nil {
		data["expires_at"] = request.ExpiresAt.Unix()
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func GetQQBenefitBotStatus(c *gin.Context) {
	input, ok := qqBenefitReadBotInput(c)
	if !ok {
		return
	}
	allowed, err := model.QQBenefitCommandsAllowed(input.QQ)
	if err != nil {
		qqBenefitError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"bound": allowed}})
}

func ConfirmQQBenefit(c *gin.Context) {
	input, ok := qqBenefitReadBotInput(c)
	if !ok {
		return
	}
	binding, granted, err := model.ConfirmQQBenefit(input.QQ, input.GroupID, input.Code)
	if err != nil {
		qqBenefitError(c, err)
		return
	}
	allowed, err := model.QQBenefitCommandsAllowed(input.QQ)
	if err != nil || !allowed || binding == nil {
		qqBenefitError(c, model.ErrQQBenefitSession)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"bound": true, "reward_granted": granted, "reward_usd": 1,
	}})
}
