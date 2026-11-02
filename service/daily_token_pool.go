package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
)

func dailyPoolOutputLimit(request dto.Request) (int64, error) {
	defaultLimit := uint(4096)
	var limit *uint
	multiplier := int64(1)
	switch request := request.(type) {
	case *dto.GeneralOpenAIRequest:
		if request.MaxCompletionTokens != nil {
			limit = request.MaxCompletionTokens
		} else {
			if request.MaxTokens == nil {
				request.MaxTokens = &defaultLimit
			}
			limit = request.MaxTokens
		}
		if request.N != nil {
			if *request.N < 1 || *request.N > 128 {
				return 0, errors.New("shared token pool requires n between 1 and 128")
			}
			multiplier = int64(*request.N)
		}
	case *dto.OpenAIResponsesRequest:
		if request.MaxOutputTokens == nil {
			request.MaxOutputTokens = &defaultLimit
		}
		limit = request.MaxOutputTokens
	case *dto.ClaudeRequest:
		if request.MaxTokens == nil && request.MaxTokensToSample != nil {
			request.MaxTokens = request.MaxTokensToSample
		}
		if request.MaxTokens == nil {
			request.MaxTokens = &defaultLimit
		}
		limit = request.MaxTokens
	case *dto.GeminiChatRequest:
		if request.GenerationConfig.MaxOutputTokens == nil {
			request.GenerationConfig.MaxOutputTokens = &defaultLimit
		}
		limit = request.GenerationConfig.MaxOutputTokens
		if request.GenerationConfig.CandidateCount != nil {
			if *request.GenerationConfig.CandidateCount < 1 || *request.GenerationConfig.CandidateCount > 128 {
				return 0, errors.New("shared token pool requires candidateCount between 1 and 128")
			}
			multiplier = int64(*request.GenerationConfig.CandidateCount)
		}
	case *dto.EmbeddingRequest, *dto.GeminiEmbeddingRequest, *dto.GeminiBatchEmbeddingRequest, *dto.RerankRequest:
		return 0, nil
	default:
		return 0, errors.New("this request format is not supported by the daily token pool")
	}
	if limit == nil || uint64(*limit) > uint64(model.MaxDailyTokenPoolLimit/128) {
		return 0, errors.New("invalid output token limit for shared token pool")
	}
	return int64(*limit) * multiplier, nil
}

func dailyPoolToolOverrideBudget(overrides map[string]interface{}) (int64, error) {
	if len(overrides) == 0 {
		return 0, nil
	}
	var toolValues []interface{}
	for key, value := range overrides {
		switch key {
		case "tools":
			toolValues = append(toolValues, value)
		case "tool_choice":
		case "operations":
			operations, ok := extractParamOperations(value)
			if !ok {
				return 0, errors.New("invalid shared token pool tool override operations")
			}
			for _, raw := range operations {
				op, ok := raw.(map[string]interface{})
				if !ok {
					return 0, errors.New("invalid shared token pool tool override operation")
				}
				path, _ := op["path"].(string)
				mode, _ := op["mode"].(string)
				if (path != "tools" && path != "tool_choice") || (mode != "set" && mode != "delete") {
					return 0, errors.New("shared token pool only supports literal tools and tool_choice overrides")
				}
				for field := range op {
					switch field {
					case "path", "mode", "value", "keep_origin":
					case "description":
						if _, ok := op[field].(string); !ok {
							return 0, errors.New("shared token pool tool override description must be a string")
						}
					case "logic":
						logic, ok := op[field].(string)
						if !ok || (logic != "" && strings.ToUpper(logic) != "AND" && strings.ToUpper(logic) != "OR") {
							return 0, errors.New("invalid shared token pool tool override condition logic")
						}
					case "conditions":
						if err := validateDailyPoolToolOverrideConditions(op[field]); err != nil {
							return 0, err
						}
					default:
						return 0, errors.New("shared token pool tool overrides must be literal operations")
					}
				}
				if path == "tools" && mode == "set" {
					toolValues = append(toolValues, op["value"])
				}
			}
		default:
			return 0, errors.New("shared token pool only supports tools and tool_choice channel parameter overrides")
		}
	}
	data, err := common.Marshal(overrides)
	if err != nil {
		return 0, err
	}
	// Reserve every JSON byte, including custom/provider fields dropped by DTO
	// token metadata. Count ALL possible literal values regardless of conditions,
	// keep_origin or repeated sets: matching uses the later converted body.
	budget := int64(len(data))
	if budget > model.MaxDailyTokenPoolLimit {
		return 0, errors.New("shared token pool tool overrides exceed the token limit")
	}
	for _, value := range toolValues {
		data, err := common.Marshal(value)
		if err != nil {
			return 0, err
		}
		var tools []json.RawMessage
		if err := common.Unmarshal(data, &tools); err != nil {
			return 0, errors.New("shared token pool tools override must be an array or null")
		}
		if int64(len(tools)) > (model.MaxDailyTokenPoolLimit-budget)/8 {
			return 0, errors.New("shared token pool tool overrides exceed the token limit")
		}
		budget += int64(len(tools)) * 8
	}
	return budget, nil
}

