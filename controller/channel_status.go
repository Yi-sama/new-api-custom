package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const (
	channelStatusNormalAvailabilityThreshold   = 75.0
	channelStatusDegradedAvailabilityThreshold = 50.0
	channelStatusSourceUser                    = "user"
	channelStatusSourceAPI                     = "api"
)

type channelStatusCard struct {
	ChannelID        int                  `json:"channel_id"`
	ChannelName      string               `json:"channel_name"`
	ChannelType      int                  `json:"channel_type"`
	Status           string               `json:"status"`
	Availability24h  float64              `json:"availability_24h"`
	LatestLatencyMs  int64                `json:"latest_latency_ms"`
	AverageLatencyMs float64              `json:"average_latency_ms"`
	Blocks           []channelStatusBlock `json:"blocks"`
	LastCheckedAt    int64                `json:"last_checked_at"`
	Error            string               `json:"error,omitempty"`
}

type channelStatusBlock struct {
	StartedAt int64  `json:"started_at"`
	Status    string `json:"status"`
}

type channelStatusModelView struct {
	Model string              `json:"model"`
	Cards []channelStatusCard `json:"cards"`
}

type channelStatusGroupView struct {
	Group  string                   `json:"group"`
	Models []channelStatusModelView `json:"models"`
}

type channelStatusGroupOption struct {
	Group  string   `json:"group"`
	Models []string `json:"models"`
}

func aggregateChannelStatusChecks(checks []model.ChannelStatusCheck) []model.ChannelStatusCheck {
	if len(checks) < 2 {
		return checks
	}
	byRequest := make(map[string]model.ChannelStatusCheck, len(checks))
	for _, check := range checks {
		requestID := check.RequestID
		if requestID == "" {
			requestID = fmt.Sprintf("legacy-%d", check.ID)
		}
		current, exists := byRequest[requestID]
		if !exists {
			byRequest[requestID] = check
			continue
		}
		currentSuccess := current.Status == "normal" || current.Status == "degraded"
		checkSuccess := check.Status == "normal" || check.Status == "degraded"
		switch {
		case checkSuccess && !currentSuccess:
			byRequest[requestID] = check
		case checkSuccess == currentSuccess && check.CheckedAt >= current.CheckedAt:
			byRequest[requestID] = check
		}
	}
	result := make([]model.ChannelStatusCheck, 0, len(byRequest))
	for _, check := range byRequest {
		result = append(result, check)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].CheckedAt < result[j].CheckedAt })
	return result
}

func readChannelStatusConfig() (model.ChannelStatusConfig, error) {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[model.ChannelStatusConfigOptionKey]
	common.OptionMapRWMutex.RUnlock()
	return model.ParseChannelStatusConfig(common.Interface2String(raw))
}

func loadChannelStatusChecks(channelID int, group string, modelName string, source string) ([]model.ChannelStatusCheck, error) {
	var checks []model.ChannelStatusCheck
	err := model.DB.Where(map[string]any{
		"channel_id": channelID,
		"group":      group,
		"model":      modelName,
	}).Where("source = ?", source).Where("checked_at >= ?", common.GetTimestamp()-86400).Order("checked_at asc").Find(&checks).Error
	return checks, err
}

func GetChannelStatusGroups(c *gin.Context) {
	// Keep the candidate list consistent with the existing group selector:
	// configured groups are authoritative even when no channel is currently
	// assigned to them. Include channel groups as well for installations that
	// have not added a ratio entry yet.
	groups := make(map[string]bool)
	for group := range ratio_setting.GetGroupRatioCopy() {
		if name := strings.TrimSpace(group); name != "" {
			groups[name] = true
		}
	}
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		for _, group := range channel.GetGroups() {
			if name := strings.TrimSpace(group); name != "" {
				groups[name] = true
			}
		}
	}
	items := make([]channelStatusGroupOption, 0, len(groups))
	for group := range groups {
		models := model.SortedChannelStatusModels(channels, group)
		items = append(items, channelStatusGroupOption{Group: group, Models: models})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Group < items[j].Group })
	common.ApiSuccess(c, gin.H{"items": items})
}

func channelStatusGroupEnabled(cfg model.ChannelStatusConfig, group string) (model.ChannelStatusGroupConfig, bool) {
	for _, item := range cfg.Groups {
		if item.Group == group {
			return item, item.Enabled
		}
	}
	return model.ChannelStatusGroupConfig{}, false
}

