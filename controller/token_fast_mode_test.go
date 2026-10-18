package controller

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenFastModeAvailabilityFollowsEffectiveGroups(t *testing.T) {
	configureTokenAutoGroupsTest(t, "1", `["vip"]`)
	user := setupTokenAutoGroupsControllerTest(t)
	previous := setting.FastModeAllowedGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(previous))
	})
	tests := []struct {
		name   string
		group  string
		auto   string
		policy string
		want   bool
	}{
		{name: "allowed explicit", group: "vip", policy: `["vip"]`, want: true},
		{name: "denied explicit", group: "default", policy: `["vip"]`},
		{name: "empty policy", group: "vip", policy: `[]`},
		{name: "inherited allowed", policy: `["default"]`, want: true},
		{name: "inherited denied", policy: `["vip"]`},
		{name: "global auto allowed", group: "auto", policy: `["vip"]`, want: true},
		{name: "custom auto denied without global fallback", group: "auto", auto: `["default"]`, policy: `["vip"]`},
		{name: "custom auto allowed", group: "auto", auto: `["vip"]`, policy: `["vip"]`, want: true},
		{name: "custom auto permission revoked", group: "auto", auto: `["removed"]`, policy: `["removed"]`},
		{name: "custom auto current limit", group: "auto", auto: `["default","vip"]`, policy: `["vip"]`},
		{name: "malformed auto fails closed", group: "auto", auto: `{`, policy: `["vip"]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(test.policy))
			token := &model.Token{UserId: user.Id, Key: "private-token-key", Group: test.group, AutoGroups: test.auto, FastMode: true}
			data, err := common.Marshal(buildMaskedTokenResponse(token))
			require.NoError(t, err)
			var response map[string]any
			require.NoError(t, common.Unmarshal(data, &response))
			assert.Equal(t, test.want, response["fast_mode_available"])
			assert.True(t, token.FastMode, "Eligibility must not overwrite the saved switch")
			assert.NotEqual(t, token.Key, response["key"])
		})
	}
}

func tokenSettingsRequest(token *model.Token) map[string]any {
	return map[string]any{
		"id":                   token.Id,
		"name":                 token.Name,
		"status":               token.Status,
		"expired_time":         token.ExpiredTime,
		"remain_quota":         token.RemainQuota,
		"unlimited_quota":      token.UnlimitedQuota,
		"model_limits_enabled": token.ModelLimitsEnabled,
		"model_limits":         token.ModelLimits,
		"group":                token.Group,
		"cross_group_retry":    token.CrossGroupRetry,
	}
}

func TestTokenFastModeReturnedByListSearchAndGet(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	previous := setting.FastModeAllowedGroups2JSONString()
	require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(`["default"]`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(previous))
	})
	token := seedToken(t, db, 1, "fast-mode-visible", "fast-mode-visible-key")
	token.FastMode = true
	token.Group = "default"
	require.NoError(t, db.Save(token).Error)

	listCtx, listRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/?p=1&size=10", nil, 1)
	GetAllTokens(listCtx)
	listResponse := decodeAPIResponse(t, listRecorder)
	require.True(t, listResponse.Success, listResponse.Message)
	var listData struct {
		Items []struct {
			ID                int  `json:"id"`
			FastMode          bool `json:"fast_mode"`
			FastModeAvailable bool `json:"fast_mode_available"`
		} `json:"items"`
	}
	require.NoError(t, common.Unmarshal(listResponse.Data, &listData))
	require.Len(t, listData.Items, 1)
	assert.Equal(t, token.Id, listData.Items[0].ID)
	assert.True(t, listData.Items[0].FastMode)
	assert.True(t, listData.Items[0].FastModeAvailable)

	searchCtx, searchRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/search?keyword=fast-mode-visible&p=1&size=10", nil, 1)
	SearchTokens(searchCtx)
	searchResponse := decodeAPIResponse(t, searchRecorder)
	require.True(t, searchResponse.Success, searchResponse.Message)
	var searchData struct {
		Items []struct {
			FastMode          bool `json:"fast_mode"`
			FastModeAvailable bool `json:"fast_mode_available"`
		} `json:"items"`
	}
	require.NoError(t, common.Unmarshal(searchResponse.Data, &searchData))
	require.Len(t, searchData.Items, 1)
	assert.True(t, searchData.Items[0].FastMode)
	assert.True(t, searchData.Items[0].FastModeAvailable)

	getCtx, getRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/"+strconv.Itoa(token.Id), nil, 1)
	getCtx.Params = append(getCtx.Params, gin.Param{Key: "id", Value: strconv.Itoa(token.Id)})
	GetToken(getCtx)
	getResponse := decodeAPIResponse(t, getRecorder)
	require.True(t, getResponse.Success, getResponse.Message)
	var getData struct {
		FastMode          bool `json:"fast_mode"`
		FastModeAvailable bool `json:"fast_mode_available"`
	}
	require.NoError(t, common.Unmarshal(getResponse.Data, &getData))
	assert.True(t, getData.FastMode)
	assert.True(t, getData.FastModeAvailable)
}

func TestAddTokenFastModeDefaultsOffAndAcceptsTrue(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	requests := []struct {
		name     string
		fastMode *bool
		want     bool
	}{
		{name: "omitted defaults off", want: false},
		{name: "explicit true", fastMode: common.GetPointer(true), want: true},
	}
	for _, test := range requests {
		t.Run(test.name, func(t *testing.T) {
			body := map[string]any{
				"name":            "create-fast-mode-" + test.name,
				"expired_time":    -1,
				"unlimited_quota": true,
				"group":           "default",
			}
			if test.fastMode != nil {
				body["fast_mode"] = *test.fastMode
			}
			ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
			AddToken(ctx)
			require.True(t, decodeAPIResponse(t, recorder).Success)
			var persisted model.Token
			require.NoError(t, db.Where("name = ?", body["name"]).First(&persisted).Error)
			assert.Equal(t, test.want, persisted.FastMode)
		})
	}
}

func TestUpdateTokenFastModeOnlyRequiresOwnerAndPreservesSettings(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "fast-mode-settings", "fast-mode-settings-key")
	token.RemainQuota = 321
	token.UnlimitedQuota = false
	token.ModelLimitsEnabled = true
	token.ModelLimits = "gpt-test"
	token.Group = "default"
	require.NoError(t, db.Save(token).Error)

	missingCtx, missingRecorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/?fast_mode_only=true", map[string]any{"id": token.Id}, 1)
	UpdateToken(missingCtx)
	assert.False(t, decodeAPIResponse(t, missingRecorder).Success)

	otherCtx, otherRecorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/?fast_mode_only=true", map[string]any{"id": token.Id, "fast_mode": true}, 2)
	UpdateToken(otherCtx)
	assert.False(t, decodeAPIResponse(t, otherRecorder).Success)

	for _, fastMode := range []bool{true, false} {
		ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/?fast_mode_only=true", map[string]any{"id": token.Id, "fast_mode": fastMode}, 1)
		UpdateToken(ctx)
		response := decodeAPIResponse(t, recorder)
		require.True(t, response.Success, response.Message)
		var persisted model.Token
		require.NoError(t, db.First(&persisted, token.Id).Error)
		assert.Equal(t, fastMode, persisted.FastMode)
		assert.Equal(t, token.Name, persisted.Name)
		assert.Equal(t, token.RemainQuota, persisted.RemainQuota)
		assert.Equal(t, token.UnlimitedQuota, persisted.UnlimitedQuota)
		assert.Equal(t, token.ModelLimitsEnabled, persisted.ModelLimitsEnabled)
		assert.Equal(t, token.ModelLimits, persisted.ModelLimits)
	}
}

func TestUpdateTokenOmitsFastModeWithoutResettingIt(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "preserve-fast-mode", "preserve-fast-mode-key")
	token.FastMode = true
	require.NoError(t, db.Save(token).Error)

	body := tokenSettingsRequest(token)
	body["name"] = "preserve-fast-mode-updated"
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)
	require.True(t, decodeAPIResponse(t, recorder).Success)
	var persisted model.Token
	require.NoError(t, db.First(&persisted, token.Id).Error)
	assert.True(t, persisted.FastMode)
}