func validateDailyPoolToolOverrideConditions(raw interface{}) error {
	// Match the condition shapes accepted by the outbound engine. Malformed
	// conditions make its parser fall back to legacy overrides; never allow that.
	switch conditions := raw.(type) {
	case map[string]interface{}:
		if len(conditions) == 0 {
			return errors.New("shared token pool override conditions must not be an empty object")
		}
		for path := range conditions {
			if strings.TrimSpace(path) == "" {
				return errors.New("shared token pool override condition path is required")
			}
		}
	case []interface{}:
		for _, rawCondition := range conditions {
			condition, ok := rawCondition.(map[string]interface{})
			if !ok {
				return errors.New("shared token pool override condition must be an object")
			}
			path, _ := condition["path"].(string)
			mode, _ := condition["mode"].(string)
			if strings.TrimSpace(path) == "" {
				return errors.New("shared token pool override condition path is required")
			}
			switch strings.ToLower(mode) {
			case "full", "prefix", "suffix", "contains", "gt", "gte", "lt", "lte":
			default:
				return errors.New("invalid shared token pool override condition mode")
			}
			for _, field := range []string{"invert", "pass_missing_key"} {
				if value, exists := condition[field]; exists {
					if _, ok := value.(bool); !ok {
						return errors.New("invalid shared token pool override condition flag")
					}
				}
			}
		}
	default:
		return errors.New("shared token pool override conditions must be an array or object")
	}
	return nil
}

