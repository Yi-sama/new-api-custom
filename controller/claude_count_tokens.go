package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// ClaudeCountTokens provides an unbilled local estimate when upstream counting is unavailable.
func ClaudeCountTokens(c *gin.Context) {
	request, err := helper.GetAndValidateClaudeRequest(c)
	if err != nil {
		status := http.StatusBadRequest
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		c.JSON(status, gin.H{
			"type":  "error",
			"error": gin.H{"type": "invalid_request_error", "message": err.Error()},
		})
		return
	}

	info := relaycommon.GenRelayInfoClaude(c, request)
	tokens, err := service.EstimateRequestToken(c, request.GetTokenCountMeta(), info, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"type":  "error",
			"error": gin.H{"type": "invalid_request_error", "message": err.Error()},
		})
		return
	}

	c.Header("X-Token-Count-Source", "local-estimate")
	c.JSON(http.StatusOK, gin.H{"input_tokens": tokens})
}
