package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type qqBenefitHTTPFixture struct {
	router *gin.Engine
	user   *model.User
	token  string
	secret string
}

func setupQQBenefitHTTP(t *testing.T) qqBenefitHTTPFixture {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedis, previousSecret := common.RedisEnabled, common.SessionSecret
	previousQuota, previousType := common.QuotaPerUnit, common.MainDatabaseType()
	previousSecure, previousOrigins := common.SessionCookieSecure, common.SessionCookieTrustedURLs
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.Log{},
		&model.QQBenefitRequest{}, &model.QQBenefitBinding{}))
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled, common.SessionSecret, common.QuotaPerUnit = false, "qq-benefit-http-session-secret", 500000
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.SessionCookieSecure = true
	common.SessionCookieTrustedURLs = []string{"https://site.example"}
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled, common.SessionSecret, common.QuotaPerUnit = previousRedis, previousSecret, previousQuota
		common.SetMainDatabaseType(previousType)
		common.SessionCookieSecure, common.SessionCookieTrustedURLs = previousSecure, previousOrigins
		require.NoError(t, sqlDB.Close())
	})
	secret := strings.Repeat("test-bot-secret-", 4)
	t.Setenv("QQ_BENEFIT_BOT_SECRET", secret)
	t.Setenv("QQ_BENEFIT_GROUP_ID", "1128511890")
	user := &model.User{Username: "qq-http-user", Password: "unused", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, AuthVersion: 1, Group: "default", Quota: 11}
	require.NoError(t, db.Create(user).Error)
	session, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "qq-benefit-http-test")
	require.NoError(t, err)
	router := gin.New()
	bot := router.Group("/api/qq-benefits/bot", middleware.QQBenefitBotAuth(), middleware.DisableCache())
	bot.POST("/start", StartQQBenefit)
	bot.POST("/status", GetQQBenefitBotStatus)
	bot.POST("/confirm", ConfirmQQBenefit)
	web := router.Group("/api/user", middleware.UserAuth(), middleware.DisableCache())
	web.GET("/qq-binding", GetQQBenefitStatus)
	web.POST("/qq-binding/code", middleware.SessionCookieOriginGuard(), CreateQQBenefitCode)
	return qqBenefitHTTPFixture{router, user, session.AccessToken, secret}
}

func (fixture qqBenefitHTTPFixture) request(t *testing.T, path, body, credential string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if credential == "bot" {
		stamp := strconv.FormatInt(time.Now().Unix(), 10)
		mac := hmac.New(sha256.New, []byte(fixture.secret))
		mac.Write([]byte(stamp + "\nPOST\n" + path + "\n" + body))
		request.Header.Set("X-QQ-Benefit-Time", stamp)
		request.Header.Set("X-QQ-Benefit-Signature", hex.EncodeToString(mac.Sum(nil)))
	} else if credential == "web" {
		request.Header.Set("Authorization", "Bearer "+fixture.token)
		request.Header.Set("Origin", "https://site.example")
	}
	response := httptest.NewRecorder()
	fixture.router.ServeHTTP(response, request)
	return response
}

