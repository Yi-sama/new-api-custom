package router

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	oldDB, oldRedis, oldSecret := model.DB, common.RedisEnabled, common.SessionSecret
	oldMaster, oldPath := common.IsMasterNode, common.SQLitePath
	oldMainType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
	t.Setenv("SQL_DSN", "local")
	common.IsMasterNode, common.SQLitePath = false, filepath.Join(t.TempDir(), "studio.db")
	require.NoError(t, model.InitDB())
	db := model.DB
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	common.RedisEnabled, common.SessionSecret = false, "studio-router-test-secret"
	t.Cleanup(func() {
		model.DB, common.RedisEnabled, common.SessionSecret = oldDB, oldRedis, oldSecret
		common.IsMasterNode, common.SQLitePath = oldMaster, oldPath
		common.SetDatabaseTypes(oldMainType, oldLogType)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.Channel{}, &model.Ability{}, &model.ImageStudioCredit{}))
	user := model.User{Username: "studio-router-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1}
	user.SetSetting(dto.UserSetting{AcceptUnsetRatioModel: true})
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "gpt-4o", ChannelId: 1, Enabled: true}).Error)
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
	for _, path := range []string{"/context", "/groups", "/v1/models"} {
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
			if path == "/v1/models" {
				assert.Contains(t, response.Body.String(), `"object":"list"`)
				assert.Contains(t, response.Body.String(), `"id":"gpt-4o"`)
				request = httptest.NewRequest(http.MethodGet, "/api/image-studio"+path, nil)
				request.AddCookie(&http.Cookie{Name: "new_api_image_studio", Value: token})
				request.Header.Set("X-NewAPI-Group", "not-authorized")
				forbidden := httptest.NewRecorder()
				router.ServeHTTP(forbidden, request)
				assert.Equal(t, http.StatusForbidden, forbidden.Code)
			}
		})
	}
	for _, route := range router.Routes() {
		if route.Path == "/api/image-studio/context" || route.Path == "/api/image-studio/groups" || route.Path == "/api/image-studio/v1/models" {
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
