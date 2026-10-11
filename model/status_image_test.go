package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestStatusImageConfigValidation(t *testing.T) {
	require.NoError(t, validateOptionValue(StatusImageConfigOptionKey, DefaultStatusImageConfigJSONString()))
	for _, raw := range []string{"null", "[]", "{}", `{"site_name":42}`, strings.Repeat("x", 8193)} {
		require.Error(t, validateOptionValue(StatusImageConfigOptionKey, raw))
	}
	for _, change := range []func(*StatusImageConfig){
		func(c *StatusImageConfig) { value := " "; c.SiteName = &value },
		func(c *StatusImageConfig) { value := strings.Repeat("a", 61); c.ChannelTitle = &value },
		func(c *StatusImageConfig) { value := ""; c.SubTitle = &value },
		func(c *StatusImageConfig) { value := 3; c.ChannelColumns = &value },
		func(c *StatusImageConfig) { value := strings.Repeat("a", 161); c.Footer = &value },
		func(c *StatusImageConfig) { value := strings.Repeat("a", 501); c.GroupFilter = &value },
		func(c *StatusImageConfig) { value := strings.Repeat("a", 501); c.ChannelGroupFilter = &value },
		func(c *StatusImageConfig) { value := strings.Repeat("a", 501); c.SubGroupFilter = &value },
	} {
		cfg := DefaultStatusImageConfig()
		change(&cfg)
		raw, err := common.Marshal(cfg)
		require.NoError(t, err)
		require.Error(t, validateOptionValue(StatusImageConfigOptionKey, string(raw)))
	}
}

func TestStatusImageIndependentFiltersAllowLegacyAndExplicitEmptyValues(t *testing.T) {
	cfg := DefaultStatusImageConfig()
	legacy, channel, sub := "Legacy pool", "Channel pool", ""
	cfg.GroupFilter = &legacy
	cfg.ChannelGroupFilter = &channel
	cfg.SubGroupFilter = &sub
	for _, legacyOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "independent", true: "legacy"}[legacyOnly], func(t *testing.T) {
			if legacyOnly {
				cfg.ChannelGroupFilter, cfg.SubGroupFilter = nil, nil
			}
			raw, err := common.Marshal(cfg)
			require.NoError(t, err)
			require.NoError(t, ValidateStatusImageConfig(string(raw)))
			var restored StatusImageConfig
			require.NoError(t, common.Unmarshal(raw, &restored))
			require.Equal(t, &legacy, restored.GroupFilter)
			if legacyOnly {
				require.Nil(t, restored.ChannelGroupFilter)
				require.Nil(t, restored.SubGroupFilter)
			} else {
				require.Equal(t, &channel, restored.ChannelGroupFilter)
				require.Equal(t, &sub, restored.SubGroupFilter)
			}
		})
	}
}

func TestStatusImageConfigRejectsMissingOrNullFields(t *testing.T) {
	for _, field := range []string{"site_name", "channel_title", "sub_title", "footer", "group_filter", "channel_columns", "show_success_rate", "show_latency", "show_history", "show_updated_at", "show_sub_details"} {
		for _, missing := range []bool{true, false} {
			var fields map[string]any
			require.NoError(t, common.UnmarshalJsonStr(DefaultStatusImageConfigJSONString(), &fields))
			if missing {
				delete(fields, field)
			} else {
				fields[field] = nil
			}
			raw, err := common.Marshal(fields)
			require.NoError(t, err)
			require.Error(t, validateOptionValue(StatusImageConfigOptionKey, string(raw)), "field %s, missing=%v", field, missing)
		}
	}
}