func TestQQBenefitHTTPFullBindingCreditsOnce(t *testing.T) {
	fixture := setupQQBenefitHTTP(t)
	response := fixture.request(t, "/api/user/qq-binding/code", `{"qq":"100001"}`, "web")
	assert.Contains(t, response.Body.String(), "QQ_BENEFIT_NEED_START")
	response = fixture.request(t, "/api/qq-benefits/bot/start",
		`{"qq":"100001","group_id":"1128511890","message_id":"message-1"}`, "bot")
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"bound":false`)
	response = fixture.request(t, "/api/user/qq-binding/code", `{"qq":"100001"}`, "web")
	var codeResponse struct {
		Success bool `json:"success"`
		Data    struct {
			Code string `json:"code"`
			QQ   string `json:"qq"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &codeResponse))
	require.True(t, codeResponse.Success)
	assert.Equal(t, "100001", codeResponse.Data.QQ)
	require.Regexp(t, `^[0-9]{6}$`, codeResponse.Data.Code)
	body, err := common.Marshal(qqBenefitBotInput{QQ: "100001", GroupID: "1128511890", Code: codeResponse.Data.Code})
	require.NoError(t, err)
	for _, reward := range []string{`"reward_granted":true`, `"reward_granted":false`} {
		response = fixture.request(t, "/api/qq-benefits/bot/confirm", string(body), "bot")
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"bound":true`)
		assert.Contains(t, response.Body.String(), reward)
	}
	require.NoError(t, model.DB.First(fixture.user, fixture.user.Id).Error)
	assert.Equal(t, 500011, fixture.user.Quota)
	binding, err := model.GetQQBenefitBinding("100001", 0)
	require.NoError(t, err)
	require.NotNil(t, binding)
	for i := 0; i < 2; i++ {
		response = fixture.request(t, "/api/qq-benefits/bot/start",
			`{"qq":"100001","group_id":"1128511890","message_id":"message-again"}`, "bot")
		var repeated struct {
			Success bool `json:"success"`
			Data    struct {
				Bound     bool  `json:"bound"`
				AwardedAt int64 `json:"awarded_at"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &repeated))
		assert.True(t, repeated.Success)
		assert.True(t, repeated.Data.Bound)
		assert.Equal(t, binding.AwardedAt.Unix(), repeated.Data.AwardedAt)
	}
	require.NoError(t, model.DB.First(fixture.user, fixture.user.Id).Error)
	assert.Equal(t, 500011, fixture.user.Quota)
	response = fixture.request(t, "/api/qq-benefits/bot/status", `{"qq":"100001","group_id":"1128511890"}`, "bot")
	assert.Contains(t, response.Body.String(), `"bound":true`)
	request := httptest.NewRequest(http.MethodGet, "/api/user/qq-binding", nil)
	request.Header.Set("Authorization", "Bearer "+fixture.token)
	recorder := httptest.NewRecorder()
	fixture.router.ServeHTTP(recorder, request)
	assert.Contains(t, recorder.Body.String(), `"qq":"100001"`)
	assert.NotContains(t, recorder.Body.String(), "user_id")
	assert.NotContains(t, recorder.Body.String(), "session")
}

func TestQQBenefitHTTPRejectsAnonymousWrongGroupAndCrossOrigin(t *testing.T) {
	fixture := setupQQBenefitHTTP(t)
	assert.Equal(t, http.StatusUnauthorized,
		fixture.request(t, "/api/user/qq-binding/code", `{"qq":"100001"}`, "").Code)
	assert.Equal(t, http.StatusUnauthorized,
		fixture.request(t, "/api/qq-benefits/bot/start", `{"qq":"100001","group_id":"1128511890","message_id":"1"}`, "").Code)
	response := fixture.request(t, "/api/qq-benefits/bot/start",
		`{"qq":"100001","group_id":"1125690897","message_id":"1"}`, "bot")
	assert.Contains(t, response.Body.String(), "QQ_BENEFIT_INVALID")
	request := httptest.NewRequest(http.MethodPost, "/api/user/qq-binding/code", strings.NewReader(`{"qq":"100001"}`))
	request.Header.Set("Authorization", "Bearer "+fixture.token)
	request.Header.Set("Origin", "https://other.example")
	recorder := httptest.NewRecorder()
	fixture.router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	var count int64
	require.NoError(t, model.DB.Model(&model.QQBenefitRequest{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestQQBenefitHTTPGroupAllowlist(t *testing.T) {
	fixture := setupQQBenefitHTTP(t)
	for _, test := range []struct {
		name    string
		config  string
		group   string
		allowed bool
	}{
		{"legacy test", "1128511890", "1128511890", true},
		{"legacy formal denied", "1128511890", "1125690897", false},
		{"default test", "", "1128511890", true},
		{"default formal denied", "", "1125690897", false},
		{"both test", "1128511890,1125690897", "1128511890", true},
		{"both formal", "1128511890, 1125690897 ", "1125690897", true},
		{"third group denied", "1128511890,1125690897", "999999", false},
		{"partial group denied", "1128511890,1125690897", "112569089", false},
		{"private denied", "1128511890,1125690897", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("QQ_BENEFIT_GROUP_ID", test.config)
			body, err := common.Marshal(qqBenefitBotInput{QQ: "100001", GroupID: test.group})
			require.NoError(t, err)
			response := fixture.request(t, "/api/qq-benefits/bot/status", string(body), "bot")
			require.Equal(t, http.StatusOK, response.Code)
			if test.allowed {
				assert.Contains(t, response.Body.String(), `"bound":false`)
			} else {
				assert.Contains(t, response.Body.String(), "QQ_BENEFIT_INVALID")
			}
		})
	}
}
