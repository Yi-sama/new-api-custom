package controller

import (
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

func TestDailyTokenPoolLegacySettingsSavePreservesFreeFunding(t *testing.T) {
	setupImageStudioSession(t, common.RoleRootUser)
	config := model.DefaultDailyTokenPoolConfig()
	config.Enabled, config.FreeUsage = true, true
	config.Groups, config.ChannelIDs = []string{"shared"}, []int{34}
	require.NoError(t, model.SaveDailyTokenPoolConfig(config))
	payload := map[string]interface{}{
		"enabled": true, "title": config.Title, "daily_limit": config.DailyLimit,
		"reset_timezone": config.ResetTimezone, "groups": config.Groups,
		"channel_ids": config.ChannelIDs, "public_visible": true,
	}
	for _, explicitPaid := range []bool{false, true} {
		if explicitPaid {
			payload["free_usage"] = false
		}
		body, err := common.Marshal(payload)
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPut, "/api/daily-token-pool/admin", strings.NewReader(string(body)))
		ctx.Request.Header.Set("Content-Type", "application/json")
		UpdateDailyTokenPool(ctx)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		stored, err := model.GetDailyTokenPoolConfig()
		require.NoError(t, err)
		assert.Equal(t, !explicitPaid, stored.FreeUsage)
	}
}
