package common

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenFastModeSupportedRequestsOnly(t *testing.T) {
	originalGroups := setting.FastModeAllowedGroups2JSONString()
	t.Cleanup(func() { require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(originalGroups)) })
	require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(`["gpt-paid"]`))
	for _, tc := range []struct {
		name    string
		model   string
		channel int
		mode    int
		enabled bool
		want    bool
	}{
		{"default off", "gpt-6.1-sol", constant.ChannelTypeOpenAI, relayconstant.RelayModeResponses, false, false},
		{"responses", "gpt-6.1-sol", constant.ChannelTypeOpenAI, relayconstant.RelayModeResponses, true, true},
		{"codex", "gpt-5.6-sol", constant.ChannelTypeCodex, relayconstant.RelayModeResponses, true, true},
		{"chat snapshot", "gpt-4.1-2025-04-14", constant.ChannelTypeOpenAI, relayconstant.RelayModeChatCompletions, true, true},
		{"image", "gpt-image-2", constant.ChannelTypeOpenAI, relayconstant.RelayModeImagesGenerations, true, false},
		{"embedding", "text-embedding-3-small", constant.ChannelTypeOpenAI, relayconstant.RelayModeEmbeddings, true, false},
		{"other provider", "claude-opus-5", constant.ChannelTypeOpenAI, relayconstant.RelayModeChatCompletions, true, false},
		{"fine tune", "ft:gpt-4.1:org:name:id", constant.ChannelTypeOpenAI, relayconstant.RelayModeChatCompletions, true, false},
		{"compact", "gpt-6.1-sol", constant.ChannelTypeOpenAI, relayconstant.RelayModeResponsesCompact, true, false},
		{"realtime", "gpt-4o-realtime-preview", constant.ChannelTypeOpenAI, relayconstant.RelayModeRealtime, true, false},
		{"non OpenAI channel", "gpt-6.1-sol", constant.ChannelTypeAnthropic, relayconstant.RelayModeChatCompletions, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := &RelayInfo{TokenFastMode: tc.enabled, RelayMode: tc.mode, UsingGroup: "gpt-paid",
				ChannelMeta: &ChannelMeta{ChannelType: tc.channel, UpstreamModelName: tc.model}}
			info.ConfigureFastMode()
			assert.Equal(t, tc.want, info.FastMode)
			ratio := 1.0
			if tc.want {
				ratio = 1.5
			}
			assert.Equal(t, ratio, info.FastModeBillingRatio())
		})
	}
}

func TestTokenFastModeUsesSelectedGroupAndResetsOnRetry(t *testing.T) {
	originalGroups := setting.FastModeAllowedGroups2JSONString()
	t.Cleanup(func() { require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(originalGroups)) })
	require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(`["gpt-paid"]`))
	info := &RelayInfo{
		TokenFastMode: true, TokenGroup: "auto", UserGroup: "default", UsingGroup: "gpt-paid",
		RelayMode:   relayconstant.RelayModeResponses,
		ChannelMeta: &ChannelMeta{ChannelType: constant.ChannelTypeOpenAI, UpstreamModelName: "gpt-6.1-sol"},
	}
	info.ConfigureFastMode()
	require.True(t, info.FastMode)
	info.FastModeRequested = true
	info.FastModeTier = "priority"
	info.FastModeProtocol = "codex-oauth-priority-v1"
	info.UsingGroup = "other"
	info.ConfigureFastMode()
	assert.False(t, info.FastMode)
	assert.False(t, info.FastModeRequested)
	assert.Empty(t, info.FastModeTier)
	assert.Empty(t, info.FastModeProtocol)
	assert.Equal(t, 1.0, info.FastModeBillingRatio())
	reader := strings.NewReader(`{"model":"gpt-6.1-sol","input":"hello"}`)
	body, closer, err := info.FastModeRequestBody(reader)
	require.NoError(t, err)
	assert.Same(t, reader, body, "disallowed groups must not receive injected priority")
	assert.Nil(t, closer)
	info.TokenGroup = "gpt-paid"
	info.UsingGroup = "auto"
	info.ConfigureFastMode()
	assert.False(t, info.FastMode, "the Key group must not override the selected group")
	info.UsingGroup = "gpt-paid"
	require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(`[]`))
	info.ConfigureFastMode()
	assert.False(t, info.FastMode, "empty selection disables all Fast requests")
}

