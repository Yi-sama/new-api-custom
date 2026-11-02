package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDailyTokenPoolAudioSettlementReleasesReservation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previousDB := model.DB
	previousLog, previousBatch := common.LogConsumeEnabled, common.BatchUpdateEnabled
	model.DB = db
	common.LogConsumeEnabled, common.BatchUpdateEnabled = false, false
	t.Cleanup(func() {
		model.DB = previousDB
		common.LogConsumeEnabled, common.BatchUpdateEnabled = previousLog, previousBatch
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&model.DailyTokenPoolDay{}, &model.DailyTokenPoolReservation{}, &model.User{}, &model.Channel{}))
	config := model.DefaultDailyTokenPoolConfig()
	config.Enabled = true
	now := time.Now()
	id, err := model.ReserveDailyTokenPool(config, 50, now)
	require.NoError(t, err)
	info := &relaycommon.RelayInfo{
		DailyTokenPoolReservationID: id,
		StartTime:                   now, FirstResponseTime: now,
		OriginModelName: "gpt-4o-audio-preview",
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	usage := &dto.Usage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10}
	usage.PromptTokensDetails.AudioTokens = 7
	usage.CompletionTokenDetails.AudioTokens = 3

	PostAudioConsumeQuota(ctx, info, usage, "")

	status, err := model.GetDailyTokenPoolStatus(config, now)
	require.NoError(t, err)
	assert.Equal(t, int64(10), status.UsedTokens)
	assert.Zero(t, status.ReservedTokens)
	assert.Empty(t, info.DailyTokenPoolReservationID)
}

func TestDailyTokenPoolOutputReservationHonorsProtocolAndMultipleChoices(t *testing.T) {
	limit := uint(12)
	choices := 3
	for _, test := range []struct {
		name    string
		request dto.Request
		want    int64
	}{
		{"chat defaults to bounded output", &dto.GeneralOpenAIRequest{}, 4096},
		{"chat counts every choice", &dto.GeneralOpenAIRequest{MaxTokens: &limit, N: &choices}, 36},
		{"completion limit takes precedence", &dto.GeneralOpenAIRequest{MaxTokens: &limit, MaxCompletionTokens: &limit}, 12},
		{"responses uses its limit", &dto.OpenAIResponsesRequest{MaxOutputTokens: &limit}, 12},
		{"claude defaults to bounded output", &dto.ClaudeRequest{}, 4096},
		{"gemini defaults to bounded output", &dto.GeminiChatRequest{}, 4096},
		{"embeddings has no output reservation", &dto.EmbeddingRequest{}, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual, err := dailyPoolOutputLimit(test.request)
			require.NoError(t, err)
			assert.Equal(t, test.want, actual)
			if actual > 0 {
				assert.Greater(t, test.request.GetTokenCountMeta().MaxTokens, 0)
			}
		})
	}
	_, err := dailyPoolOutputLimit(&dto.AudioRequest{})
	assert.Error(t, err)
}
