package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func setImageStudioTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("IMAGE_STUDIO_URL", "https://studio.example.com")
	t.Setenv("IMAGE_STUDIO_IDENTITY_DOMAIN", "identity.example.com")
	t.Setenv("IMAGE_STUDIO_IDENTITY_EMAIL_FORMAT", "owui-{id}@{domain}")
	t.Setenv("IMAGE_STUDIO_COOKIE_DOMAIN", "example.com")
}

func setupImageStudioSession(t *testing.T, role int) (service.AuthIdentity, *model.UserSession) {
	t.Helper()
	previousDB, previousLogDB, previousRedis, previousSecret := model.DB, model.LOG_DB, common.RedisEnabled, common.SessionSecret
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.Log{}, &model.ImageStudioCredit{}))
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled = false
	common.SessionSecret = "image-studio-test-session-secret"
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedis
		common.SessionSecret = previousSecret
		_ = sqlDB.Close()
	})
	user := &model.User{
		Username:    " Studio\r\n \u4e2d\u6587 User ",
		Password:    "unused-password-hash",
		Role:        role,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
	}
	require.NoError(t, db.Create(user).Error)
	session := &model.UserSession{
		SID:             "studio-session",
		UserID:          user.Id,
		Version:         1,
		UserAuthVersion: user.AuthVersion,
		Status:          model.UserSessionStatusActive,
		RefreshHash:     "unused-refresh-hash",
		LoginMethod:     "password",
		CreatedAt:       time.Now().Unix(),
		LastActiveAt:    time.Now().Unix(),
		ExpiresAt:       time.Now().Add(24 * time.Hour).Unix(),
	}
	require.NoError(t, model.CreateUserSession(session))
	return service.AuthIdentity{UserID: user.Id, SessionID: session.SID, UserAuthVersion: 1, SessionVersion: 1}, session
}

func TestImageStudioConfigAndRoleMapping(t *testing.T) {
	setImageStudioTestEnv(t)
	config, err := loadImageStudioConfig()
	require.NoError(t, err)
	email, err := imageStudioEmail(config, 42)
	require.NoError(t, err)
	assert.Equal(t, "owui-42@identity.example.com", email)
	assert.True(t, ImageStudioEnabled())
	t.Setenv("IMAGE_STUDIO_IDENTITY_EMAIL_FORMAT", "")
	defaultConfig, err := loadImageStudioConfig()
	require.NoError(t, err)
	defaultEmail, err := imageStudioEmail(defaultConfig, 42)
	require.NoError(t, err)
	assert.Equal(t, "new-api-42@identity.example.com", defaultEmail)

	for _, testCase := range []struct {
		role int
		want string
		ok   bool
	}{
		{common.RoleRootUser, "admin", true},
		{common.RoleAdminUser, "admin", true},
		{common.RoleCommonUser, "user", true},
		{common.RoleGuestUser, "", false},
	} {
		got, ok := imageStudioRole(testCase.role)
		assert.Equal(t, testCase.want, got)
		assert.Equal(t, testCase.ok, ok)
	}

	t.Setenv("IMAGE_STUDIO_URL", "http://studio.example.com")
	assert.False(t, ImageStudioEnabled())
	t.Setenv("IMAGE_STUDIO_URL", "http://localhost:8080")
	assert.True(t, ImageStudioEnabled())
	t.Setenv("IMAGE_STUDIO_URL", "https://studio.example.com/ai")
	assert.False(t, ImageStudioEnabled())
	t.Setenv("IMAGE_STUDIO_IDENTITY_EMAIL_FORMAT", "attacker@example.net")
	assert.False(t, ImageStudioEnabled())
	t.Setenv("IMAGE_STUDIO_IDENTITY_EMAIL_FORMAT", "owui-{id}@{domain}")
	t.Setenv("IMAGE_STUDIO_COOKIE_DOMAIN", "com")
	assert.False(t, ImageStudioEnabled())
}

