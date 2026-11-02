package openai

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestKeepsCodexUltraHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	ctx.Request.Header.Set("X-Codex-Turn-Metadata", `{"reasoning_effort":"ultra"}`)

	request := dto.OpenAIResponsesRequest{
		Model:     "gpt-6-astra",
		Reasoning: &dto.Reasoning{Effort: "max"},
	}
	info, err := relaycommon.GenRelayInfo(ctx, types.RelayFormatOpenAIResponses, &request, nil)
	require.NoError(t, err)

	_, err = (&Adaptor{}).ConvertOpenAIResponsesRequest(ctx, info, request)
	require.NoError(t, err)
	assert.Equal(t, "ultra", info.ReasoningEffort)
}

func TestConvertOpenAIResponsesRequestKeepsPlainMax(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	ctx.Request.Header.Set("X-Codex-Turn-Metadata", `{"reasoning_effort":"max"}`)

	request := dto.OpenAIResponsesRequest{
		Model:     "gpt-6-astra",
		Reasoning: &dto.Reasoning{Effort: "max"},
	}
	info, err := relaycommon.GenRelayInfo(ctx, types.RelayFormatOpenAIResponses, &request, nil)
	require.NoError(t, err)

	_, err = (&Adaptor{}).ConvertOpenAIResponsesRequest(ctx, info, request)
	require.NoError(t, err)
	assert.Equal(t, "max", info.ReasoningEffort)
}