func channelHasGroup(channel *model.Channel, group string) bool {
	if channel == nil {
		return false
	}
	for _, value := range channel.GetGroups() {
		if strings.TrimSpace(value) == group {
			return true
		}
	}
	return false
}

func channelHasModel(channel *model.Channel, target string) bool {
	for _, value := range channel.GetModels() {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func channelStatusFromChecks(checks []model.ChannelStatusCheck) channelStatusCard {
	card := channelStatusCard{Status: "unknown", Blocks: make([]channelStatusBlock, 12)}
	cutoff := common.GetTimestamp() - int64(12*time.Hour/time.Second)
	for i := range card.Blocks {
		card.Blocks[i] = channelStatusBlock{StartedAt: cutoff + int64(i)*int64(time.Hour/time.Second), Status: "unknown"}
	}
	if len(checks) == 0 {
		return card
	}
	successes := 0
	var latencyTotal int64
	for _, check := range checks {
		if check.Status == "normal" || check.Status == "degraded" {
			successes++
			if check.LatencyMs > 0 {
				latencyTotal += check.LatencyMs
			}
		}
		if check.CheckedAt >= card.LastCheckedAt {
			card.LatestLatencyMs, card.LastCheckedAt, card.Error = check.LatencyMs, check.CheckedAt, check.Error
		}
	}
	card.Availability24h = float64(successes) / float64(len(checks)) * 100
	switch {
	case card.Availability24h >= channelStatusNormalAvailabilityThreshold:
		card.Status = "normal"
	case card.Availability24h >= channelStatusDegradedAvailabilityThreshold:
		card.Status = "degraded"
	default:
		card.Status = "failed"
	}
	if successes > 0 {
		card.AverageLatencyMs = float64(latencyTotal) / float64(successes)
	}
	for i := range card.Blocks {
		bucketStart := card.Blocks[i].StartedAt
		bucketEnd := bucketStart + int64(time.Hour/time.Second)
		for _, check := range checks {
			if check.CheckedAt >= bucketStart && check.CheckedAt < bucketEnd {
				card.Blocks[i].Status = check.Status
				if card.Blocks[i].Status == "degraded" {
					card.Blocks[i].Status = "normal"
				}
			}
		}
	}
	return card
}

func GetChannelStatusOverview(c *gin.Context) {
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
	views := make([]channelStatusGroupView, 0)
	for _, groupCfg := range cfg.Groups {
		if !groupCfg.Enabled {
			continue
		}
		groupView := channelStatusGroupView{
			Group:  groupCfg.Group,
			Models: make([]channelStatusModelView, 0),
		}
		for _, selectedModel := range groupCfg.Models {
			modelView := channelStatusModelView{
				Model: selectedModel,
				Cards: make([]channelStatusCard, 0),
			}
			checks := make([]model.ChannelStatusCheck, 0)
			hasChannel := false
			for _, channel := range channels {
				if channel == nil || channel.Status == common.ChannelStatusManuallyDisabled || !channelHasGroup(channel, groupCfg.Group) || !channelHasModel(channel, selectedModel) {
					continue
				}
				hasChannel = true
				channelChecks, err := loadChannelStatusChecks(channel.Id, groupCfg.Group, selectedModel, channelStatusSourceUser)
				if err != nil {
					common.ApiError(c, err)
					return
				}
				checks = append(checks, channelChecks...)
			}
			if cfg.Mode == model.ChannelStatusModeAPI || (cfg.Mode == model.ChannelStatusModeUserThenAPI && len(checks) == 0) {
				checks = make([]model.ChannelStatusCheck, 0)
				for _, channel := range channels {
					if channel == nil || channel.Status == common.ChannelStatusManuallyDisabled || !channelHasGroup(channel, groupCfg.Group) || !channelHasModel(channel, selectedModel) {
						continue
					}
					channelChecks, err := loadChannelStatusChecks(channel.Id, groupCfg.Group, selectedModel, channelStatusSourceAPI)
					if err != nil {
						common.ApiError(c, err)
						return
					}
					checks = append(checks, channelChecks...)
				}
			}
			if hasChannel {
				modelView.Cards = append(modelView.Cards, channelStatusFromChecks(aggregateChannelStatusChecks(checks)))
			}
			groupView.Models = append(groupView.Models, modelView)
		}
		views = append(views, groupView)
	}
	common.ApiSuccess(c, gin.H{"enabled": cfg.Enabled, "mode": cfg.Mode, "groups": views, "checked_at": common.GetTimestamp()})
}

func runChannelStatusMonitor(ctx context.Context) (int, error) {
	cfg, err := readChannelStatusConfig()
	if err != nil || !cfg.Enabled || cfg.Mode == model.ChannelStatusModeUser {
		return 0, err
	}
	testUserID, err := resolveChannelTestUserID(nil)
	if err != nil {
		return 0, err
	}
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, groupCfg := range cfg.Groups {
		if !groupCfg.Enabled {
			continue
		}
		for _, selectedModel := range groupCfg.Models {
			if cfg.Mode == model.ChannelStatusModeUserThenAPI {
				var userCount int64
				if err := model.DB.Model(&model.ChannelStatusCheck{}).Where("group = ? AND model = ? AND source = ? AND checked_at >= ?", groupCfg.Group, selectedModel, channelStatusSourceUser, common.GetTimestamp()-86400).Count(&userCount).Error; err != nil {
					return count, err
				}
				if userCount > 0 {
					continue
				}
			}
			requestID := common.GetUUID()
			for _, channel := range channels {
				if ctx != nil && ctx.Err() != nil {
					return count, ctx.Err()
				}
				if channel == nil || channel.Status == common.ChannelStatusManuallyDisabled || !channelHasGroup(channel, groupCfg.Group) || !channelHasModel(channel, selectedModel) {
					continue
				}
				started := time.Now()
				result := testChannel(ctx, channel, testUserID, selectedModel, "", shouldUseStreamForAutomaticChannelTest(channel))
				status := "failed"
				errorText := ""
				if result.newAPIError == nil && result.localErr == nil {
					status = "normal"
				} else if result.newAPIError != nil {
					errorText = common.LocalLogPreview(result.newAPIError.MaskSensitiveErrorWithStatusCode())
				} else if result.localErr != nil {
					errorText = common.LocalLogPreview(result.localErr.Error())
				}
				if err := model.DB.Create(&model.ChannelStatusCheck{
					RequestID: requestID,
					ChannelID: channel.Id,
					Group:     groupCfg.Group,
					Model:     selectedModel,
					Status:    status,
					LatencyMs: time.Since(started).Milliseconds(),
					Error:     errorText,
					Source:    channelStatusSourceAPI,
					CheckedAt: common.GetTimestamp(),
				}).Error; err != nil {
					return count, err
				}
				count++
			}
		}
	}
	return count, nil
}

func ChannelStatusMonitorHandlerRun(ctx context.Context, task *model.SystemTask, runnerID string) {
	count, err := runChannelStatusMonitor(ctx)
	status := model.SystemTaskStatusSucceeded
	if err != nil {
		status = model.SystemTaskStatusFailed
	}
	finishSystemTaskHandler(task, runnerID, status, map[string]int{"checked": count}, err)
}

type channelStatusMonitorHandler struct{}

func (channelStatusMonitorHandler) Type() string { return model.SystemTaskTypeChannelStatus }
func (channelStatusMonitorHandler) Enabled() bool {
	cfg, err := readChannelStatusConfig()
	return err == nil && cfg.Enabled && cfg.Mode != model.ChannelStatusModeUser
}
func (channelStatusMonitorHandler) Interval() time.Duration {
	return 5 * time.Minute
}
func (channelStatusMonitorHandler) NewPayload() any { return nil }
func (channelStatusMonitorHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	ChannelStatusMonitorHandlerRun(ctx, task, runnerID)
}

func recordUserChannelStatus(c *gin.Context, info *relaycommon.RelayInfo, channel *model.Channel, success bool, latency time.Duration, requestErr *types.NewAPIError) {
	if c == nil || info == nil || channel == nil || info.IsChannelTest || !common.GetContextKeyBool(c, constant.ContextKeyUpstreamRequestAttempted) {
		return
	}
	status := "failed"
	errorText := ""
	if success {
		status = "normal"
	} else if requestErr != nil {
		errorText = common.LocalLogPreview(requestErr.MaskSensitiveErrorWithStatusCode())
	}
	if err := model.DB.Create(&model.ChannelStatusCheck{
		ChannelID: channel.Id,
		RequestID: common.GetContextKeyString(c, constant.ContextKeyChannelStatusRequestID),
		Group:     info.UsingGroup,
		Model:     info.OriginModelName,
		Status:    status,
		LatencyMs: latency.Milliseconds(),
		Error:     errorText,
		Source:    channelStatusSourceUser,
		CheckedAt: common.GetTimestamp(),
	}).Error; err != nil {
		common.SysError(fmt.Sprintf("保存用户渠道状态统计失败: %v", err))
	}
}
