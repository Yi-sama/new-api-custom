package common_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepCopyNilSource(t *testing.T) {
	var src *dto.OpenAIResponsesRequest
	dst, err := common.DeepCopy(src)
	require.EqualError(t, err, "copy source cannot be nil")
	assert.Nil(t, dst)
}

func TestDeepCopyRawMessageValues(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value json.RawMessage
	}{
		{name: "absent"},
		{name: "empty", value: json.RawMessage{}},
		{name: "null", value: json.RawMessage(`null`)},
		{name: "false", value: json.RawMessage(`false`)},
		{name: "zero", value: json.RawMessage(`0`)},
		{name: "object", value: json.RawMessage(`{"count":0,"enabled":false}`)},
		{name: "array", value: json.RawMessage(`[{"type":"input_text","text":"hello"}]`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := &struct{ Payload json.RawMessage }{Payload: tc.value}
			dst, err := common.DeepCopy(src)
			require.NoError(t, err)
			require.Equal(t, src.Payload, dst.Payload)
			if len(dst.Payload) > 0 {
				before := bytes.Clone(src.Payload)
				dst.Payload[0] ^= 1
				assert.Equal(t, before, []byte(src.Payload))
			}
		})
	}
}

func TestDeepCopyResponsesRequestPreservesParameters(t *testing.T) {
	stream, temperature, maxTokens := false, 0.0, uint(0)
	src := &dto.OpenAIResponsesRequest{
		Model: "diagnostic",
		Input: json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"hello"}]}]`),
		Tools: json.RawMessage(`[{"type":"function","name":"lookup","parameters":{"type":"object"}}]`),
		Store: json.RawMessage(`false`),
		Reasoning: &dto.Reasoning{
			Effort: "xhigh", Mode: json.RawMessage(`false`), Context: json.RawMessage(`{"count":0}`),
		},
		Stream: &stream, Temperature: &temperature, MaxOutputTokens: &maxTokens,
	}
	before, err := common.Marshal(src)
	require.NoError(t, err)
	dst, err := common.DeepCopy(src)
	require.NoError(t, err)
	after, err := common.Marshal(dst)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
	require.NotNil(t, dst.Reasoning)
	require.NotNil(t, dst.Stream)
	require.NotNil(t, dst.Temperature)
	require.NotNil(t, dst.MaxOutputTokens)

	dst.Input[0] = ' '
	dst.Tools[0] = ' '
	dst.Store[0] = ' '
	dst.Reasoning.Mode[0] = ' '
	dst.Reasoning.Context[0] = ' '
	dst.Reasoning.Effort = "high"
	*dst.Stream, *dst.Temperature, *dst.MaxOutputTokens = true, 1.0, 1
	unchanged, err := common.Marshal(src)
	require.NoError(t, err)
	assert.Equal(t, before, unchanged)
}

func TestDeepCopyNestedRawMessages(t *testing.T) {
	type item struct{ Payload json.RawMessage }
	raw := json.RawMessage(`{"enabled":false}`)
	src := &struct {
		Items    []item
		Index    map[string]json.RawMessage
		Optional *json.RawMessage
	}{
		Items:    []item{{Payload: json.RawMessage(`{"count":0}`)}},
		Index:    map[string]json.RawMessage{"key": json.RawMessage(`["original"]`)},
		Optional: &raw,
	}
	dst, err := common.DeepCopy(src)
	require.NoError(t, err)
	require.Equal(t, src, dst)
	require.Len(t, dst.Items, 1)
	require.Contains(t, dst.Index, "key")
	require.NotNil(t, dst.Optional)
	dst.Items[0].Payload[0] = ' '
	dst.Index["key"][0] = ' '
	(*dst.Optional)[0] = ' '
	assert.Equal(t, json.RawMessage(`{"count":0}`), src.Items[0].Payload)
	assert.Equal(t, json.RawMessage(`["original"]`), src.Index["key"])
	assert.Equal(t, json.RawMessage(`{"enabled":false}`), *src.Optional)
}
