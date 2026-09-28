package relay

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResolveOriginTaskReplacesChannelRatioOnFirstAttempt(t *testing.T) {
	oldDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() {
		model.DB = oldDB
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.Channel{}))
	channel := &model.Channel{Id: 2, Key: "test", Type: 1, Status: common.ChannelStatusEnabled, ChannelRatio: common.GetPointer(0.18)}
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, db.Create(&model.Task{UserId: 7, TaskID: "original", ChannelId: 2}).Error)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/videos/original/remix", nil)
	ctx.Params = gin.Params{{Key: "video_id", Value: "original"}}
	common.SetContextKey(ctx, constant.ContextKeyChannelRatio, common.GetPointer(0.15))
	info := &relaycommon.RelayInfo{
		UserId: 7, UserGroup: "regular", UsingGroup: "default",
		ChannelMeta:   &relaycommon.ChannelMeta{ChannelId: 1},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	require.Nil(t, ResolveOriginTask(ctx, info))
	require.Equal(t, 2, info.ChannelId)
	require.Equal(t, 0.18, helper.HandleGroupRatio(ctx, info).GroupRatio)
}
