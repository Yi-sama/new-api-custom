package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestChannelRatioRangesIncludeFallbackAndSpecialOverride(t *testing.T) {
	oldGroups := ratio_setting.GroupRatio2JSONString()
	oldAdditions := ratio_setting.GroupRatioAddition2JSONString()
	oldSpecial := ratio_setting.GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioAdditionByJSONString(oldAdditions))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(oldSpecial))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"range-test":0.2}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioAdditionByJSONString(`{"range-test":0.01}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"range-test":0.1}}`))
	channels := []model.Channel{
		{Group: "range-test", ChannelRatio: common.GetPointer(0.15)},
		{Group: "range-test", ChannelRatio: common.GetPointer(0.18)},
	}
	require.Equal(t, ChannelRatioRange{Min: 0.16, Max: 0.19}, aggregateChannelRatioRanges(channels, "")["range-test"])
	require.Equal(t, ChannelRatioRange{Min: 0.11, Max: 0.11}, aggregateChannelRatioRanges(channels, "vip")["range-test"])
	channels = append(channels, model.Channel{Group: "range-test"})
	rangeWithFallback := aggregateChannelRatioRanges(channels, "")["range-test"]
	require.InDelta(t, 0.16, rangeWithFallback.Min, 1e-12)
	require.InDelta(t, 0.21, rangeWithFallback.Max, 1e-12)
}

func TestChannelRatioTaskSettlementUsesSnapshotAfterConfigChange(t *testing.T) {
	truncate(t)
	seedUser(t, 710, 10000)
	seedToken(t, 710, 710, "ratio-task", 10000)
	seedChannel(t, 710)
	old := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(old)) })
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
	for i, ratio := range []float64{0.15, 0} {
		task := makeTask(710, 710, 1000, 710, BillingSourceWallet, 0)
		task.TaskID = []string{"ratio-paid", "ratio-free"}[i]
		task.Group = "default"
		task.PrivateData.BillingContext = &model.TaskBillingContext{
			ModelRatio: 2, GroupRatio: ratio, OriginModelName: "removed-model",
			RatioInfo: &hosttypes.GroupRatioInfo{GroupRatio: ratio, ChannelRatio: common.GetPointer(ratio)},
		}
		require.NoError(t, model.DB.Create(task).Error)
		RecalculateTaskQuotaByTokens(context.Background(), task, 1000)
		require.Equal(t, []int{300, 0}[i], getTaskQuota(t, task.ID))
		other := taskBillingOther(task)
		require.Equal(t, ratio, other["channel_ratio"])
		require.Equal(t, ratio, other["effective_ratio"])
	}
}

func TestChannelRatioRangesExcludeDisabledChannels(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.Create(&[]model.Channel{
		{Group: "range-enabled", Status: 1, ChannelRatio: common.GetPointer(0.15)},
		{Group: "range-enabled", Status: 2, ChannelRatio: common.GetPointer(0.9)},
	}).Error)
	ranges, err := GetChannelRatioRanges("")
	require.NoError(t, err)
	require.Equal(t, ChannelRatioRange{Min: 0.15, Max: 0.15}, ranges["range-enabled"])
}
