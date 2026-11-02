package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func prepareDailyPoolOverrideTest(t *testing.T) (*gin.Context, *relaycommon.RelayInfo, *model.Channel, model.DailyTokenPoolConfig) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	previousDB := model.DB
	settings := model_setting.GetGlobalSettings()
	previousPassthrough := settings.PassThroughRequestEnabled
	model.DB, settings.PassThroughRequestEnabled = db, false
	t.Cleanup(func() {
		model.DB, settings.PassThroughRequestEnabled = previousDB, previousPassthrough
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.Channel{}, &model.DailyTokenPoolDay{}, &model.DailyTokenPoolReservation{}))
	channel := &model.Channel{Id: 34, Type: constant.ChannelTypeOpenAI, ParamOverride: common.GetPointer("{}")}
	require.NoError(t, db.Create(channel).Error)
	config := model.DefaultDailyTokenPoolConfig()
	config.Enabled, config.ResetTimezone = true, "UTC"
	config.Groups, config.ChannelIDs = []string{"shared"}, []int{channel.Id}
	require.NoError(t, model.SaveDailyTokenPoolConfig(config))
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "shared")
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "pool-test")
	common.SetContextKey(ctx, constant.ContextKeyChannelId, channel.Id)
	common.SetContextKey(ctx, constant.ContextKeyChannelType, channel.Type)
	common.SetContextKey(ctx, constant.ContextKeyChannelParamOverride, map[string]interface{}{})
	info := &relaycommon.RelayInfo{
		TokenGroup: "shared", RelayFormat: types.RelayFormatOpenAI, OriginModelName: "pool-test",
		Request: &dto.GeneralOpenAIRequest{
			Model: "pool-test", MaxTokens: common.GetPointer(uint(12)), N: common.GetPointer(3),
			Messages: []dto.Message{{Role: "user", Content: "hello"}},
		},
	}
	return ctx, info, channel, config
}

