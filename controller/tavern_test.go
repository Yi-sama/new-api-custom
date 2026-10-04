package controller

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTavernAuthenticatedChatGiftAndWalletAccounting(t *testing.T) {
	initModelListColumnNames(t)
	identity, _ := setupImageStudioSession(t, common.RoleCommonUser)
	configureImageStudioRelayGroups(t)
	token, _, err := service.IssueAccessToken(identity)
	require.NoError(t, err)
	key := strings.Repeat("b", 64)
	keyFile := filepath.Join(t.TempDir(), "key")
	require.NoError(t, os.WriteFile(keyFile, []byte(key), 0600))
	t.Setenv("TAVERN_SERVICE_KEY_FILE", keyFile)
	originalMemory, originalBatch, originalLogs, originalRetries := common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.RetryTimes
	common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.RetryTimes = false, false, true, 0
	t.Cleanup(func() {
		common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.RetryTimes = originalMemory, originalBatch, originalLogs, originalRetries
	})
	require.NoError(t, model.DB.AutoMigrate(&model.TavernCredit{}, &model.Channel{}, &model.Ability{}, &model.UserSubscription{}, &model.ChannelStatusCheck{}))
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", identity.UserID).Update("quota", 0).Error)

	failUpstream := false
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		assert.Empty(t, r.Header.Get("X-Tavern-Service-Key"))
		assert.Equal(t, "Bearer fixture-upstream-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		if failUpstream {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"error":{"message":"fixture failed","type":"server_error"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"chatcmpl-tavern","object":"chat.completion","created":1,"model":"bridge-vip-model","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`))
	}))
	t.Cleanup(upstream.Close)
	channel := &model.Channel{
		Type: constant.ChannelTypeOpenAI, Key: "fixture-upstream-key",
		Status: common.ChannelStatusEnabled, Name: "tavern-fixture",
		Group: "vip", Models: "bridge-vip-model", BaseURL: &upstream.URL,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group: "vip", Model: "bridge-vip-model", ChannelId: channel.Id, Enabled: true,
	}).Error)

	router := gin.New()
	router.Use(middleware.UserAuth())
	router.POST("/pg/tavern/credit", middleware.TavernServiceAuth(true), ClaimTavernCredit)
	router.GET("/pg/tavern/credit", middleware.TavernServiceAuth(true), TavernCredit)
	router.POST("/pg/chat/completions", middleware.TavernServiceAuth(false), middleware.Distribute(), Playground)
	chatBody := `{"model":"bridge-vip-model","group":"vip","max_tokens":8,"messages":[{"role":"user","content":"hello"}]}`
	send := func(method, path, proof, credential, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+credential)
		request.Header.Set("X-Tavern-Service-Key", proof)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	assert.Equal(t, 401, send(http.MethodPost, "/pg/tavern/credit", key, "", "").Code)
	for _, proof := range []string{"", "browser-forged-key"} {
		assert.Equal(t, 403, send(http.MethodPost, "/pg/tavern/credit", proof, token, "").Code)
	}
	for i := range 2 {
		claim := send(http.MethodPost, "/pg/tavern/credit", key, token, `{"user_id":999,"amount":999}`)
		require.Equal(t, 200, claim.Code, claim.Body.String())
		var response struct {
			Data struct {
				Quota        int  `json:"quota"`
				NewlyGranted bool `json:"newly_granted"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(claim.Body.Bytes(), &response))
		assert.Equal(t, 500000, response.Data.Quota)
		assert.Equal(t, i == 0, response.Data.NewlyGranted)
	}
	denied := send(http.MethodPost, "/pg/chat/completions", "forged", token, chatBody)
	require.Equal(t, 403, denied.Code)
	assert.Zero(t, calls)
	giftChat := send(http.MethodPost, "/pg/chat/completions", key, token, chatBody)
	require.Equal(t, 200, giftChat.Code, giftChat.Body.String())
	credit, err := model.GetTavernCredit(identity.UserID)
	require.NoError(t, err)
	assert.Equal(t, 499995, credit.Quota)
	wallet, err := model.GetUserQuota(identity.UserID, false)
	require.NoError(t, err)
	assert.Zero(t, wallet)
	var log model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ?", identity.UserID).Order("id desc").First(&log).Error)
	assert.Equal(t, 5, log.Quota)
	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	assert.Equal(t, "tavern", other["billing_source"])
	assert.Equal(t, float64(5), other["tavern_gift_consumed"])
	assert.Equal(t, float64(0), other["wallet_quota_deducted"])

	require.NoError(t, model.DB.Model(&model.TavernCredit{}).Where("user_id = ?", identity.UserID).Update("quota", 2).Error)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", identity.UserID).Update("quota", 1000).Error)
	mixed := send(http.MethodPost, "/pg/chat/completions", key, token, chatBody)
	require.Equal(t, 200, mixed.Code, mixed.Body.String())
	wallet, err = model.GetUserQuota(identity.UserID, false)
	require.NoError(t, err)
	assert.Equal(t, 997, wallet)
	credit, err = model.GetTavernCredit(identity.UserID)
	require.NoError(t, err)
	assert.Zero(t, credit.Quota)

	require.NoError(t, model.DB.Model(&model.TavernCredit{}).Where("user_id = ?", identity.UserID).Update("quota", 100).Error)
	regular := send(http.MethodPost, "/pg/chat/completions", "", token, chatBody)
	require.Equal(t, 200, regular.Code, regular.Body.String())
	wallet, err = model.GetUserQuota(identity.UserID, false)
	require.NoError(t, err)
	assert.Equal(t, 992, wallet)
	credit, err = model.GetTavernCredit(identity.UserID)
	require.NoError(t, err)
	assert.Equal(t, 100, credit.Quota)

	failUpstream = true
	failed := send(http.MethodPost, "/pg/chat/completions", key, token, chatBody)
	require.NotEqual(t, 200, failed.Code)
	require.Eventually(t, func() bool {
		credit, giftErr := model.GetTavernCredit(identity.UserID)
		wallet, walletErr := model.GetUserQuota(identity.UserID, false)
		return giftErr == nil && walletErr == nil && credit.Quota == 100 && wallet == 992
	}, time.Second, time.Millisecond)
	claimAgain := send(http.MethodPost, "/pg/tavern/credit", key, token, "")
	require.Equal(t, 200, claimAgain.Code)
	assert.NotContains(t, claimAgain.Body.String(), `"newly_granted":true`)
	assert.NotContains(t, claimAgain.Body.String(), key)
}
