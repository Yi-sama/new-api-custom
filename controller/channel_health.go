package controller

import (
	"context"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type channelHealthKeyResult struct {
	Index        int    `json:"index"`
	KeyPreview   string `json:"key_preview"`
	Status       string `json:"status"`
	ResponseTime int64  `json:"response_time_ms"`
	Error        string `json:"error,omitempty"`
}

type channelHealthResult struct {
	ChannelID int                      `json:"channel_id"`
	Name      string                   `json:"name"`
	Type      int                      `json:"type"`
	BaseURL   string                   `json:"base_url"`
	Status    string                   `json:"status"`
	Keys      []channelHealthKeyResult `json:"keys"`
}

func healthKeyPreview(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 10 {
		return key
	}
	return key[:6] + "..." + key[len(key)-4:]
}

func healthStatus(result testResult) (string, string) {
	if result.localErr == nil && result.newAPIError == nil {
		return "healthy", ""
	}
	if result.localErr != nil {
		return "error", result.localErr.Error()
	}
	if result.newAPIError != nil {
		return "error", result.newAPIError.Error()
	}
	return "unknown", ""
}

func testChannelHealth(ctx context.Context, channel *model.Channel, testUserID int) channelHealthResult {
	keys := channel.GetKeys()
	if len(keys) == 0 {
		keys = []string{""}
	}
	result := channelHealthResult{
		ChannelID: channel.Id,
		Name:      channel.Name,
		Type:      channel.Type,
		BaseURL:   channel.GetBaseURL(),
		Status:    "healthy",
		Keys:      make([]channelHealthKeyResult, 0, len(keys)),
	}
	for index, key := range keys {
		target := *channel
		target.Key = key
		target.Keys = nil
		target.ChannelInfo.IsMultiKey = false
		started := time.Now()
		probe := testChannel(ctx, &target, testUserID, "", "", shouldUseStreamForAutomaticChannelTest(&target))
		status, message := healthStatus(probe)
		result.Keys = append(result.Keys, channelHealthKeyResult{
			Index: index, KeyPreview: healthKeyPreview(key), Status: status,
			ResponseTime: time.Since(started).Milliseconds(), Error: message,
		})
		if status != "healthy" {
			result.Status = "partial"
		}
	}
	allHealthy := true
	for _, key := range result.Keys {
		if key.Status != "healthy" {
			allHealthy = false
			break
		}
	}
	if !allHealthy {
		healthyCount := 0
		for _, key := range result.Keys {
			if key.Status == "healthy" {
				healthyCount++
			}
		}
		if healthyCount == 0 {
			result.Status = "failed"
		}
	}
	return result
}

func GetChannelHealth(c *gin.Context) {
	testUserID, err := resolveChannelTestUserID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	results := make([]channelHealthResult, 0, len(channels))
	for _, channel := range channels {
		if channel == nil || channel.Status == common.ChannelStatusManuallyDisabled {
			continue
		}
		results = append(results, testChannelHealth(c.Request.Context(), channel, testUserID))
	}
	common.ApiSuccess(c, gin.H{"items": results, "checked_at": common.GetTimestamp()})
}