func TestDailyTokenPoolToolOverridesReservePayloadAndPreserveOutboundLimits(t *testing.T) {
	for _, test := range []struct {
		name       string
		override   string
		responses  bool
		tools      string
		toolChoice string
		toolCount  int64
	}{
		{
			name:       "channel34 operations",
			override:   `{"operations":[{"mode":"set","path":"tools","value":[{"type":"web_search_preview"}]},{"mode":"set","path":"tool_choice","value":"auto"}]}`,
			tools:      `[{"type":"web_search_preview"}]`,
			toolChoice: `"auto"`,
			toolCount:  1,
		},
		{
			name:       "responses counts provider fields",
			override:   `{"tools":[{"type":"function","name":"inspect","description":"Inspect a record","parameters":{"type":"object"},"provider_prompt":"Not represented by chat token metadata"}],"tool_choice":{"type":"function","name":"inspect"}}`,
			responses:  true,
			tools:      `[{"type":"function","name":"inspect","description":"Inspect a record","parameters":{"type":"object"},"provider_prompt":"Not represented by chat token metadata"}]`,
			toolChoice: `{"type":"function","name":"inspect"}`,
			toolCount:  1,
		},
		{
			name:      "mixed legacy then operations",
			override:  `{"tools":[{"type":"function","function":{"name":"old"}}],"tool_choice":"required","operations":[{"mode":"set","path":"tools","value":[{"type":"custom","custom":{"name":"new","description":"Complete raw custom definition"}}]},{"mode":"delete","path":"tool_choice"}]}`,
			tools:     `[{"type":"custom","custom":{"name":"new","description":"Complete raw custom definition"}}]`,
			toolCount: 2,
		},
		{
			name:     "delete tools",
			override: `{"operations":[{"mode":"delete","path":"tools"},{"mode":"delete","path":"tool_choice"}]}`,
		},
		{
			name:      "nonmatching shorthand condition still reserves value",
			override:  `{"operations":[{"mode":"set","path":"tools","value":[{"type":"web_search"}],"conditions":{"request_path":"/v1/responses"}}]}`,
			toolCount: 1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, info, channel, _ := prepareDailyPoolOverrideTest(t)
			if test.responses {
				info.RelayFormat = types.RelayFormatOpenAIResponses
				info.Request = &dto.OpenAIResponsesRequest{
					Model: "pool-test", Input: json.RawMessage(`"hello"`),
					MaxOutputTokens: common.GetPointer(uint(12)),
				}
			}
			require.NoError(t, model.DB.Model(channel).Update("param_override", test.override).Error)
			var overrides map[string]interface{}
			require.NoError(t, common.UnmarshalJsonStr(test.override, &overrides))
			common.SetContextKey(ctx, constant.ContextKeyChannelParamOverride, overrides)
			before, err := common.Marshal(info.Request)
			require.NoError(t, err)
			prompt, err := EstimateRequestToken(ctx, info.Request.GetTokenCountMeta(), info, true)
			require.NoError(t, err)
			output := int64(36)
			capField := "max_tokens"
			if test.responses {
				output, capField = 12, "max_output_tokens"
			}

			require.Nil(t, ReserveDailyTokenPool(ctx, info, channel))
			require.NotEmpty(t, info.DailyTokenPoolReservationID)
			assert.Nil(t, info.ChannelMeta, "reservation must preserve the first-channel sentinel")
			var reservation model.DailyTokenPoolReservation
			require.NoError(t, model.DB.First(&reservation, "id = ?", info.DailyTokenPoolReservationID).Error)
			literalJSON, err := common.Marshal(overrides)
			require.NoError(t, err)
			assert.Equal(t, int64(prompt)+output+int64(len(literalJSON))+8*test.toolCount, reservation.Reserved)
			after, err := common.Marshal(info.Request)
			require.NoError(t, err)
			assert.JSONEq(t, string(before), string(after), "the outbound override must not be applied early")

			// Follow TextHelper/ResponsesHelper: initialize selected metadata,
			// marshal the DTO, filter fields, then run the real override engine.
			info.InitChannelMeta(ctx)
			body, err := relaycommon.RemoveDisabledFields(after, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
			require.NoError(t, err)
			body, err = relaycommon.ApplyParamOverrideWithRelayInfo(body, info)
			require.NoError(t, err)
			var upstream map[string]json.RawMessage
			require.NoError(t, common.Unmarshal(body, &upstream))
			assert.JSONEq(t, "12", string(upstream[capField]))
			if !test.responses {
				assert.JSONEq(t, "3", string(upstream["n"]))
			}
			if test.tools == "" {
				assert.NotContains(t, upstream, "tools")
			} else {
				assert.JSONEq(t, test.tools, string(upstream["tools"]))
			}
			if test.toolChoice == "" {
				assert.NotContains(t, upstream, "tool_choice")
			} else {
				assert.JSONEq(t, test.toolChoice, string(upstream["tool_choice"]))
			}
		})
	}
}

