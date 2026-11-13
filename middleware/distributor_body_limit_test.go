package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDistributeRejectsOversizeBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())
	previousLimit := constant.MaxRequestBodyMB
	constant.MaxRequestBodyMB = 1
	t.Cleanup(func() { constant.MaxRequestBodyMB = previousLimit })

	for _, test := range []struct {
		name        string
		path        string
		contentType string
		unknownSize bool
	}{
		{"responses", "/v1/responses", "application/json", false},
		{"chunked JSON", "/v1/responses", "application/json", true},
		{"chat", "/v1/chat/completions", "application/json", false},
		{"video form", "/v1/videos", gin.MIMEPOSTForm, false},
		{"video multipart", "/v1/videos", "multipart/form-data; boundary=fixture", false},
		{"midjourney", "/mj/submit/imagine", "application/json", false},
		{"image edit", "/v1/images/edits", "multipart/form-data; boundary=fixture", false},
		{"audio transcription", "/v1/audio/transcriptions", "multipart/form-data; boundary=fixture", false},
		{"audio translation", "/v1/audio/translations", "multipart/form-data; boundary=fixture", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			reachedHandler := false
			router := gin.New()
			router.Use(func(c *gin.Context) {
				common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
				common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{})
			})
			router.POST(test.path, Distribute(), func(c *gin.Context) {
				reachedHandler = true
				c.Status(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(strings.Repeat(" ", (1<<20)+1)))
			req.Header.Set("Content-Type", test.contentType)
			if test.unknownSize {
				req.ContentLength = -1
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)

			require.Equal(t, http.StatusRequestEntityTooLarge, response.Code, response.Body.String())
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &envelope))
			assert.Equal(t, string(types.ErrorCodeReadRequestBodyFailed), envelope.Error.Code)
			assert.False(t, reachedHandler)
		})
	}
}

func TestDistributeMalformedJSONRemainsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())
	router := gin.New()
	router.POST("/v1/responses", Distribute())
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	assert.Equal(t, http.StatusBadRequest, response.Code)
}