func ReserveDailyTokenPool(c *gin.Context, info *relaycommon.RelayInfo, channel *model.Channel) *types.NewAPIError {
	config, err := model.GetDailyTokenPoolConfig()
	if err != nil {
		return types.NewErrorWithStatusCode(errors.New("shared token pool settings are unavailable"), types.ErrorCodeUpdateDataError, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if group == "" {
		group = info.TokenGroup
	}
	deny := func(err error, code int) *types.NewAPIError {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, code, types.ErrOptionWithSkipRetry())
	}
	if info.DailyTokenPoolFree {
		if !config.Enabled {
			return deny(errors.New("daily shared token pool is disabled"), http.StatusServiceUnavailable)
		}
		if !config.FreeUsage || !config.Matches(group, channel.Id) {
			return deny(errors.New("selected request is outside the free shared token pool; paid fallback is disabled"), http.StatusForbidden)
		}
	}
	if !config.Matches(group, channel.Id) {
		return nil
	}
	actualChannel, err := model.GetChannelById(channel.Id, false)
	if err != nil {
		return deny(errors.New("unable to load shared token pool channel settings"), http.StatusServiceUnavailable)
	}
	if info.RelayFormat == types.RelayFormatOpenAIRealtime {
		return deny(errors.New("realtime sessions are not supported by the daily token pool"), http.StatusBadRequest)
	}
	selectedSettings, _ := common.GetContextKeyType[dto.ChannelSettings](c, constant.ContextKeyChannelSetting)
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled || actualChannel.GetSetting().PassThroughBodyEnabled || selectedSettings.PassThroughBodyEnabled {
		return deny(errors.New("shared token pool does not support raw passthrough"), http.StatusBadRequest)
	}
	overrides := actualChannel.GetParamOverride()
	if selected, exists := common.GetContextKey(c, constant.ContextKeyChannelParamOverride); exists {
		var ok bool
		overrides, ok = selected.(map[string]interface{})
		if !ok {
			return deny(errors.New("invalid shared token pool channel parameter overrides"), http.StatusBadRequest)
		}
	}
	// Relay helpers initialize ChannelMeta from this context after reservation;
	// it includes affinity templates and may differ from the database snapshot.
	overrideBudget, err := dailyPoolToolOverrideBudget(overrides)
	if err != nil {
		return deny(err, http.StatusBadRequest)
	}
	output, err := dailyPoolOutputLimit(info.Request)
	if err != nil {
		return deny(err, http.StatusBadRequest)
	}
	prompt, err := EstimateRequestToken(c, info.Request.GetTokenCountMeta(), info, true)
	if err != nil {
		return deny(err, http.StatusBadRequest)
	}
	if int64(max(0, prompt)) > model.MaxDailyTokenPoolLimit-output-overrideBudget {
		return deny(errors.New("shared token pool request exceeds the token limit"), http.StatusBadRequest)
	}
	amount := max(int64(1), int64(max(0, prompt))+output+overrideBudget)
	id, err := model.ReserveDailyTokenPool(config, amount, time.Now())
	if errors.Is(err, model.ErrDailyTokenPoolExhausted) {
		_, reset, _ := config.Window(time.Now())
		c.Header("Retry-After", fmt.Sprint(max(int64(1), time.Until(reset).Milliseconds()/1000)))
		return deny(errors.New("daily shared token pool has insufficient available tokens; reduce the output limit or wait for the daily reset"), http.StatusTooManyRequests)
	}
	if err != nil {
		return deny(errors.New("unable to reserve shared token pool tokens"), http.StatusServiceUnavailable)
	}
	info.DailyTokenPoolReservationID = id
	return nil
}

func SettleDailyTokenPool(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage) {
	if info.DailyTokenPoolReservationID == "" || usage == nil {
		return
	}
	prompt := int64(usage.PromptTokens)
	completion := int64(usage.CompletionTokens)
	if prompt < 0 || completion < 0 || prompt > model.MaxDailyTokenPoolLimit || completion > model.MaxDailyTokenPoolLimit-prompt {
		logger.LogError(c, "invalid shared token pool usage; retaining reservation")
		return
	}
	// Claude input_tokens excludes cache reads/writes; OpenAI includes them.
	if usageSemanticFromUsage(info, usage) == "anthropic" {
		cache := int64(usage.PromptTokensDetails.CachedTokens)
		created := int64(usage.PromptTokensDetails.CacheCreationTokensTotal())
		if cache < 0 || created < 0 || cache > model.MaxDailyTokenPoolLimit-prompt-completion || created > model.MaxDailyTokenPoolLimit-prompt-completion-cache {
			logger.LogError(c, "invalid shared token pool cache usage; retaining reservation")
			return
		}
		prompt += cache + created
	}
	if err := model.SettleDailyTokenPool(info.DailyTokenPoolReservationID, prompt+completion); err != nil {
		logger.LogError(c, "shared token pool settlement failed: "+err.Error())
		return
	}
	info.DailyTokenPoolReservationID = ""
}

func RefundDailyTokenPool(c *gin.Context, info *relaycommon.RelayInfo) {
	if err := model.SettleDailyTokenPool(info.DailyTokenPoolReservationID, 0); err != nil {
		logger.LogError(c, "shared token pool refund failed: "+err.Error())
		return
	}
	info.DailyTokenPoolReservationID = ""
}
