package controller

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

type nikoPricingResponse struct {
	Success     bool               `json:"success"`
	GroupRatio  map[string]float64 `json:"group_ratio"`
	UsableGroup map[string]string  `json:"usable_group"`
}

type NikoGroupPreview struct {
	FetchedAt   int64              `json:"fetched_at"`
	SourceURL   string             `json:"source_url"`
	GroupRatio  map[string]float64 `json:"group_ratio"`
	UsableGroup map[string]string  `json:"usable_group"`
}

type NikoGroupSyncItem struct {
	ID           string  `json:"id,omitempty"`
	LocalGroup   string  `json:"local_group"`
	RemoteGroup  string  `json:"remote_group"`
	Status       string  `json:"status"`
	RemoteRatio  float64 `json:"remote_ratio,omitempty"`
	AppliedRatio float64 `json:"applied_ratio,omitempty"`
	Error        string  `json:"error,omitempty"`
}

type NikoGroupSyncResult struct {
	FetchedAt         int64               `json:"fetched_at"`
	Applied           bool                `json:"applied"`
	Items             []NikoGroupSyncItem `json:"items"`
	RemoteCount       int                 `json:"remote_count"`
	GroupRatio        map[string]float64  `json:"group_ratio"`
	UsableGroup       map[string]string   `json:"usable_group"`
	UpdatedGroupRatio map[string]float64  `json:"updated_group_ratio,omitempty"`
}

var nikoGroupSyncMu sync.Mutex

func enabledNikoGroupMappings(mappings []model.NikoGroupMapping) []model.NikoGroupMapping {
	enabledMappings := make([]model.NikoGroupMapping, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping.Enabled {
			enabledMappings = append(enabledMappings, mapping)
		}
	}
	return enabledMappings
}

func syncNikoMappingsToUsableGroups(mappings []model.NikoGroupMapping, usableGroups map[string]string) []model.NikoGroupMapping {
	result := make([]model.NikoGroupMapping, 0, len(mappings))
	for _, mapping := range mappings {
		if localGroup := strings.TrimSpace(mapping.LocalGroup); localGroup != "" {
			_, selected := usableGroups[localGroup]
			mapping.Enabled = selected
		}
		if mapping.Enabled {
			result = append(result, mapping)
		}
	}
	return result
}

func readNikoConfig() (model.NikoGroupSyncConfig, error) {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[model.NikoGroupSyncOptionKey]
	common.OptionMapRWMutex.RUnlock()
	return model.ParseNikoGroupSyncConfig(common.Interface2String(raw))
}

