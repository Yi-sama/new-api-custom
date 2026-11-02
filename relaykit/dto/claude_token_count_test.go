package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeTokenCountMetaSeparatesToolResultMedia(t *testing.T) {
	text := "Screenshot captured"
	request := &ClaudeRequest{
		Model: "claude-opus-5-5",
		Messages: []ClaudeMessage{{
			Role: "user",
			Content: []ClaudeMediaMessage{{
				Type: "tool_result",
				Content: []ClaudeMediaMessage{
					{Type: "text", Text: &text},
					{Type: "image", Source: &ClaudeMessageSource{Type: "base64", MediaType: "image/png", Data: "BASE64_IMAGE_CONTENT_NOT_TEXT"}},
					{Type: "document", Source: &ClaudeMessageSource{Type: "base64", MediaType: "application/pdf", Data: "BASE64_DOCUMENT_CONTENT_NOT_TEXT"}},
				},
			}},
		}},
	}

	meta := request.GetTokenCountMeta()

	assert.Equal(t, "user\nScreenshot captured", meta.CombineText)
	require.Len(t, meta.Files, 2)
	assert.Equal(t, types.FileTypeImage, meta.Files[0].FileType)
	assert.Equal(t, types.FileTypeFile, meta.Files[1].FileType)
	assert.NotNil(t, meta.Files[0].Source)
	assert.NotNil(t, meta.Files[1].Source)
}

func TestClaudeTokenCountMetaPreservesToolTextAndInputs(t *testing.T) {
	request := &ClaudeRequest{
		Messages: []ClaudeMessage{{
			Role: "assistant",
			Content: []ClaudeMediaMessage{
				{Type: "tool_use", Name: "lookup", Input: map[string]any{"query": "hello"}},
				{Type: "tool_result", Content: "result text"},
			},
		}},
	}

	meta := request.GetTokenCountMeta()

	assert.Equal(t, "assistant\nlookup\n{\"query\":\"hello\"}\nresult text", meta.CombineText)
	assert.Empty(t, meta.Files)
}