func TestTokenFastModeFinalBodyAndDowngrade(t *testing.T) {
	info := &RelayInfo{FastMode: true}
	raw := `{"model":"gpt-6.1-sol","service_tier":"flex","store":false,"temperature":0,"input":"hello","extra":9007199254740993}`
	body, closer, err := info.FastModeRequestBody(strings.NewReader(raw))
	require.NoError(t, err)
	require.NotNil(t, closer)
	defer closer.Close()
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"gpt-6.1-sol","service_tier":"priority","store":false,"temperature":0,"input":"hello","extra":9007199254740993}`, string(data))
	assert.True(t, info.FastModeRequested)
	info.PriceData.AddOtherRatio("fast_mode", 1.5)
	info.ObserveFastModeTier([]byte(`{"type":"response.completed","response":{"service_tier":"default"}}`))
	assert.Equal(t, 1.0, info.FastModeBillingRatio())
	assert.Equal(t, 100.0, info.PriceData.ApplyOtherRatiosToFloat(100))
	info.ObserveFastModeTier([]byte(`{"service_tier":"fast"}`))
	assert.Equal(t, 150.0, info.PriceData.ApplyOtherRatiosToFloat(100), "a second tier event must not stack the multiplier")
}

func TestTokenFastModeOffLeavesBodyAndBillingRulesUnchanged(t *testing.T) {
	info := &RelayInfo{}
	raw := `{"model":"gpt-6.1-sol", "service_tier":"flex"}`
	reader := strings.NewReader(raw)
	body, closer, err := info.FastModeRequestBody(reader)
	require.NoError(t, err)
	assert.Same(t, reader, body)
	assert.Nil(t, closer)
	standard, err := info.FastModeStandardBillingBody([]byte(raw))
	require.NoError(t, err)
	assert.Equal(t, raw, string(standard))
}

func TestTokenFastModeProtocolTrustAndRetry(t *testing.T) {
	for _, tc := range []struct {
		name      string
		trusted   bool
		requested bool
		marker    string
		tier      string
		ratio     float64
	}{
		{"trusted OAuth default echo", true, true, "codex-oauth-priority-v1", "default", 1.5},
		{"untrusted marker", false, true, "codex-oauth-priority-v1", "default", 1},
		{"missing marker", true, true, "", "default", 1},
		{"unknown protocol", true, true, "unknown", "default", 1},
		{"Fast not sent", true, false, "codex-oauth-priority-v1", "default", 1},
		{"explicit flex remains cheaper", true, true, "codex-oauth-priority-v1", "flex", 1},
		{"explicit standard remains cheaper", true, true, "codex-oauth-priority-v1", "standard", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := &RelayInfo{FastMode: true, FastModeRequested: tc.requested, FastModeTier: tc.tier,
				ChannelMeta: &ChannelMeta{ChannelSetting: dto.ChannelSettings{TrustSub2APIFastProtocol: tc.trusted}}}
			header := make(http.Header)
			header.Set("X-Sub2API-Fast-Protocol", tc.marker)
			info.ObserveFastModeProtocol(header)
			assert.Equal(t, tc.ratio, info.FastModeBillingRatio())
			info.ObserveFastModeProtocol(nil)
			assert.Empty(t, info.FastModeProtocol, "a later attempt must not inherit OAuth provenance")
			assert.Equal(t, 1.0, info.FastModeBillingRatio())
		})
	}
}
