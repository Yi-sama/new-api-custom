package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type closeNotifyRecorder struct {
	*httptest.ResponseRecorder
}

func (recorder *closeNotifyRecorder) CloseNotify() <-chan bool {
	return make(chan bool)
}

func TestImageWorkbenchRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	distDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "index.html"), []byte("not the studio"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "studio.html"), []byte("workbench"), 0o600))

	var proxiedPath string
	var proxiedAuthorization string
	history := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxiedPath = r.URL.Path
		proxiedAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(history.Close)

	t.Setenv("IMAGE_WORKBENCH_DIST_DIR", distDir)
	t.Setenv("IMAGE_WORKBENCH_HISTORY_URL", history.URL)
	router := gin.New()
	SetImageWorkbenchRouter(router)

	staticResponse := httptest.NewRecorder()
	router.ServeHTTP(staticResponse, httptest.NewRequest(http.MethodGet, "/studio/", nil))
	assert.Equal(t, http.StatusOK, staticResponse.Code)
	assert.Equal(t, "workbench", staticResponse.Body.String())

	proxyRequest := httptest.NewRequest(http.MethodGet, "/studio-api/history?limit=1", nil)
	proxyRequest.Header.Set("Authorization", "Bearer dashboard-token")
	proxyResponse := &closeNotifyRecorder{ResponseRecorder: httptest.NewRecorder()}
	router.ServeHTTP(proxyResponse, proxyRequest)
	proxyBody, err := io.ReadAll(proxyResponse.Result().Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, proxyResponse.Code)
	assert.JSONEq(t, `{"ok":true}`, string(proxyBody))
	assert.Equal(t, "/studio-api/history", proxiedPath)
	assert.Equal(t, "Bearer dashboard-token", proxiedAuthorization)
}

func TestImageWorkbenchRejectsNonLocalHistoryURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("IMAGE_WORKBENCH_DIST_DIR", t.TempDir())
	t.Setenv("IMAGE_WORKBENCH_HISTORY_URL", "https://example.com")

	router := gin.New()
	SetImageWorkbenchRouter(router)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/studio-api/health", nil))

	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.Contains(t, response.Body.String(), imageWorkbenchUnavailable)
}
