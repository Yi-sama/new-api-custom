package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestFetchNikoGroupRatiosParsesTopLevelGroupRatio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"group_ratio":{"default":0.6,"codex-混池":0.06,"codex-Plus":0.11,"codex-Pro":0.15},"usable_group":{"codex-混池":"team+Plus+Pro大乱炖","codex-Plus":"plus池子","codex-Pro":"纯codex-Pro号池"}}`))
	}))
	defer server.Close()

	cfg := model.DefaultNikoGroupSyncConfig()
	cfg.SourceURL = server.URL
	preview, err := fetchNikoGroupRatios(context.Background(), cfg)
	require.NoError(t, err)
	require.Equal(t, 4, len(preview.GroupRatio))
	require.Equal(t, 0.15, preview.GroupRatio["codex-Pro"])
	require.Equal(t, "plus池子", preview.UsableGroup["codex-Plus"])
}

func TestParseNikoGroupSyncConfigPreservesCustomMappings(t *testing.T) {
	raw := `{"mappings":[{"id":"custom","enabled":true,"local_group":"default","remote_group":"codex-Base","ratio_mode":"remote"}]}`
	cfg, err := model.ParseNikoGroupSyncConfig(raw)
	require.NoError(t, err)
	require.Len(t, cfg.Mappings, 1)
	require.Equal(t, "default", cfg.Mappings[0].LocalGroup)
	require.Equal(t, "codex-Base", cfg.Mappings[0].RemoteGroup)
}

func TestParseNikoGroupSyncConfigPreservesIncompleteDraftMappings(t *testing.T) {
	raw := `{"mappings":[{"id":"local-only","enabled":true,"local_group":"default","remote_group":""},{"id":"remote-only","enabled":false,"local_group":"","remote_group":"niko-default"}]}`
	cfg, err := model.ParseNikoGroupSyncConfig(raw)
	require.NoError(t, err)
	require.Len(t, cfg.Mappings, 2)
	require.Equal(t, "default", cfg.Mappings[0].LocalGroup)
	require.Empty(t, cfg.Mappings[0].RemoteGroup)
	require.Empty(t, cfg.Mappings[1].LocalGroup)
	require.Equal(t, "niko-default", cfg.Mappings[1].RemoteGroup)
}

func TestDefaultNikoConfigHasNoHardCodedMappings(t *testing.T) {
	cfg := model.DefaultNikoGroupSyncConfig()
	require.Empty(t, cfg.Mappings)
}

func TestEnabledNikoGroupMappingsDropsDisabledRows(t *testing.T) {
	mappings := enabledNikoGroupMappings([]model.NikoGroupMapping{
		{ID: "enabled", Enabled: true, LocalGroup: "default"},
		{ID: "disabled", Enabled: false, LocalGroup: "vip"},
	})
	require.Len(t, mappings, 1)
	require.Equal(t, "enabled", mappings[0].ID)
}

func TestSyncNikoMappingsToUsableGroupsFollowsGroupPricingSelection(t *testing.T) {
	mappings := syncNikoMappingsToUsableGroups([]model.NikoGroupMapping{
		{ID: "selected", Enabled: false, LocalGroup: "default"},
		{ID: "unselected", Enabled: true, LocalGroup: "vip"},
		{ID: "remote-only", Enabled: true, LocalGroup: "", RemoteGroup: "niko-vip"},
	}, map[string]string{"default": "默认"})
	require.Len(t, mappings, 2)
	require.Equal(t, "selected", mappings[0].ID)
	require.Equal(t, "remote-only", mappings[1].ID)
}

func TestBuildNikoSyncResultUsesConfiguredMappings(t *testing.T) {
	original := ratio_setting.GetGroupRatioCopy()
	t.Cleanup(func() {
		data, _ := common.Marshal(original)
		_ = ratio_setting.UpdateGroupRatioByJSONString(string(data))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))

	cfg := model.DefaultNikoGroupSyncConfig()
	cfg.Mappings = []model.NikoGroupMapping{
		{ID: "codex-mixed", Enabled: true, LocalGroup: "codex-混池", RemoteGroup: "codex-混池", RatioMode: "remote"},
		{ID: "codex-plus", Enabled: true, LocalGroup: "codex-Plus", RemoteGroup: "codex-Plus", RatioMode: "remote"},
		{ID: "codex-pro", Enabled: true, LocalGroup: "codex-Pro", RemoteGroup: "codex-Pro", RatioMode: "remote"},
	}
	result, updated := buildNikoSyncResult(cfg, NikoGroupPreview{
		GroupRatio: map[string]float64{
			"codex-混池":   0.06,
			"codex-Plus": 0.11,
			"codex-Pro":  0.15,
			"Gemini":     0.1,
		},
		UsableGroup: map[string]string{
			"codex-混池":   "team+Plus+Pro大乱炖",
			"codex-Plus": "plus池子",
			"codex-Pro":  "纯codex-Pro号池",
		},
	}, true)

	require.Equal(t, 0.06, updated["codex-混池"])
	require.Equal(t, 0.11, updated["codex-Plus"])
	require.Equal(t, 0.15, updated["codex-Pro"])
	require.Equal(t, 1.0, updated["default"])
	require.Len(t, result.Items, 3)
}

func TestBuildNikoSyncResultDoesNotModifyUsableGroups(t *testing.T) {
	original := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(original))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组"}`))

	cfg := model.DefaultNikoGroupSyncConfig()
	cfg.Mappings = []model.NikoGroupMapping{
		{ID: "codex-mixed", Enabled: true, LocalGroup: "codex-混池", RemoteGroup: "codex-混池", RatioMode: "remote"},
		{ID: "codex-plus", Enabled: true, LocalGroup: "codex-Plus", RemoteGroup: "codex-Plus", RatioMode: "remote"},
		{ID: "codex-pro", Enabled: true, LocalGroup: "codex-Pro", RemoteGroup: "codex-Pro", RatioMode: "remote"},
	}
	result, _ := buildNikoSyncResult(cfg, NikoGroupPreview{
		GroupRatio: map[string]float64{
			"codex-混池":   0.06,
			"codex-Plus": 0.11,
			"codex-Pro":  0.15,
		},
		UsableGroup: map[string]string{
			"codex-混池":   "team+Plus+Pro大乱炖",
			"codex-Plus": "plus池子",
			"codex-Pro":  "纯codex-Pro号池",
		},
	}, false)

	require.False(t, result.Applied)
	require.Equal(t, map[string]string{"default": "默认分组"}, setting.GetUserUsableGroupsCopy())
}