func TestDailyTokenPoolConditionalChannel34OverridesReserveEveryPossibleValue(t *testing.T) {
	const override = `{
		"operations": [
			{
				"mode": "set", "path": "tools", "value": [{"type": "web_search"}], "logic": "AND",
				"description": "Restore web search tools for client metadata compaction",
				"conditions": [
					{"path": "request_path", "mode": "full", "value": "/v1/responses"},
					{"path": "client_metadata.request_kind", "mode": "full", "value": "compaction"},
					{"path": "client_metadata.implementation", "mode": "full", "value": "responses"},
					{"path": "tools.#", "mode": "full", "value": 0},
					{"path": "input.#(type==\"web_search_call\").type", "mode": "full", "value": "web_search_call"}
				]
			},
			{
				"mode": "set", "path": "tool_choice", "value": "auto", "logic": "AND",
				"description": "Use automatic tool choice for client metadata compaction",
				"conditions": [
					{"path": "request_path", "mode": "full", "value": "/v1/responses"},
					{"path": "client_metadata.request_kind", "mode": "full", "value": "compaction"},
					{"path": "client_metadata.implementation", "mode": "full", "value": "responses"}
				]
			},
			{
				"mode": "set", "path": "tools", "value": [{"type": "web_search"}], "logic": "AND",
				"description": "Restore web search tools for header metadata compaction",
				"conditions": [
					{"path": "request_path", "mode": "full", "value": "/v1/responses"},
					{"path": "request_headers.x-codex-turn-metadata|@fromstr|request_kind", "mode": "full", "value": "compaction"},
					{"path": "request_headers.x-codex-turn-metadata|@fromstr|implementation", "mode": "full", "value": "responses"},
					{"path": "tools.#", "mode": "full", "value": 0},
					{"path": "input.#(type==\"web_search_call\").type", "mode": "full", "value": "web_search_call"}
				]
			},
			{
				"mode": "set", "path": "tool_choice", "value": "auto", "logic": "AND",
				"description": "Use automatic tool choice for header metadata compaction",
				"conditions": [
					{"path": "request_path", "mode": "full", "value": "/v1/responses"},
					{"path": "request_headers.x-codex-turn-metadata|@fromstr|request_kind", "mode": "full", "value": "compaction"},
					{"path": "request_headers.x-codex-turn-metadata|@fromstr|implementation", "mode": "full", "value": "responses"}
				]
			}
		]
	}`
	for _, scenario := range []string{
		"client metadata matches", "header metadata matches", "both metadata sources match",
		"request path does not match", "request kind does not match", "implementation does not match",
		"no metadata matches", "existing tools are preserved", "no prior web search call",
	} {
		t.Run(scenario, func(t *testing.T) {
			ctx, info, channel, _ := prepareDailyPoolOverrideTest(t)
			request := &dto.OpenAIResponsesRequest{
				Model: "pool-test", MaxOutputTokens: common.GetPointer(uint(12)),
				Input:          json.RawMessage(`[{"type":"web_search_call","id":"ws_1","status":"completed"}]`),
				Tools:          json.RawMessage(`[]`),
				ToolChoice:     json.RawMessage(`"none"`),
				ClientMetadata: json.RawMessage(`{"request_kind":"compaction","implementation":"responses"}`),
			}
			info.Request, info.RelayFormat = request, types.RelayFormatOpenAIResponses
			info.RequestURLPath = "/v1/responses"
			wantTools, wantChoice := `[{"type":"web_search"}]`, `"auto"`
			switch scenario {
			case "header metadata matches", "both metadata sources match":
				info.RequestHeaders = map[string]string{"X-Codex-Turn-Metadata": string(request.ClientMetadata)}
				if scenario == "header metadata matches" {
					request.ClientMetadata = nil
				}
			case "request path does not match":
				info.RequestURLPath = "/v1/chat/completions"
				wantTools, wantChoice = `[]`, `"none"`
			case "request kind does not match":
				request.ClientMetadata = json.RawMessage(`{"request_kind":"completion","implementation":"responses"}`)
				wantTools, wantChoice = `[]`, `"none"`
			case "implementation does not match":
				request.ClientMetadata = json.RawMessage(`{"request_kind":"compaction","implementation":"chat"}`)
				wantTools, wantChoice = `[]`, `"none"`
			case "no metadata matches":
				request.ClientMetadata = nil
				wantTools, wantChoice = `[]`, `"none"`
			case "existing tools are preserved":
				request.Tools = json.RawMessage(`[{"type":"function","name":"existing","parameters":{"type":"object"}}]`)
				wantTools = string(request.Tools)
			case "no prior web search call":
				request.Input = json.RawMessage(`"hello"`)
				wantTools = `[]`
			}
			require.NoError(t, model.DB.Model(channel).Update("param_override", override).Error)
			var overrides map[string]interface{}
			require.NoError(t, common.UnmarshalJsonStr(override, &overrides))
			common.SetContextKey(ctx, constant.ContextKeyChannelParamOverride, overrides)
			prompt, err := EstimateRequestToken(ctx, request.GetTokenCountMeta(), info, true)
			require.NoError(t, err)
			before, err := common.Marshal(request)
			require.NoError(t, err)

			require.Nil(t, ReserveDailyTokenPool(ctx, info, channel))
			assert.Nil(t, info.ChannelMeta)
			var reservation model.DailyTokenPoolReservation
			require.NoError(t, model.DB.First(&reservation, "id = ?", info.DailyTokenPoolReservationID).Error)
			allPossibleValues, err := common.Marshal(overrides)
			require.NoError(t, err)
			// Both possible tools arrays and both choices are reserved even if
			// neither condition matches, or one set makes a later set ineligible.
			assert.Equal(t, int64(prompt)+12+int64(len(allPossibleValues))+16, reservation.Reserved)
			after, err := common.Marshal(request)
			require.NoError(t, err)
			assert.JSONEq(t, string(before), string(after))

			info.InitChannelMeta(ctx)
			body, err := relaycommon.RemoveDisabledFields(after, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
			require.NoError(t, err)
			body, err = relaycommon.ApplyParamOverrideWithRelayInfo(body, info)
			require.NoError(t, err)
			var upstream map[string]json.RawMessage
			require.NoError(t, common.Unmarshal(body, &upstream))
			assert.JSONEq(t, wantTools, string(upstream["tools"]))
			assert.JSONEq(t, wantChoice, string(upstream["tool_choice"]))
			assert.JSONEq(t, "12", string(upstream["max_output_tokens"]))
			assert.NotContains(t, upstream, "description", "operation descriptions are inert metadata")
		})
	}
}

func TestDailyTokenPoolToolOverridesRejectUncountedOrLimitChangingOperations(t *testing.T) {
	ctx, info, channel, _ := prepareDailyPoolOverrideTest(t)
	for _, test := range []struct {
		name     string
		override string
	}{
		{"legacy output cap", `{"tools":[],"max_tokens":99999}`},
		{"legacy choices", `{"tool_choice":"auto","n":128}`},
		{"legacy prompt", `{"tools":[],"messages":[{"role":"system","content":"injected"}]}`},
		{"set output cap", `{"operations":[{"mode":"set","path":"max_tokens","value":99999}]}`},
		{"delete output cap", `{"operations":[{"mode":"delete","path":"max_completion_tokens"}]}`},
		{"responses output cap", `{"operations":[{"mode":"set","path":"max_output_tokens","value":99999}]}`},
		{"choices", `{"operations":[{"mode":"set","path":"n","value":128}]}`},
		{"injected instructions", `{"operations":[{"mode":"set","path":"instructions","value":"injected"}]}`},
		{"copy client prompt", `{"operations":[{"mode":"copy","from":"messages","to":"tools"}]}`},
		{"move output cap", `{"operations":[{"mode":"move","from":"max_tokens","to":"tools"}]}`},
		{"wildcard", `{"operations":[{"mode":"delete","path":"*"}]}`},
		{"root replacement", `{"operations":[{"mode":"set","path":"","value":{"n":128}}]}`},
		{"nested path", `{"operations":[{"mode":"set","path":"tools.0.function","value":{}}]}`},
		{"append", `{"operations":[{"mode":"append","path":"tools","value":{}}]}`},
		{"nonstring description", `{"operations":[{"mode":"set","path":"tools","value":[],"description":{"max_tokens":99999}}]}`},
		{"conditional unsafe target even when not matching", `{"operations":[{"mode":"set","path":"n","value":128,"logic":"AND","conditions":{"request_path":"/never"}}]}`},
		{"scalar conditions fallback", `{"operations":[{"mode":"set","path":"tools","value":[],"conditions":false}]}`},
		{"null conditions fallback", `{"operations":[{"mode":"set","path":"tools","value":[],"conditions":null}]}`},
		{"empty condition object fallback", `{"operations":[{"mode":"set","path":"tools","value":[],"conditions":{}}]}`},
		{"blank shorthand path", `{"operations":[{"mode":"set","path":"tools","value":[],"conditions":{" ":true}}]}`},
		{"nonobject condition fallback", `{"operations":[{"mode":"set","path":"tools","value":[],"conditions":["model"]}]}`},
		{"missing condition mode fallback", `{"operations":[{"mode":"set","path":"tools","value":[],"conditions":[{"path":"model","value":"pool-test"}]}]}`},
		{"missing condition path fallback", `{"operations":[{"mode":"set","path":"tools","value":[],"conditions":[{"mode":"full","value":"pool-test"}]}]}`},
		{"invalid comparison", `{"operations":[{"mode":"set","path":"tools","value":[],"conditions":[{"path":"model","mode":"set","value":"pool-test"}]}]}`},
		{"invalid condition flag", `{"operations":[{"mode":"set","path":"tools","value":[],"conditions":[{"path":"model","mode":"full","invert":"true"}]}]}`},
		{"invalid condition logic", `{"operations":[{"mode":"set","path":"tools","value":[],"logic":"XOR","conditions":{"model":"pool-test"}}]}`},
		{"nonstring condition logic", `{"operations":[{"mode":"set","path":"tools","value":[],"logic":true,"conditions":{"model":"pool-test"}}]}`},
		{"operations object fallback", `{"operations":{"mode":"set","path":"tools","value":[]}}`},
		{"invalid operation fallback", `{"operations":[{"path":"tools","value":[]}]}`},
		{"nonobject operation fallback", `{"operations":["tools"]}`},
		{"tools object", `{"tools":{"instructions":"not an array"}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var overrides map[string]interface{}
			require.NoError(t, common.UnmarshalJsonStr(test.override, &overrides))
			// This is the effective context map, including affinity overrides.
			// The stored channel deliberately has no overrides.
			common.SetContextKey(ctx, constant.ContextKeyChannelParamOverride, overrides)
			apiErr := ReserveDailyTokenPool(ctx, info, channel)
			require.NotNil(t, apiErr)
			assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
			assert.Empty(t, info.DailyTokenPoolReservationID)
		})
	}
	var count int64
	require.NoError(t, model.DB.Model(&model.DailyTokenPoolReservation{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestDailyTokenPoolInjectedToolPayloadMustFitRemainingPool(t *testing.T) {
	ctx, info, channel, config := prepareDailyPoolOverrideTest(t)
	previousCount := constant.CountToken
	constant.CountToken = false
	t.Cleanup(func() { constant.CountToken = previousCount })
	prompt, err := EstimateRequestToken(ctx, info.Request.GetTokenCountMeta(), info, true)
	require.NoError(t, err)
	config.DailyLimit = int64(prompt) + 36
	require.NoError(t, model.SaveDailyTokenPoolConfig(config))
	common.SetContextKey(ctx, constant.ContextKeyChannelParamOverride, map[string]interface{}{
		"tools": []interface{}{
			map[string]interface{}{"type": "custom", "custom": map[string]interface{}{
				"name": "inspect", "description": strings.Repeat("injected definition ", 20),
			}},
		},
		"tool_choice": map[string]interface{}{"type": "custom", "provider_prompt": "Count this raw field too"},
	})

	apiErr := ReserveDailyTokenPool(ctx, info, channel)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
	assert.NotEmpty(t, ctx.Writer.Header().Get("Retry-After"))
	assert.Empty(t, info.DailyTokenPoolReservationID)
	var count int64
	require.NoError(t, model.DB.Model(&model.DailyTokenPoolReservation{}).Count(&count).Error)
	assert.Zero(t, count)

	common.SetContextKey(ctx, constant.ContextKeyChannelParamOverride, map[string]interface{}{})
	require.Nil(t, ReserveDailyTokenPool(ctx, info, channel), "the same request without injected tools fits exactly")
	var reservation model.DailyTokenPoolReservation
	require.NoError(t, model.DB.First(&reservation, "id = ?", info.DailyTokenPoolReservationID).Error)
	assert.Equal(t, config.DailyLimit, reservation.Reserved, "forced estimation and all three output choices remain reserved")
}

func TestDailyTokenPoolToolOverridesDoNotBypassRequestLimits(t *testing.T) {
	ctx, info, channel, _ := prepareDailyPoolOverrideTest(t)
	common.SetContextKey(ctx, constant.ContextKeyChannelParamOverride, map[string]interface{}{"tools": []interface{}{}})
	for _, test := range []struct {
		name    string
		request dto.Request
		body    string
	}{
		{"zero choices", &dto.GeneralOpenAIRequest{}, `{"max_tokens":12,"n":0}`},
		{"too many choices", &dto.GeneralOpenAIRequest{}, `{"max_tokens":12,"n":129}`},
		{"negative choices", &dto.GeneralOpenAIRequest{}, `{"max_tokens":12,"n":-1}`},
		{"chat output", &dto.GeneralOpenAIRequest{}, `{"max_tokens":7812500001}`},
		{"chat completion output", &dto.GeneralOpenAIRequest{}, `{"max_completion_tokens":7812500001}`},
		{"responses output", &dto.OpenAIResponsesRequest{}, `{"max_output_tokens":7812500001}`},
		{"claude output", &dto.ClaudeRequest{}, `{"max_tokens":7812500001}`},
		{"gemini candidates", &dto.GeminiChatRequest{}, `{"generationConfig":{"maxOutputTokens":12,"candidateCount":129}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, common.UnmarshalJsonStr(test.body, test.request))
			info.Request = test.request
			apiErr := ReserveDailyTokenPool(ctx, info, channel)
			require.NotNil(t, apiErr)
			assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
			assert.Empty(t, info.DailyTokenPoolReservationID)
		})
	}
}

