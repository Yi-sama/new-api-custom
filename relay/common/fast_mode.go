package common

import (
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/QuantumNous/new-api/constant"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const FastModeRatio = 1.5

// Restrict opt-in to text models; images, realtime, pro and fine-tunes stay unchanged.
var fastModeModel = regexp.MustCompile(`^(gpt-(4o|4\.1)(-mini)?|gpt-5(\.[0-9]+)?(-mini|-codex|-codex-mini|-codex-max|-sol|-terra|-luna)?|gpt-6(\.[0-9]+)?(-sol|-astra|-luna)?|o3(-mini)?|o4-mini)(-[0-9]{4}-[0-9]{2}-[0-9]{2})?$`)

func (info *RelayInfo) ConfigureFastMode() {
	info.FastMode = info.TokenFastMode && setting.IsFastModeAllowedForGroup(info.UsingGroup) && info.ChannelMeta != nil &&
		(info.ChannelType == constant.ChannelTypeOpenAI || info.ChannelType == constant.ChannelTypeCodex) &&
		(info.RelayMode == relayconstant.RelayModeChatCompletions || info.RelayMode == relayconstant.RelayModeResponses) &&
		fastModeModel.MatchString(info.UpstreamModelName)
	info.FastModeRequested = false
	info.FastModeTier = ""
	info.FastModeProtocol = ""
}

func (info *RelayInfo) FastModeBillingRatio() float64 {
	if info.FastMode {
		if info.FastModeTier == "" || info.FastModeTier == "priority" || info.FastModeTier == "fast" ||
			(info.FastModeTier == "default" && info.FastModeProtocol == "codex-oauth-priority-v1") {
			return FastModeRatio
		}
	}
	return 1
}

// The marker describes the selected credential and final outbound tier, not
// proof of accelerated execution. Never read it from client request headers.
func (info *RelayInfo) ObserveFastModeProtocol(header http.Header) {
	info.FastModeProtocol = ""
	if info.FastModeRequested && info.ChannelMeta != nil &&
		info.ChannelSetting.TrustSub2APIFastProtocol &&
		header.Get("X-Sub2API-Fast-Protocol") == "codex-oauth-priority-v1" {
		info.FastModeProtocol = "codex-oauth-priority-v1"
	}
}

// Apply after filtering, overrides and passthrough selection so per-key policy wins.
func (info *RelayInfo) FastModeRequestBody(body io.Reader) (io.Reader, io.Closer, error) {
	if !info.FastMode {
		return body, nil, nil
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, nil, err
	}
	if !gjson.ValidBytes(data) || !gjson.ParseBytes(data).IsObject() {
		return nil, nil, fmt.Errorf("Fast mode requires a JSON object")
	}
	if !fastModeModel.MatchString(gjson.GetBytes(data, "model").String()) {
		return nil, nil, fmt.Errorf("Fast mode model changed to an unsupported model")
	}
	data, err = sjson.SetBytes(data, "service_tier", "priority")
	if err != nil {
		return nil, nil, err
	}
	outbound, closer, err := NewOutboundJSONBody(data)
	if err == nil {
		info.FastModeRequested = true
	}
	return outbound, closer, err
}

func (info *RelayInfo) ObserveFastModeTier(data []byte) {
	if !info.FastModeRequested {
		return
	}
	tier := gjson.GetBytes(data, "service_tier")
	if !tier.Exists() {
		tier = gjson.GetBytes(data, "response.service_tier")
	}
	if tier.Type == gjson.String && tier.String() != "" {
		info.FastModeTier = tier.String()
		info.PriceData.AddOtherRatio("fast_mode", info.FastModeBillingRatio())
	}
}

// Price Fast keys against Standard request rules, then apply the site's 1.5x once.
func (info *RelayInfo) FastModeStandardBillingBody(data []byte) ([]byte, error) {
	if !info.FastMode {
		return data, nil
	}
	if len(data) == 0 {
		data = []byte(`{}`)
	}
	if !gjson.ValidBytes(data) || !gjson.ParseBytes(data).IsObject() {
		return nil, fmt.Errorf("Fast billing requires a JSON object")
	}
	return sjson.SetBytes(data, "service_tier", "default")
}