func TestNikoGroupSyncScheduledPayloadNeverAppliesRemoteRatios(t *testing.T) {
	payload, ok := (nikoGroupSyncHandler{}).NewPayload().(map[string]bool)
	require.True(t, ok)
	require.NotContains(t, payload, "apply")
}

func TestBuildNikoSyncResultPreservesUnmappedAndAppliesAdjustment(t *testing.T) {
	original := ratio_setting.GetGroupRatioCopy()
	t.Cleanup(func() {
		data, _ := common.Marshal(original)
		_ = ratio_setting.UpdateGroupRatioByJSONString(string(data))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"keep":2}`))

	manual := 0.4
	cfg := model.DefaultNikoGroupSyncConfig()
	cfg.RatioAdjustment = 0.05
	cfg.Mappings = []model.NikoGroupMapping{
		{ID: "a", Enabled: true, LocalGroup: "default", RemoteGroup: "codex-Pro", RatioMode: "remote"},
		{ID: "b", Enabled: true, LocalGroup: "manual", RemoteGroup: "ignored", RatioMode: "manual", ManualRatio: &manual},
		{ID: "c", Enabled: true, LocalGroup: "missing", RemoteGroup: "absent", RatioMode: "remote"},
	}
	result, updated := buildNikoSyncResult(cfg, NikoGroupPreview{GroupRatio: map[string]float64{"codex-Pro": 0.15}}, true)
	require.Equal(t, 0.2, updated["default"])
	require.Equal(t, 0.4, updated["manual"])
	require.Equal(t, 2.0, updated["keep"])
	require.NotContains(t, updated, "missing")
	require.Equal(t, "failed", result.Items[2].Status)
}

func TestRunNikoGroupSyncSerializesConcurrentFetches(t *testing.T) {
	var active int32
	var maxActive int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&active, 1)
		for {
			old := atomic.LoadInt32(&maxActive)
			if n <= old || atomic.CompareAndSwapInt32(&maxActive, old, n) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		_, _ = w.Write([]byte(`{"success":true,"group_ratio":{"default":0.6}}`))
		atomic.AddInt32(&active, -1)
	}))
	defer server.Close()

	original := model.DefaultNikoGroupSyncConfigJSONString()
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap[model.NikoGroupSyncOptionKey] = `{"enabled":false,"source_url":"` + server.URL + `","timeout_seconds":10,"ratio_adjustment":0,"mappings":[]}`
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap[model.NikoGroupSyncOptionKey] = original
		common.OptionMapRWMutex.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = runNikoGroupSync(context.Background())
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&maxActive); got != 1 {
		t.Fatalf("max concurrent fetches = %d, want 1", got)
	}
}

func TestFetchNikoGroupRatiosReportsUnreachableConfiguredURL(t *testing.T) {
	cfg := model.DefaultNikoGroupSyncConfig()
	cfg.SourceURL = "http://127.0.0.1:3999/api/pricing"
	cfg.TimeoutSeconds = 1

	_, err := fetchNikoGroupRatios(context.Background(), cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "127.0.0.1:3999")
}

func TestApplyNikoBaseRatiosAddsAdjustmentOnce(t *testing.T) {
	cfg := model.DefaultNikoGroupSyncConfig()
	cfg.RatioAdjustment = 0.01
	cfg.Mappings = []model.NikoGroupMapping{
		{ID: "codex-mixed", Enabled: true, LocalGroup: "codex-混池", RemoteGroup: "codex-混池", RatioMode: "remote"},
		{ID: "codex-plus", Enabled: true, LocalGroup: "codex-Plus", RemoteGroup: "codex-Plus", RatioMode: "remote"},
		{ID: "codex-pro", Enabled: true, LocalGroup: "codex-Pro", RemoteGroup: "codex-Pro", RatioMode: "remote"},
		{ID: "local-only", Enabled: true, LocalGroup: "vip", RemoteGroup: "", RatioMode: "remote"},
		{ID: "remote-only", Enabled: false, LocalGroup: "", RemoteGroup: "niko-vip", RatioMode: "remote"},
	}
	cfg.BaseGroupRatio = map[string]float64{
		"codex-混池":   0.04,
		"codex-Plus": 0.11,
		"codex-Pro":  0.15,
		"vip":        0.2,
	}
	current := map[string]float64{"default": 1, "codex-混池": 0.04}

	effective, err := applyNikoBaseRatios(cfg, cfg.BaseGroupRatio, current)
	require.NoError(t, err)
	require.Equal(t, 1.0, effective["default"])
	require.Equal(t, 0.05, effective["codex-混池"])
	require.Equal(t, 0.12, effective["codex-Plus"])
	require.Equal(t, 0.16, effective["codex-Pro"])
	require.InDelta(t, 0.21, effective["vip"], 1e-12)

	repeated, err := applyNikoBaseRatios(cfg, cfg.BaseGroupRatio, effective)
	require.NoError(t, err)
	require.Equal(t, effective["codex-混池"], repeated["codex-混池"])
	require.Equal(t, effective["codex-Plus"], repeated["codex-Plus"])
	require.Equal(t, effective["codex-Pro"], repeated["codex-Pro"])
}