func fetchNikoGroupRatios(ctx context.Context, cfg model.NikoGroupSyncConfig) (NikoGroupPreview, error) {
	if !strings.HasPrefix(cfg.SourceURL, "https://") && !strings.HasPrefix(cfg.SourceURL, "http://") {
		return NikoGroupPreview{}, fmt.Errorf("source_url must start with http:// or https://")
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, cfg.SourceURL, nil)
	if err != nil {
		return NikoGroupPreview{}, err
	}
	client := service.GetHttpClient()
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return NikoGroupPreview{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return NikoGroupPreview{}, fmt.Errorf("niko endpoint returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return NikoGroupPreview{}, err
	}
	var payload nikoPricingResponse
	if err := common.UnmarshalJsonStr(string(body), &payload); err != nil {
		return NikoGroupPreview{}, fmt.Errorf("invalid niko response: %w", err)
	}
	if !payload.Success {
		return NikoGroupPreview{}, fmt.Errorf("niko endpoint reported success=false")
	}
	if len(payload.GroupRatio) == 0 {
		return NikoGroupPreview{}, fmt.Errorf("niko response group_ratio is empty")
	}
	for name, ratio := range payload.GroupRatio {
		if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 || ratio > 10 {
			return NikoGroupPreview{}, fmt.Errorf("niko group_ratio %s is outside 0..10", name)
		}
	}
	return NikoGroupPreview{
		FetchedAt: common.GetTimestamp(), SourceURL: cfg.SourceURL,
		GroupRatio: payload.GroupRatio, UsableGroup: payload.UsableGroup,
	}, nil
}

func buildNikoSyncResult(cfg model.NikoGroupSyncConfig, preview NikoGroupPreview, apply bool) (NikoGroupSyncResult, map[string]float64) {
	current := ratio_setting.GetGroupRatioCopy()
	updated := make(map[string]float64, len(current))
	for k, v := range current {
		updated[k] = v
	}
	result := NikoGroupSyncResult{FetchedAt: preview.FetchedAt, Applied: apply, RemoteCount: len(preview.GroupRatio), Items: make([]NikoGroupSyncItem, 0, len(cfg.Mappings))}
	for i, mapping := range cfg.Mappings {
		item := NikoGroupSyncItem{ID: mapping.ID, LocalGroup: mapping.LocalGroup, RemoteGroup: mapping.RemoteGroup, Status: "skipped"}
		if item.ID == "" {
			item.ID = fmt.Sprintf("mapping-%d", i+1)
		}
		if strings.TrimSpace(mapping.LocalGroup) == "" || strings.TrimSpace(mapping.RemoteGroup) == "" {
			item.Error = "mapping incomplete"
			result.Items = append(result.Items, item)
			continue
		}
		if !mapping.Enabled {
			item.Error = "mapping disabled"
			result.Items = append(result.Items, item)
			continue
		}
		remoteRatio, found := preview.GroupRatio[mapping.RemoteGroup]
		if mapping.RatioMode == "manual" {
			if mapping.ManualRatio == nil {
				item.Status, item.Error = "failed", "manual_ratio is required"
				result.Items = append(result.Items, item)
				continue
			}
			remoteRatio = *mapping.ManualRatio
			found = true
		}
		if !found {
			item.Status, item.Error = "failed", "remote group not found"
			result.Items = append(result.Items, item)
			continue
		}
		applied := remoteRatio
		if mapping.RatioMode != "manual" {
			applied += cfg.RatioAdjustment
		}
		item.RemoteRatio, item.AppliedRatio = remoteRatio, applied
		if math.IsNaN(applied) || math.IsInf(applied, 0) || applied < 0 || applied > 10 {
			item.Status, item.Error = "failed", "applied ratio is outside 0..10"
			result.Items = append(result.Items, item)
			continue
		}
		if old, ok := current[mapping.LocalGroup]; ok && old == applied {
			item.Status = "unchanged"
		} else {
			updated[mapping.LocalGroup] = applied
			if apply {
				item.Status = "applied"
			} else {
				item.Status = "ready"
			}
		}
		result.Items = append(result.Items, item)
	}
	return result, updated
}

func runNikoGroupSync(ctx context.Context) (NikoGroupSyncResult, error) {
	nikoGroupSyncMu.Lock()
	defer nikoGroupSyncMu.Unlock()
	cfg, err := readNikoConfig()
	if err != nil {
		return NikoGroupSyncResult{}, err
	}
	preview, err := fetchNikoGroupRatios(ctx, cfg)
	if err != nil {
		return NikoGroupSyncResult{}, err
	}
	result, updated := buildNikoSyncResult(cfg, preview, false)
	result.GroupRatio = preview.GroupRatio
	result.UsableGroup = preview.UsableGroup
	result.UpdatedGroupRatio = updated
	return result, nil
}

func PreviewNikoGroupSync(c *gin.Context) {
	result, err := runNikoGroupSync(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func applyNikoBaseRatios(config model.NikoGroupSyncConfig, base, current map[string]float64) (map[string]float64, error) {
	effective := make(map[string]float64, len(current))
	for group, ratio := range current {
		effective[group] = ratio
	}
	for _, mapping := range config.Mappings {
		if !mapping.Enabled || strings.TrimSpace(mapping.LocalGroup) == "" {
			continue
		}
		ratio, ok := base[mapping.LocalGroup]
		if !ok || math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 || ratio > 10 {
			return nil, fmt.Errorf("base ratio for %s must be between 0 and 10", mapping.LocalGroup)
		}
		adjusted := ratio + config.RatioAdjustment
		if math.IsNaN(adjusted) || math.IsInf(adjusted, 0) || adjusted < 0 || adjusted > 10 {
			return nil, fmt.Errorf("effective ratio for %s must be between 0 and 10", mapping.LocalGroup)
		}
		effective[mapping.LocalGroup] = adjusted
	}
	return effective, nil
}

func SaveNikoGroupSync(c *gin.Context) {
	var request struct {
		Config     model.NikoGroupSyncConfig `json:"config"`
		GroupRatio map[string]float64        `json:"group_ratio"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	if len(request.Config.BaseGroupRatio) == 0 {
		request.Config.BaseGroupRatio = make(map[string]float64, len(request.GroupRatio))
		for group, ratio := range request.GroupRatio {
			request.Config.BaseGroupRatio[group] = ratio
		}
	}
	request.Config.Mappings = syncNikoMappingsToUsableGroups(
		request.Config.Mappings,
		setting.GetUserUsableGroupsCopy(),
	)
	configBytes, err := common.Marshal(request.Config)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.ValidateNikoGroupSyncConfig(string(configBytes)); err != nil {
		common.ApiError(c, err)
		return
	}
	effectiveGroupRatio, err := applyNikoBaseRatios(request.Config, request.Config.BaseGroupRatio, request.GroupRatio)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ratioBytes, err := common.Marshal(effectiveGroupRatio)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := ratio_setting.CheckGroupRatio(string(ratioBytes)); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOptionsBulk(map[string]string{
		model.NikoGroupSyncOptionKey: string(configBytes),
		"GroupRatio":                 string(ratioBytes),
	}); err != nil {
		common.ApiError(c, fmt.Errorf("persist Niko settings: %w", err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func NikoGroupSyncHandlerRun(ctx context.Context, task *model.SystemTask, runnerID string) {
	result, err := runNikoGroupSync(ctx)
	status := model.SystemTaskStatusSucceeded
	if err != nil {
		status = model.SystemTaskStatusFailed
	}
	finishSystemTaskHandler(task, runnerID, status, result, err)
}

type nikoGroupSyncHandler struct{}

func (nikoGroupSyncHandler) Type() string { return model.SystemTaskTypeNikoGroupSync }
func (nikoGroupSyncHandler) Enabled() bool {
	cfg, err := readNikoConfig()
	return err == nil && cfg.Enabled
}
func (nikoGroupSyncHandler) Interval() time.Duration { return 15 * time.Minute }
func (nikoGroupSyncHandler) NewPayload() any         { return map[string]bool{"manual": false} }
func (nikoGroupSyncHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	NikoGroupSyncHandlerRun(ctx, task, runnerID)
}
