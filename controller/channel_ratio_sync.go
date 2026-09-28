package controller

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type channelRatioSyncRemoteResponse struct {
	Success    bool               `json:"success"`
	GroupRatio map[string]float64 `json:"group_ratio"`
}

type channelRatioSyncGroup struct {
	Name  string  `json:"name"`
	Ratio float64 `json:"ratio"`
}

func fetchChannelRatioSyncGroups(ctx context.Context, channel *model.Channel) ([]channelRatioSyncGroup, error) {
	settings := channel.GetOtherSettings()
	sourceURL := strings.TrimSpace(settings.ChannelRatioSyncSourceURL)
	if sourceURL == "" {
		sourceURL = channel.GetBaseURL()
	}
	sourceURL = strings.TrimRight(sourceURL, "/")
	if !strings.HasSuffix(sourceURL, "/api/pricing") {
		sourceURL += "/api/pricing"
	}
	if !strings.HasPrefix(sourceURL, "http://") && !strings.HasPrefix(sourceURL, "https://") {
		return nil, fmt.Errorf("ratio sync source URL must start with http:// or https://")
	}
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, err
	}
	client := service.GetHttpClient()
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ratio sync endpoint returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var payload channelRatioSyncRemoteResponse
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid ratio sync response: %w", err)
	}
	if !payload.Success || len(payload.GroupRatio) == 0 {
		return nil, fmt.Errorf("ratio sync response has no group ratios")
	}
	result := make([]channelRatioSyncGroup, 0, len(payload.GroupRatio))
	for name, ratio := range payload.GroupRatio {
		if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 || ratio > 10 {
			return nil, fmt.Errorf("remote ratio for %s is outside 0..10", name)
		}
		result = append(result, channelRatioSyncGroup{Name: name, Ratio: ratio})
	}
	return result, nil
}

func syncChannelRatio(ctx context.Context, channelID int, apply bool) (any, error) {
	return syncChannelRatioWithGroup(ctx, channelID, apply, "")
}

func syncChannelRatioWithGroup(ctx context.Context, channelID int, apply bool, remoteGroup string) (any, error) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, err
	}
	settings := channel.GetOtherSettings()
	groups, err := fetchChannelRatioSyncGroups(ctx, channel)
	if err != nil {
		settings.ChannelRatioSyncLastError = err.Error()
		channel.SetOtherSettings(settings)
		_ = model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("settings", channel.OtherSettings).Error
		return nil, err
	}
	if !apply {
		return gin.H{"channel_id": channel.Id, "channel_name": channel.Name, "groups": groups}, nil
	}
	selected := strings.TrimSpace(remoteGroup)
	if selected == "" {
		selected = strings.TrimSpace(settings.ChannelRatioSyncRemoteGroup)
	}
	if selected == "" {
		return nil, fmt.Errorf("remote group is required")
	}
	var ratio float64
	found := false
	for _, group := range groups {
		if group.Name == selected {
			ratio, found = group.Ratio, true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("remote group %s was not found", selected)
	}
	channel.ChannelRatio = &ratio
	settings.ChannelRatioSyncRemoteRatio = &ratio
	settings.ChannelRatioSyncLastSyncTime = common.GetTimestamp()
	settings.ChannelRatioSyncLastError = ""
	channel.SetOtherSettings(settings)
	if err := model.DB.Model(&model.Channel{}).Where("id = ?", channel.Id).
		Updates(map[string]any{"channel_ratio": ratio, "settings": channel.OtherSettings}).Error; err != nil {
		return nil, err
	}
	model.InitChannelCache()
	return gin.H{"channel_id": channel.Id, "channel_name": channel.Name, "group": selected, "ratio": ratio}, nil
}

func PreviewChannelRatioSync(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	data, err := syncChannelRatio(c.Request.Context(), id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func SyncChannelRatio(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request struct {
		RemoteGroup string `json:"remote_group"`
	}
	_ = c.ShouldBindJSON(&request)
	data, err := syncChannelRatioWithGroup(c.Request.Context(), id, true, request.RemoteGroup)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

type channelRatioSyncHandler struct{}

func (channelRatioSyncHandler) Type() string            { return "channel_ratio_sync" }
func (channelRatioSyncHandler) Enabled() bool           { return true }
func (channelRatioSyncHandler) Interval() time.Duration { return time.Minute }
func (channelRatioSyncHandler) NewPayload() any         { return nil }
func (channelRatioSyncHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	var channels []model.Channel
	err := model.DB.Find(&channels).Error
	var synced int
	if err == nil {
		for _, channel := range channels {
			if !channel.GetOtherSettings().ChannelRatioSyncEnabled {
				continue
			}
			if _, syncErr := syncChannelRatio(ctx, channel.Id, true); syncErr == nil {
				synced++
			}
		}
	}
	finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusSucceeded, gin.H{"synced": synced}, err)
}
