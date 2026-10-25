package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeCountTokens(t *testing.T) {
	oldCountToken := constant.CountToken
	constant.CountToken = false
	t.Cleanup(func() { constant.CountToken = oldCountToken })

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{"text without max_tokens", `{"model":"claude-opus-5-5","messages":[{"role":"user","content":"Hello world"}]}`, http.StatusOK},
		{"missing messages", `{"model":"claude-opus-5-5"}`, http.StatusBadRequest},
		{"missing model", `{"messages":[{"role":"user","content":"Hello"}]}`, http.StatusBadRequest},
		{"invalid JSON", `{"model":`, http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens?beta=true", strings.NewReader(test.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "claude-opus-5-5")
			t.Cleanup(func() { common.CleanupBodyStorage(ctx) })

			ClaudeCountTokens(ctx)

			assert.Equal(t, test.status, recorder.Code)
			var response map[string]any
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			if test.status == http.StatusOK {
				assert.Greater(t, response["input_tokens"].(float64), float64(0))
				assert.Equal(t, "local-estimate", recorder.Header().Get("X-Token-Count-Source"))
				assert.NotContains(t, response, "usage")
			} else {
				assert.Equal(t, "error", response["type"])
			}
		})
	}
}
