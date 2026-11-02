package controller

import (
	"net/http"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func statusImageGroups(channels []*model.Channel) []model.ChannelStatusGroupConfig {
	names := make(map[string]bool)
	for group := range ratio_setting.GetGroupRatioCopy() {
		if name := strings.TrimSpace(group); name != "" {
			names[name] = true
		}
	}
	active := make([]*model.Channel, 0, len(channels))
	for _, channel := range channels {
		if channel == nil || channel.Status == common.ChannelStatusManuallyDisabled {
			continue
		}
		active = append(active, channel)
	}
	groups := make([]model.ChannelStatusGroupConfig, 0, len(names))
	for name := range names {
		groups = append(groups, model.ChannelStatusGroupConfig{
			Group: name, Enabled: true, Models: model.SortedChannelStatusModels(active, name),
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Group < groups[j].Group })
	return groups
}

// Read-only presentation data; never probes models or changes account scheduling.
func GetStatusImageOverview(c *gin.Context) {
	kind := c.DefaultQuery("kind", "channel")
	if kind != "channel" && kind != "sub" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid report kind"})
		return
	}
	common.OptionMapRWMutex.RLock()
	raw := common.Interface2String(common.OptionMap[model.StatusImageConfigOptionKey])
	common.OptionMapRWMutex.RUnlock()
	config := model.DefaultStatusImageConfig()
	if strings.TrimSpace(raw) != "" {
		if err := model.ValidateStatusImageConfig(raw); err != nil {
			common.ApiError(c, err)
			return
		}
		// Preserve absent independent filters so older saved selections still apply.
		config = model.StatusImageConfig{}
		if err := common.UnmarshalJsonStr(raw, &config); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	data := gin.H{"config": config, "checked_at": common.GetTimestamp()}
	if kind == "sub" {
		data["sub"] = model.GetSubStatusSummary(c.Request.Context())
		common.ApiSuccess(c, data)
		return
	}
	cfg, err := readChannelStatusConfig()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// Image visibility is independent of the paid/active monitoring selection.
	cfg.Groups = statusImageGroups(channels)
	views, err := buildChannelStatusOverview(cfg, channels)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for i := range views {
		for j := range views[i].Models {
			for k := range views[i].Models[j].Cards {
				views[i].Models[j].Cards[k].Error = ""
			}
		}
	}
	data["channel"] = gin.H{
		"enabled": cfg.Enabled, "mode": cfg.Mode,
		"groups": views, "checked_at": common.GetTimestamp(),
	}
	common.ApiSuccess(c, data)
}