func TestStatusImageOptionPersistsReloadsAndRejectsInvalidChanges(t *testing.T) {
	db := useFrontendOptionMigrationDB(t)
	previous := common.OptionMap
	common.OptionMap = map[string]string{ChannelStatusConfigOptionKey: "unchanged"}
	t.Cleanup(func() { common.OptionMap = previous })
	cfg := DefaultStatusImageConfig()
	siteName := "测试站"
	cfg.SiteName = &siteName
	latency := false
	cfg.ShowLatency = &latency
	channelGroups, subGroups := "A,B", ""
	cfg.ChannelGroupFilter, cfg.SubGroupFilter = &channelGroups, &subGroups
	selectedModels := []StatusImageModelSelection{{Group: "A", Model: "gpt-5"}, {Group: "B", Model: "claude-sonnet"}}
	cfg.ChannelModelFilter = &selectedModels
	raw, err := common.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, UpdateOption(StatusImageConfigOptionKey, string(raw)))
	require.Equal(t, string(raw), requireOptionValue(t, db, StatusImageConfigOptionKey))
	delete(common.OptionMap, StatusImageConfigOptionKey)
	loadOptionsFromDatabase()
	require.Equal(t, string(raw), common.OptionMap[StatusImageConfigOptionKey])
	var restored StatusImageConfig
	require.NoError(t, common.UnmarshalJsonStr(common.OptionMap[StatusImageConfigOptionKey], &restored))
	require.Equal(t, &channelGroups, restored.ChannelGroupFilter)
	require.Equal(t, &subGroups, restored.SubGroupFilter)
	require.Equal(t, &selectedModels, restored.ChannelModelFilter)
	require.Equal(t, "unchanged", common.OptionMap[ChannelStatusConfigOptionKey])
	require.Error(t, UpdateOption(StatusImageConfigOptionKey, `{}`))
	require.Equal(t, string(raw), requireOptionValue(t, db, StatusImageConfigOptionKey))

	require.NoError(t, db.Migrator().DropTable(&Option{}))
	require.Error(t, UpdateOption(StatusImageConfigOptionKey, DefaultStatusImageConfigJSONString()))
	require.Equal(t, string(raw), common.OptionMap[StatusImageConfigOptionKey])
}

func TestStatusImageModelSelectionPreservesAllNoneAndExactPairs(t *testing.T) {
	for _, selection := range []any{
		nil,
		[]StatusImageModelSelection{},
		[]StatusImageModelSelection{{Group: "A", Model: "gpt-5"}, {Group: "B", Model: "claude-sonnet"}},
	} {
		var fields map[string]any
		require.NoError(t, common.UnmarshalJsonStr(DefaultStatusImageConfigJSONString(), &fields))
		fields["channel_model_filter"] = selection
		raw, err := common.Marshal(fields)
		require.NoError(t, err)
		require.NoError(t, ValidateStatusImageConfig(string(raw)))
		var restored StatusImageConfig
		require.NoError(t, common.Unmarshal(raw, &restored))
		if selection == nil {
			require.Nil(t, restored.ChannelModelFilter)
		} else {
			require.NotNil(t, restored.ChannelModelFilter)
			require.Equal(t, selection, *restored.ChannelModelFilter)
			roundTrip, err := common.Marshal(restored)
			require.NoError(t, err)
			require.Contains(t, string(roundTrip), `"channel_model_filter":`)
		}
	}
}

func TestStatusImageModelSelectionRejectsInvalidPairs(t *testing.T) {
	for _, selection := range []any{
		"gpt-5",
		[]any{nil},
		[]any{"gpt-5"},
		[]any{map[string]any{"group": "A"}},
		[]StatusImageModelSelection{{Group: " ", Model: "gpt-5"}},
		[]StatusImageModelSelection{{Group: "A", Model: strings.Repeat("x", 501)}},
		make([]StatusImageModelSelection, 501),
	} {
		var fields map[string]any
		require.NoError(t, common.UnmarshalJsonStr(DefaultStatusImageConfigJSONString(), &fields))
		fields["channel_model_filter"] = selection
		raw, err := common.Marshal(fields)
		require.NoError(t, err)
		require.Error(t, ValidateStatusImageConfig(string(raw)))
	}
}

func TestStatusImageModelSelectionAllowsListsLargerThanLegacyConfigLimit(t *testing.T) {
	cfg := DefaultStatusImageConfig()
	selection := make([]StatusImageModelSelection, 100)
	for index := range selection {
		selection[index] = StatusImageModelSelection{Group: "Pool", Model: strings.Repeat("model", 30)}
	}
	cfg.ChannelModelFilter = &selection
	raw, err := common.Marshal(cfg)
	require.NoError(t, err)
	require.Greater(t, len(raw), 8192)
	require.NoError(t, ValidateStatusImageConfig(string(raw)))
	require.Error(t, ValidateStatusImageConfig(string(raw)+strings.Repeat(" ", 2*1024*1024)))
}
