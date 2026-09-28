package helper

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelRatioPrecedenceAndSelection(t *testing.T) {
	oldGroups := ratio_setting.GroupRatio2JSONString()
	oldAdditions := ratio_setting.GroupRatioAddition2JSONString()
	oldSpecial := ratio_setting.GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioAdditionByJSONString(oldAdditions))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(oldSpecial))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"ratio-test":0.9}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioAdditionByJSONString(`{"ratio-test":0.01}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"ratio-test":0.1}}`))
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{UserGroup: "regular", UsingGroup: "ratio-test"}
	for _, ratio := range []*float64{common.GetPointer(0.15), common.GetPointer(0.18), common.GetPointer(0.0), nil} {
		common.SetContextKey(ctx, constant.ContextKeyChannelRatio, ratio)
		got := HandleGroupRatio(ctx, info)
		expected := 0.91
		if ratio != nil {
			expected = *ratio + 0.01
		}
		require.Equal(t, expected, got.GroupRatio)
		require.Equal(t, ratio, got.ChannelRatio)
	}
	common.SetContextKey(ctx, constant.ContextKeyChannelRatio, common.GetPointer(0.18))
	info.UserGroup = "vip"
	got := HandleGroupRatio(ctx, info)
	require.Equal(t, 0.11, got.GroupRatio)
	require.Equal(t, 0.18, *got.ChannelRatio)
	require.True(t, got.HasSpecialRatio)
}

func TestChannelRatioPriceRefreshAcrossFreeAndPaidChannels(t *testing.T) {
	old := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(old)) })
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"channel-ratio-test":1}`))
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserGroup: "ratio-test", UsingGroup: "default", OriginModelName: "channel-ratio-test",
	}
	for _, ratio := range []float64{0.15, 0, 0.18} {
		common.SetContextKey(ctx, constant.ContextKeyChannelRatio, common.GetPointer(ratio))
		price, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
		require.NoError(t, err)
		require.Equal(t, ratio, price.GroupRatioInfo.GroupRatio)
		if ratio == 0 {
			require.Zero(t, price.QuotaToPreConsume)
		} else {
			require.Positive(t, price.QuotaToPreConsume)
			require.False(t, price.FreeModel)
		}
	}
}
