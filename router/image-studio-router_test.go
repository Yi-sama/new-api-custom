package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestImageStudioRouterRegistersOnlyBridgeContractRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetImageStudioRouter(router)

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, path := range []string{
		"GET /api/image-studio/context",
		"GET /api/image-studio/groups",
		"GET /api/image-studio/v1/models",
		"POST /api/image-studio/v1/chat/completions",
		"POST /api/image-studio/v1/responses",
		"POST /api/image-studio/v1/images/generations",
		"POST /api/image-studio/v1/images/edits",
		"POST /api/image-studio/v1/audio/speech",
		"POST /api/image-studio/v1/audio/transcriptions",
		"POST /api/image-studio/v1/audio/translations",
	} {
		assert.Contains(t, routes, path)
	}
	assert.NotContains(t, routes, http.MethodGet+" /api/image-studio/v1/chat/completions")
}

func TestImageStudioMetadataRemainsAuthenticatedDuringRelayOverload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("IMAGE_STUDIO_URL", "https://studio.example.com")
	t.Setenv("IMAGE_STUDIO_IDENTITY_DOMAIN", "studio.example.com")
	t.Setenv("IMAGE_STUDIO_IDENTITY_EMAIL_FORMAT", "")
	t.Setenv("IMAGE_STUDIO_COOKIE_DOMAIN", "")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB, oldRedis, oldSecret := model.DB, common.RedisEnabled, common.SessionSecret
	model.DB, common.RedisEnabled, common.SessionSecret = db, false, "studio-router-test-secret"
	t.Cleanup(func() {
		model.DB, common.RedisEnabled, common.SessionSecret = oldDB, oldRedis, oldSecret
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.Channel{}, &model.ImageStudioCredit{}))
	user := model.User{Username: "studio-router-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1}
	require.NoError(t, db.Create(&user).Error)
	expires := time.Now().Add(time.Hour)
	session := model.UserSession{
		SID: "studio-router-session", UserID: user.Id, Version: 1,
		UserAuthVersion: 1, Status: model.UserSessionStatusActive,
		RefreshHash: "unused-test-hash", ExpiresAt: expires.Unix(),
	}
	require.NoError(t, model.CreateUserSession(&session))
	token, _, err := service.IssueImageStudioToken(service.AuthIdentity{
		UserID: user.Id, SessionID: session.SID, UserAuthVersion: 1, SessionVersion: 1,
	}, "user", expires)
	require.NoError(t, err)

	router := gin.New()
	setImageStudioRouter(router, func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "system_memory_overloaded"}})
	})
	for _, path := range []string{"/context", "/groups"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/image-studio"+path, nil)
			anonymous := httptest.NewRecorder()
			router.ServeHTTP(anonymous, request)
			assert.Equal(t, http.StatusUnauthorized, anonymous.Code)

			request = httptest.NewRequest(http.MethodGet, "/api/image-studio"+path, nil)
			request.AddCookie(&http.Cookie{Name: "new_api_image_studio", Value: token})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			assert.Contains(t, response.Header().Get("Content-Type"), "application/json")
			assert.Contains(t, response.Header().Get("Cache-Control"), "no-store")
			if path == "/context" {
				assert.Contains(t, response.Body.String(), user.Username)
			} else {
				assert.Contains(t, response.Body.String(), `"success":true`)
			}
		})
	}
	for _, route := range router.Routes() {
		if route.Path == "/api/image-studio/context" || route.Path == "/api/image-studio/groups" {
			continue
		}
		request := httptest.NewRequest(route.Method, route.Path, nil)
		request.AddCookie(&http.Cookie{Name: "new_api_image_studio", Value: token})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		assert.Equal(t, http.StatusServiceUnavailable, response.Code, route.Path)
		assert.Contains(t, response.Body.String(), "system_memory_overloaded", route.Path)
	}
}
