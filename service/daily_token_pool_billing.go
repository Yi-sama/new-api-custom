package service

import (
	"errors"
	"net/http"
	"slices"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

const BillingSourceDailyTokenPool = "daily_token_pool"

// Resolve free funding before wallet pre-consumption and recheck every selected
// channel. A free request must never fall back to paid funding during retries.
func PrepareDailyTokenPoolBilling(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	config, err := model.GetDailyTokenPoolConfig()
	if err != nil {
		return types.NewErrorWithStatusCode(errors.New("shared token pool settings are unavailable"), types.ErrorCodeUpdateDataError, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if group == "" {
		group = info.TokenGroup
	}
	freeGroup := config.FreeUsage && slices.Contains(config.Groups, group)
	freeTokenGroup := config.FreeUsage && slices.Contains(config.Groups, info.TokenGroup)
	if !freeGroup && !freeTokenGroup && !info.DailyTokenPoolFree {
		return nil
	}
	if !freeGroup || group != info.TokenGroup || info.Billing != nil || info.FinalPreConsumedQuota != 0 {
		return types.NewErrorWithStatusCode(errors.New("free shared token pool requires its dedicated token group and configured channel; paid fallback is disabled"), types.ErrorCodeInvalidRequest, http.StatusForbidden, types.ErrOptionWithSkipRetry())
	}
	info.DailyTokenPoolFree = true
	info.BillingSource = BillingSourceDailyTokenPool
	if !config.Enabled {
		return types.NewErrorWithStatusCode(errors.New("daily shared token pool is disabled"), types.ErrorCodeInvalidRequest, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	if !config.Matches(group, common.GetContextKeyInt(c, constant.ContextKeyChannelId)) {
		return types.NewErrorWithStatusCode(errors.New("selected channel is outside the free shared token pool; paid fallback is disabled"), types.ErrorCodeInvalidRequest, http.StatusForbidden, types.ErrOptionWithSkipRetry())
	}
	return nil
}