func TestDailyTokenPoolToolOverridesDoNotEnableRawOrUnsupportedFormats(t *testing.T) {
	for _, scenario := range []string{"global passthrough", "stored passthrough", "selected passthrough", "realtime", "audio"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, info, channel, _ := prepareDailyPoolOverrideTest(t)
			common.SetContextKey(ctx, constant.ContextKeyChannelParamOverride, map[string]interface{}{"tool_choice": "auto"})
			switch scenario {
			case "global passthrough":
				model_setting.GetGlobalSettings().PassThroughRequestEnabled = true
			case "stored passthrough":
				channel.SetSetting(dto.ChannelSettings{PassThroughBodyEnabled: true})
				require.NoError(t, model.DB.Model(channel).Update("setting", channel.Setting).Error)
			case "selected passthrough":
				common.SetContextKey(ctx, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: true})
			case "realtime":
				info.RelayFormat = types.RelayFormatOpenAIRealtime
			case "audio":
				info.Request = &dto.AudioRequest{}
			}
			apiErr := ReserveDailyTokenPool(ctx, info, channel)
			require.NotNil(t, apiErr)
			assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
			assert.Empty(t, info.DailyTokenPoolReservationID)
		})
	}
}

func TestDailyTokenPoolFreeReservationRejectsConfigChangesAfterPreparation(t *testing.T) {
	for _, scenario := range []string{"disabled", "free usage removed", "channel scope changed", "group scope changed"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, info, channel, config := prepareDailyPoolOverrideTest(t)
			config.FreeUsage = true
			require.NoError(t, model.SaveDailyTokenPoolConfig(config))
			require.Nil(t, PrepareDailyTokenPoolBilling(ctx, info))
			require.True(t, info.DailyTokenPoolFree)
			wantStatus := http.StatusForbidden
			switch scenario {
			case "disabled":
				config.Enabled = false
				wantStatus = http.StatusServiceUnavailable
			case "free usage removed":
				config.FreeUsage = false
			case "channel scope changed":
				config.ChannelIDs = []int{35}
			case "group scope changed":
				config.Groups = []string{"other"}
			}
			require.NoError(t, model.SaveDailyTokenPoolConfig(config))

			apiErr := ReserveDailyTokenPool(ctx, info, channel)
			require.NotNil(t, apiErr)
			assert.Equal(t, wantStatus, apiErr.StatusCode)
			assert.True(t, info.DailyTokenPoolFree, "scope changes cannot silently become paid fallback")
			assert.Empty(t, info.DailyTokenPoolReservationID)
			var count int64
			require.NoError(t, model.DB.Model(&model.DailyTokenPoolReservation{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}
