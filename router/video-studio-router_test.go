package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoStudioRouterRegistersContractAndServesSPA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	distDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "index.html"), []byte("video canvas"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(distDir, "assets"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "assets", "app.js"), []byte("asset"), 0o600))
	t.Setenv("VIDEO_STUDIO_DIST_DIR", distDir)
	router := gin.New()
	SetVideoStudioRouter(router)
	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range []string{
		"POST /api/video-studio/session",
		"GET /api/video-studio/context",
		"GET /api/video-studio/catalog",
		"GET /api/video-studio/v1/models",
		"GET /api/video-studio/storage/workspace",
		"PUT /api/video-studio/storage/workspace",
		"GET /api/video-studio/storage/files/:key",
		"PUT /api/video-studio/storage/files/:key",
		"POST /api/video-studio/v1/videos",
		"GET /api/video-studio/v1/videos/:task_id",
		"GET /api/video-studio/v1/videos/:task_id/content",
	} {
		assert.Contains(t, routes, route)
	}
	for path, expected := range map[string]string{"/video-canvas/video": "video canvas", "/video-canvas/assets/app.js": "asset"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, expected, response.Body.String())
		assert.Equal(t, "no-cache", response.Header().Get("Cache-Control"))
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/video-canvas/../secret", nil))
	assert.NotEqual(t, http.StatusOK, response.Code)
}

func TestVideoStudioStaticRevalidatesCachedEntry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	distDir := t.TempDir()
	indexPath := filepath.Join(distDir, "index.html")
	require.NoError(t, os.WriteFile(indexPath, []byte("current navigation"), 0o600))
	t.Setenv("VIDEO_STUDIO_DIST_DIR", distDir)
	router := gin.New()
	SetVideoStudioRouter(router)

	for _, path := range []string{"/video-canvas/", "/video-canvas/video?v=2"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			require.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, "current navigation", response.Body.String())
			assert.Equal(t, "no-cache", response.Header().Get("Cache-Control"))
			require.NotEmpty(t, response.Header().Get("Last-Modified"))

			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("If-Modified-Since", response.Header().Get("Last-Modified"))
			revalidated := httptest.NewRecorder()
			router.ServeHTTP(revalidated, request)
			assert.Equal(t, http.StatusNotModified, revalidated.Code)
			assert.Equal(t, "no-cache", revalidated.Header().Get("Cache-Control"))
		})
	}
}

func TestVideoStudioStaticRequiresConfiguredDist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("VIDEO_STUDIO_DIST_DIR", "")
	router := gin.New()
	SetVideoStudioRouter(router)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/video-canvas/video", nil))
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
}
