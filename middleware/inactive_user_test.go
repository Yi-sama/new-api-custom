package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenAuthRecordsAPIActivityBeforeFailedUpstream(t *testing.T) {
	db := setupAutoModelResolverTest(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	user := model.User{Username: "activity-auth", Password: "unused", Group: "default",
		Status: common.UserStatusEnabled, Role: common.RoleCommonUser, AuthVersion: 1}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&model.Token{UserId: user.Id, Key: "activitytestkey",
		Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}).Error)
	engine := gin.New()
	engine.GET("/v1/models", TokenAuth(), func(c *gin.Context) {
		var current model.User
		require.NoError(t, db.First(&current, user.Id).Error)
		assert.Positive(t, current.LastAPICallAt)
		c.Status(http.StatusBadGateway)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer sk-activitytestkey")
	engine.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusBadGateway, recorder.Code, recorder.Body.String())

	require.NoError(t, db.Model(&user).Update("last_api_call_at", 0).Error)
	request.Header.Set("Authorization", "Bearer sk-invalid")
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Zero(t, user.LastAPICallAt)
}
