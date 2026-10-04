package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTavernServiceAuth(t *testing.T) {
	key := strings.Repeat("a", 64)
	file := filepath.Join(t.TempDir(), "key")
	require.NoError(t, os.WriteFile(file, []byte(key), 0600))
	t.Setenv("TAVERN_SERVICE_KEY_FILE", file)
	for _, tc := range []struct {
		name, header string
		required     bool
		status       int
		authorized   bool
	}{
		{"ordinary playground", "", false, 200, false},
		{"forged marker", "browser-controlled", false, 403, false},
		{"missing bridge proof", "", true, 403, false},
		{"trusted bridge", key, true, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			r := gin.New()
			r.POST("/test", TavernServiceAuth(tc.required), func(c *gin.Context) {
				assert.Equal(t, tc.authorized, c.GetBool("tavern_authenticated"))
				c.Status(200)
			})
			request := httptest.NewRequest(http.MethodPost, "/test", nil)
			request.Header.Set("X-Tavern-Service-Key", tc.header)
			r.ServeHTTP(recorder, request)
			assert.Equal(t, tc.status, recorder.Code)
			assert.NotContains(t, recorder.Body.String(), key)
		})
	}
}
