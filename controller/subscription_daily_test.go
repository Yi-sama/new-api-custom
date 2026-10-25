package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSubscriptionPlanControllerTestDB(t *testing.T) {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.SubscriptionPlan{}))
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		_ = sqlDB.Close()
	})
}

func TestAdminSubscriptionPlanDailySettingsPersistAndValidate(t *testing.T) {
	setupSubscriptionPlanControllerTestDB(t)
	confirmPaymentComplianceForTest(t)
	router := gin.New()
	router.POST("/plans", AdminCreateSubscriptionPlan)
	router.PUT("/plans/:id", AdminUpdateSubscriptionPlan)

	createBody := `{"plan":{"title":"Daily","price_amount":1,"duration_unit":"day","duration_value":30,"enabled":true,"total_amount":30000000,"quota_grant_mode":"daily","daily_amount":1000000,"quota_expiry_policy":"retain","quota_reset_period":"daily"}}`
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, httptest.NewRequest(http.MethodPost, "/plans", strings.NewReader(createBody)))
	require.Equal(t, http.StatusOK, createRecorder.Code, createRecorder.Body.String())
	var plan model.SubscriptionPlan
	require.NoError(t, model.DB.Where("title = ?", "Daily").First(&plan).Error)
	assert.Equal(t, model.SubscriptionQuotaGrantDaily, plan.QuotaGrantMode)
	assert.EqualValues(t, 1_000_000, plan.DailyAmount)
	assert.Equal(t, model.SubscriptionQuotaExpiryRetain, plan.QuotaExpiryPolicy)
	assert.Equal(t, model.SubscriptionResetNever, plan.QuotaResetPeriod)
	assert.Zero(t, plan.QuotaResetCustomSeconds)

	updateBody := `{"plan":{"title":"Daily updated","price_amount":2,"duration_unit":"day","duration_value":30,"enabled":true,"total_amount":30000000,"quota_grant_mode":"daily","daily_amount":2000000,"quota_expiry_policy":"expire","quota_reset_period":"weekly"}}`
	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, httptest.NewRequest(http.MethodPut, "/plans/1", strings.NewReader(updateBody)))
	require.Equal(t, http.StatusOK, updateRecorder.Code, updateRecorder.Body.String())
	require.NoError(t, model.DB.First(&plan, plan.Id).Error)
	assert.EqualValues(t, 2_000_000, plan.DailyAmount)
	assert.Equal(t, model.SubscriptionQuotaExpiryExpire, plan.QuotaExpiryPolicy)
	assert.Equal(t, model.SubscriptionResetNever, plan.QuotaResetPeriod)

	invalidBody := `{"plan":{"title":"Invalid","price_amount":1,"duration_unit":"day","duration_value":30,"enabled":true,"total_amount":0,"quota_grant_mode":"daily","daily_amount":0,"quota_expiry_policy":"retain"}}`
	invalidRecorder := httptest.NewRecorder()
	router.ServeHTTP(invalidRecorder, httptest.NewRequest(http.MethodPost, "/plans", strings.NewReader(invalidBody)))
	assert.Contains(t, invalidRecorder.Body.String(), "每日发放套餐总额度")
}
