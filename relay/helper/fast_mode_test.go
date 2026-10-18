package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenFastModePricesStandardExactlyOnce(t *testing.T) {
	originalGroups := setting.FastModeAllowedGroups2JSONString()
	t.Cleanup(func() { require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(originalGroups)) })
	require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(`["default"]`))
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(k, v string) error { saved[k] = v; return nil }))
	ratios, err := common.Marshal(ratio_setting.GetModelRatioCopy())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(ratios)))
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"gpt-6.1-sol":1,"claude-opus-5":1}`))

	for _, tiered := range []bool{false, true} {
		t.Run(map[bool]string{false: "ratio", true: "tiered"}[tiered], func(t *testing.T) {
			mode, expr := `{}`, `{}`
			if tiered {
				mode = `{"gpt-6.1-sol":"tiered_expr"}`
				expr = `{"gpt-6.1-sol":"param(\"service_tier\") == \"priority\" ? tier(\"fast\", p * 4) : tier(\"standard\", p * 2)"}`
			}
			require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
				"billing_setting.billing_mode": mode, "billing_setting.billing_expr": expr,
			}))
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-6.1-sol","service_tier":"priority"}`))
			ctx.Request.Header.Set("Content-Type", "application/json")
			common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
			common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "gpt-6.1-sol")
			info := &relaycommon.RelayInfo{TokenFastMode: true, RelayMode: relayconstant.RelayModeResponses,
				OriginModelName: "gpt-6.1-sol", UserGroup: "default", UsingGroup: "default"}
			info.InitChannelMeta(ctx)
			require.NoError(t, ModelMappedHelper(ctx, info, nil))
			info.ConfigureFastMode()
			meta := &types.TokenCountMeta{MaxTokens: 1}
			price, err := ModelPriceHelper(ctx, info, 1000, meta)
			require.NoError(t, err)
			require.True(t, info.FastMode)
			assert.Equal(t, 1.5, price.OtherRatios()["fast_mode"])
			if tiered {
				assert.Equal(t, 1500, price.QuotaToPreConsume, "Standard 1000 x 1.5, not Fast 2000 x 1.5")
				require.NotNil(t, info.TieredBillingSnapshot)
				assert.Equal(t, "standard", info.TieredBillingSnapshot.EstimatedTier)
			} else {
				assert.Equal(t, 1501, price.QuotaToPreConsume)
			}
			priceAgain, err := ModelPriceHelper(ctx, info, 1000, meta)
			require.NoError(t, err)
			assert.Equal(t, price.QuotaToPreConsume, priceAgain.QuotaToPreConsume, "retry repricing must not stack")
		})
	}
}
