package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func imageStudioResponseCookie(t *testing.T, recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response did not set cookie %q", name)
	return nil
}

func assertImageStudioCookieCleared(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	cookie := imageStudioResponseCookie(t, recorder, imageStudioCookieName)
	assert.Equal(t, "example.com", cookie.Domain)
	assert.Equal(t, "/", cookie.Path)
	assert.Less(t, cookie.MaxAge, 0)
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetupLoginClearsImageStudioCookie(t *testing.T) {
	setImageStudioTestEnv(t)
	identity, _ := setupImageStudioSession(t, common.RoleCommonUser)
	user, err := model.GetUserById(identity.UserID, false)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "https://api.example.com/api/user/login", nil)
	setupLogin(user, c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, imageStudioResponseCookie(t, recorder, service.RefreshCookieName))
	assertImageStudioCookieCleared(t, recorder)
}

func TestSetupLoginWithImageStudioDisabledDoesNotAffectMainLogin(t *testing.T) {
	setImageStudioTestEnv(t)
	identity, _ := setupImageStudioSession(t, common.RoleCommonUser)
	user, err := model.GetUserById(identity.UserID, false)
	require.NoError(t, err)
	t.Setenv("IMAGE_STUDIO_URL", "")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "https://api.example.com/api/user/login", nil)
	setupLogin(user, c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, imageStudioResponseCookie(t, recorder, service.RefreshCookieName))
	for _, cookie := range recorder.Result().Cookies() {
		assert.NotEqual(t, imageStudioCookieName, cookie.Name)
	}
}

func TestAuthLogoutClearsImageStudioCookieAndRevokesBridgeCredential(t *testing.T) {
	setImageStudioTestEnv(t)
	identity, session := setupImageStudioSession(t, common.RoleCommonUser)
	bridgeToken, _, err := service.IssueImageStudioToken(identity, "user", time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)
	accessToken, _, err := service.IssueAccessToken(identity)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "https://api.example.com/api/user/auth/logout", nil)
	c.Request.Header.Set("Authorization", "Bearer "+accessToken)
	c.Request.Header.Set("X-Auth-Session", identity.SessionID)
	AuthLogout(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	assertImageStudioCookieCleared(t, recorder)

	router := gin.New()
	router.GET("/api/image-studio/identity", GetImageStudioIdentity)
	identityRequest := httptest.NewRequest(http.MethodGet, "https://studio.example.com/api/image-studio/identity", nil)
	identityRequest.AddCookie(&http.Cookie{Name: imageStudioCookieName, Value: bridgeToken})
	identityRecorder := httptest.NewRecorder()
	router.ServeHTTP(identityRecorder, identityRequest)
	assert.Equal(t, http.StatusUnauthorized, identityRecorder.Code)
}

func TestAuthLogoutWithoutCredentialsClearsImageStudioCookie(t *testing.T) {
	setImageStudioTestEnv(t)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "https://api.example.com/api/user/auth/logout", nil)
	AuthLogout(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	assertImageStudioCookieCleared(t, recorder)
}

func TestAuthLogoutWithRefreshCookieClearsImageStudioCookie(t *testing.T) {
	setImageStudioTestEnv(t)
	identity, _ := setupImageStudioSession(t, common.RoleCommonUser)
	bundle, err := service.CreateLoginSession(identity.UserID, "password", "127.0.0.1", "test-agent")
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "https://api.example.com/api/user/auth/logout", nil)
	c.Request.AddCookie(&http.Cookie{Name: service.RefreshCookieName, Value: bundle.RefreshToken})
	AuthLogout(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	assertImageStudioCookieCleared(t, recorder)
	storedSession, err := model.GetUserSessionBySID(bundle.Session.SID)
	require.NoError(t, err)
	_, _, err = service.ValidateLoginSession(service.AuthIdentity{
		UserID:          identity.UserID,
		SessionID:       bundle.Session.SID,
		UserAuthVersion: storedSession.UserAuthVersion,
		SessionVersion:  storedSession.Version,
	})
	assert.ErrorIs(t, err, service.ErrLoginSessionRevoked)
}
