package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInactiveCleanupRejectsInvalidConfirmationWithoutDeleting(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	user := model.User{Username: "inactive-protected", Password: "unused", CreatedAt: 100, APIActivityStartedAt: 100}
	require.NoError(t, db.Create(&user).Error)
	for _, body := range []string{
		`{"days":0,"cutoff":200,"confirmed":true,"all":true}`,
		`{"days":1.5,"cutoff":200,"confirmed":true,"all":true}`,
		`{"days":3651,"cutoff":200,"confirmed":true,"all":true}`,
		`{"days":30,"cutoff":200,"confirmed":false,"all":true}`,
		`{"days":30,"cutoff":200,"confirmed":true}`,
		`{"days":30,"cutoff":200,"confirmed":true,"ids":[-1]}`,
		`{"days":30,"cutoff":200,"confirmed":true,"all":true,"ids":[1]}`,
		fmt.Sprintf(`{"days":30,"cutoff":%d,"confirmed":true,"all":true}`, common.GetTimestamp()),
	} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/inactive-users/delete", strings.NewReader(body))
		DeleteInactiveUsers(c)
		var response struct {
			Success bool `json:"success"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		assert.False(t, response.Success, body)
	}
	var count int64
	require.NoError(t, db.Model(&model.User{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestInactiveCleanupBulkDeletesOnlyEligibleAccounts(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.UserSession{}, &model.AuthFlow{}, &model.ExternalIdentityClaim{},
		&model.Token{}, &model.PasskeyCredential{}, &model.TwoFA{}, &model.TwoFABackupCode{},
		&model.UserOAuthBinding{}, &model.Log{}))
	now := common.GetTimestamp()
	for i, role := range []int{common.RoleCommonUser, common.RoleCommonUser, common.RoleAdminUser, common.RoleRootUser} {
		user := model.User{Id: i + 1, Username: fmt.Sprintf("cleanup-%d", i), AffCode: fmt.Sprintf("cleanup-%d", i), Password: "unused", Role: role,
			CreatedAt: now - 90*86400, APIActivityStartedAt: now - 90*86400}
		if i == 1 {
			user.LastAPICallAt = now
		}
		require.NoError(t, db.Create(&user).Error)
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", 4)
	c.Set("role", common.RoleRootUser)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/inactive-users/delete",
		strings.NewReader(fmt.Sprintf(`{"days":30,"cutoff":%d,"confirmed":true,"all":true}`, now-30*86400)))
	DeleteInactiveUsers(c)
	var response struct {
		Success bool                                   `json:"success"`
		Data    struct{ Deleted, Skipped, Failed int } `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, 1, response.Data.Deleted)
	assert.Zero(t, response.Data.Failed)
	var remaining []int
	require.NoError(t, db.Model(&model.User{}).Order("id").Pluck("id", &remaining).Error)
	assert.Equal(t, []int{2, 3, 4}, remaining)
}
