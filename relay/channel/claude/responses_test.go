package claude

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeResponsesRequestConversion(t *testing.T) {
	var request dto.OpenAIResponsesRequest
	require.NoError(t, common.UnmarshalJsonStr(`{
		"model":"claude-test","instructions":"Be concise.","max_output_tokens":128,
		"stream":true,"temperature":0,
		"tools":[{"type":"function","name":"lookup","parameters":{"type":"object","properties":{}}}],
		"input":[
			{"role":"user","content":"Look it up."},
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_1","output":"Found it."}
		]}`, &request))
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	got, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, request)
	require.NoError(t, err)
	converted, ok := got.(*dto.ClaudeRequest)
	require.True(t, ok)
	assert.Equal(t, "claude-test", converted.Model)
	require.NotNil(t, converted.MaxTokens)
	assert.EqualValues(t, 128, *converted.MaxTokens)
	require.NotNil(t, converted.Temperature)
	assert.Zero(t, *converted.Temperature)
	require.NotNil(t, converted.Stream)
	assert.True(t, *converted.Stream)
	assert.Len(t, converted.Tools, 1)
	body, err := common.Marshal(converted)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"tool_use"`)
	assert.Contains(t, string(body), `"tool_result"`)
	assert.Contains(t, string(body), "Be concise.")
	request.PreviousResponseID = "resp_previous"
	_, err = (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, request)
	require.ErrorContains(t, err, "previous_response_id")
}

func claudeResponsesContext(t *testing.T, stream bool) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "claude-responses-test")
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIResponses,
		IsStream:    stream,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"},
	}
	info.SetEstimatePromptTokens(1000000)
	return c, recorder, info
}

func TestClaudeResponsesJSONAndBilling(t *testing.T) {
	c, recorder, info := claudeResponsesContext(t, false)
	body := `{"id":"msg_1","type":"message","model":"claude-test","stop_reason":"tool_use",
		"content":[{"type":"text","text":"Checking."},{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"test"}}],
		"usage":{"input_tokens":7,"output_tokens":3,"cache_read_input_tokens":11,"cache_creation_input_tokens":5}}`
	result, apiErr := (&Adaptor{}).DoResponse(c, &http.Response{
		StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)),
	}, info)
	require.Nil(t, apiErr)
	usage, ok := result.(*dto.Usage)
	require.True(t, ok)
	assert.Equal(t, 7, usage.PromptTokens)
	assert.Equal(t, 3, usage.CompletionTokens)
	assert.Equal(t, "anthropic", usage.UsageSemantic)
	require.NotNil(t, usage.BillingUsage)
	got := recorder.Body.String()
	assert.Contains(t, got, `"object":"response"`)
	assert.Contains(t, got, `"type":"output_text"`)
	assert.Contains(t, got, `"text":"Checking."`)
	assert.Contains(t, got, `"type":"function_call"`)
	assert.Contains(t, got, `"call_id":"call_1"`)
	assert.Contains(t, got, `"input_tokens":23`)
	assert.Contains(t, got, `"output_tokens":3`)
	assert.NotContains(t, got, `"choices"`)
}

func TestClaudeResponsesStream(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 5
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{"text",
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n" +
				`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"OK"}}`,
			`"delta":"OK"`},
		{"tool",
			`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{}}}` + "\n" +
				`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"q\":\"test\"}"}}`,
			`event: response.function_call_arguments.delta`},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, recorder, info := claudeResponsesContext(t, true)
			lines := []string{
				`{"type":"message_start","message":{"id":"msg_1","model":"claude-test","usage":{"input_tokens":7,"output_tokens":0,"cache_read_input_tokens":11,"cache_creation_input_tokens":5}}}`,
			}
			lines = append(lines, strings.Split(test.body, "\n")...)
			lines = append(lines,
				`{"type":"content_block_stop","index":0}`,
				`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`,
				`{"type":"message_stop"}`)
			body := "data: " + strings.Join(lines, "\n\ndata: ") + "\n\n"
			result, apiErr := (&Adaptor{}).DoResponse(c, &http.Response{
				StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)),
			}, info)
			require.Nil(t, apiErr)
			usage := result.(*dto.Usage)
			assert.Equal(t, 7, usage.PromptTokens, "billing retains exclusive Anthropic input")
			assert.Equal(t, 3, usage.CompletionTokens)
			require.NotNil(t, usage.BillingUsage)
			got := recorder.Body.String()
			assert.Contains(t, got, "event: response.created")
			assert.Contains(t, got, test.want)
			assert.Equal(t, 1, strings.Count(got, "event: response.completed\n"))
			assert.Contains(t, got, `"input_tokens":23`)
			assert.Contains(t, got, `"output_tokens":3`)
			assert.NotContains(t, got, "data: [DONE]")
			assert.NotContains(t, got, `"choices"`)
		})
	}
}

func TestClaudeResponsesStreamRejectsErrorsAndIncompleteInput(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 5
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	for _, body := range []string{
		"",
		"data: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"busy\"}}\n\n",
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude-test\",\"usage\":{\"input_tokens\":7}}}\n\n",
	} {
		c, recorder, info := claudeResponsesContext(t, true)
		result, apiErr := (&Adaptor{}).DoResponse(c, &http.Response{
			StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)),
		}, info)
		require.NotNil(t, apiErr)
		assert.Nil(t, result)
		assert.NotContains(t, recorder.Body.String(), "event: response.completed")
	}
}