func TestImageStudioFirstEntryGiftRequiresLiveIdentityAndNeverResets(t *testing.T) {
	setImageStudioTestEnv(t)
	identity, session := setupImageStudioSession(t, common.RoleCommonUser)
	router := gin.New()
	router.GET("/identity", GetImageStudioIdentity)
	anonymous := httptest.NewRecorder()
	router.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/identity", nil))
	require.Equal(t, http.StatusUnauthorized, anonymous.Code)
	var count int64
	require.NoError(t, model.DB.Model(&model.ImageStudioCredit{}).Count(&count).Error)
	assert.Zero(t, count)
	token, _, err := service.IssueImageStudioToken(identity, "user", time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodGet, "/identity", nil)
	request.AddCookie(&http.Cookie{Name: imageStudioCookieName, Value: token})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	credit, err := model.GetImageStudioCredit(identity.UserID)
	require.NoError(t, err)
	require.Equal(t, common.QuotaFromFloat(common.QuotaPerUnit), credit.Quota)
	reserved, err := model.ReserveImageStudioCredit(identity.UserID, credit.Quota)
	require.NoError(t, err)
	assert.Equal(t, credit.Quota, reserved)
	request = httptest.NewRequest(http.MethodGet, "/identity", nil)
	request.AddCookie(&http.Cookie{Name: imageStudioCookieName, Value: token})
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	credit, err = model.GetImageStudioCredit(identity.UserID)
	require.NoError(t, err)
	assert.Zero(t, credit.Quota)
	require.NoError(t, model.DB.Model(&model.ImageStudioCredit{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestStatusReportsImageStudioEnabled(t *testing.T) {
	setImageStudioTestEnv(t)
	router := gin.New()
	router.GET("/api/status", GetStatus)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Data struct {
			ImageStudioEnabled bool `json:"image_studio_enabled"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Data.ImageStudioEnabled)
}

func TestCreateImageStudioSessionRejectsPAT(t *testing.T) {
	setImageStudioTestEnv(t)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "http://api.example.com/api/image-studio/session", nil)
	c.Set("id", 42)
	c.Set("auth_version", int64(1))
	c.Set("use_access_token", true)

	CreateImageStudioSession(c)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestCreateImageStudioSessionSetsPurposeCookie(t *testing.T) {
	setImageStudioTestEnv(t)
	identity, _ := setupImageStudioSession(t, common.RoleCommonUser)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "https://api.example.com/api/image-studio/session", nil)
	c.Set("id", identity.UserID)
	c.Set("session_id", identity.SessionID)
	c.Set("auth_version", identity.UserAuthVersion)
	c.Set("session_version", identity.SessionVersion)

	CreateImageStudioSession(c)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.Equal(t, imageStudioCookieName, cookie.Name)
	assert.Equal(t, "example.com", cookie.Domain)
	assert.Equal(t, "/", cookie.Path)
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.NotEmpty(t, cookie.Value)
	assert.NotContains(t, recorder.Body.String(), cookie.Value)
	assert.Contains(t, recorder.Body.String(), `"url":"https://studio.example.com/auth"`)

	badHostRecorder := httptest.NewRecorder()
	badHostContext, _ := gin.CreateTestContext(badHostRecorder)
	badHostContext.Request = httptest.NewRequest(http.MethodPost, "https://attacker.example.net/api/image-studio/session", strings.NewReader(""))
	badHostContext.Set("id", identity.UserID)
	badHostContext.Set("session_id", identity.SessionID)
	badHostContext.Set("auth_version", identity.UserAuthVersion)
	badHostContext.Set("session_version", identity.SessionVersion)
	CreateImageStudioSession(badHostContext)
	assert.Equal(t, http.StatusServiceUnavailable, badHostRecorder.Code)
	assert.Empty(t, badHostRecorder.Header().Values("Set-Cookie"))

	t.Setenv("IMAGE_STUDIO_COOKIE_DOMAIN", "")
	hostOnlyRecorder := httptest.NewRecorder()
	hostOnlyContext, _ := gin.CreateTestContext(hostOnlyRecorder)
	hostOnlyContext.Request = httptest.NewRequest(http.MethodPost, "https://api.example.com/api/image-studio/session", nil)
	hostOnlyContext.Set("id", identity.UserID)
	hostOnlyContext.Set("session_id", identity.SessionID)
	hostOnlyContext.Set("auth_version", identity.UserAuthVersion)
	hostOnlyContext.Set("session_version", identity.SessionVersion)
	CreateImageStudioSession(hostOnlyContext)
	assert.Equal(t, http.StatusServiceUnavailable, hostOnlyRecorder.Code)
	assert.Empty(t, hostOnlyRecorder.Header().Values("Set-Cookie"))
}

func TestImageStudioIdentityHeadersAndRevocation(t *testing.T) {
	setImageStudioTestEnv(t)
	identity, session := setupImageStudioSession(t, common.RoleAdminUser)
	token, _, err := service.IssueImageStudioToken(identity, "admin", time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodGet, "http://api.example.com/api/image-studio/identity", nil)
	request.AddCookie(&http.Cookie{Name: imageStudioCookieName, Value: token})
	router := gin.New()
	router.GET("/api/image-studio/identity", GetImageStudioIdentity)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "owui-1@identity.example.com", recorder.Header().Get("X-Studio-Email"))
	assert.Equal(t, "Studio%20%E4%B8%AD%E6%96%87%20User", recorder.Header().Get("X-Studio-Name"))
	assert.Equal(t, "admin", recorder.Header().Get("X-Studio-Role"))
	assert.Contains(t, recorder.Header().Get("Cache-Control"), "no-store")

	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", identity.UserID).Update("role", common.RoleCommonUser).Error)
	// Recreate the bridge request with an unchanged signed role; changing New API
	// permissions must not leave an Open WebUI administrator session usable.
	demotedRecorder := httptest.NewRecorder()
	router.ServeHTTP(demotedRecorder, request.Clone(request.Context()))
	assert.Equal(t, http.StatusUnauthorized, demotedRecorder.Code)

	_, err = model.RevokeUserSession(identity.UserID, identity.SessionID, "test")
	require.NoError(t, err)
	revokedRecorder := httptest.NewRecorder()
	router.ServeHTTP(revokedRecorder, request.Clone(request.Context()))
	assert.Equal(t, http.StatusUnauthorized, revokedRecorder.Code)
	assert.Contains(t, revokedRecorder.Header().Get("Cache-Control"), "no-store")
}

func TestImageStudioIdentityDisabledFailsClosed(t *testing.T) {
	t.Setenv("IMAGE_STUDIO_URL", "")
	t.Setenv("IMAGE_STUDIO_IDENTITY_DOMAIN", "identity.example.com")
	recorder := httptest.NewRecorder()
	router := gin.New()
	router.GET("/api/image-studio/identity", GetImageStudioIdentity)
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/image-studio/identity", nil))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Cache-Control"), "no-store")
}

func TestImageStudioBillingReturnsAuthenticatedUserQuota(t *testing.T) {
	setImageStudioTestEnv(t)
	identity, session := setupImageStudioSession(t, common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", identity.UserID).Updates(map[string]any{
		"quota":      123456,
		"used_quota": 654321,
	}).Error)
	token, _, err := service.IssueImageStudioToken(identity, "user", time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)

	router := gin.New()
	router.GET("/api/image-studio/billing", GetImageStudioBilling)
	request := httptest.NewRequest(http.MethodGet, "http://studio.example.com/api/image-studio/billing", nil)
	request.AddCookie(&http.Cookie{Name: imageStudioCookieName, Value: token})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		RemainingQuota int `json:"remaining_quota"`
		UsedQuota      int `json:"used_quota"`
		GiftQuota      int `json:"studio_gift_quota"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, 123456, response.RemainingQuota)
	assert.Equal(t, 654321, response.UsedQuota)
	assert.Equal(t, common.QuotaFromFloat(common.QuotaPerUnit), response.GiftQuota)
}
