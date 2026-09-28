package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelRatioUpdatePreservesOmittedAndClearsNull(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() {
		DB = originalDB
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))
	channel := Channel{Name: "A", Key: "test", Group: "default", Models: "gpt-4o", ChannelRatio: common.GetPointer(0.15)}
	require.NoError(t, channel.Insert())
	patch := Channel{Id: channel.Id, Name: "B"}
	require.NoError(t, patch.Update())
	require.Equal(t, 0.15, *patch.ChannelRatio)
	patch = Channel{Id: channel.Id, ChannelRatio: common.GetPointer(0.0)}
	require.NoError(t, patch.Update(true))
	require.NotNil(t, patch.ChannelRatio)
	require.Zero(t, *patch.ChannelRatio)
	patch = Channel{Id: channel.Id}
	require.NoError(t, patch.Update(true))
	require.Nil(t, patch.ChannelRatio)
	require.Equal(t, "B", patch.Name)
}

func TestTaskChannelRatioSnapshotRoundTripPreservesZeroAndLegacy(t *testing.T) {
	for _, ratio := range []float64{0, 0.18} {
		data := TaskPrivateData{BillingContext: &TaskBillingContext{
			GroupRatio: ratio,
			RatioInfo: &hosttypes.GroupRatioInfo{
				ChannelRatio: common.GetPointer(ratio), GroupRatio: ratio,
			},
		}}
		value, err := data.Value()
		require.NoError(t, err)
		var restored TaskPrivateData
		require.NoError(t, restored.Scan(value))
		require.Equal(t, data.BillingContext, restored.BillingContext)
	}

	var legacy TaskPrivateData
	require.NoError(t, legacy.Scan([]byte(`{"billing_context":{"group_ratio":0.2,"model_price":1}}`)))
	require.NotNil(t, legacy.BillingContext)
	require.Nil(t, legacy.BillingContext.RatioInfo)
	require.Equal(t, 0.2, legacy.BillingContext.GroupRatio)
}
