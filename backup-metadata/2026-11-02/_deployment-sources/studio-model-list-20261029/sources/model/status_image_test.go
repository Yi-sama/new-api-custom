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
	} {
		cfg := DefaultStatusImageConfig()
		change(&cfg)
		raw, err := common.Marshal(cfg)
		require.NoError(t, err)
		require.Error(t, validateOptionValue(StatusImageConfigOptionKey, string(raw)))
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
	raw, err := common.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, UpdateOption(StatusImageConfigOptionKey, string(raw)))
	require.Equal(t, string(raw), requireOptionValue(t, db, StatusImageConfigOptionKey))
	delete(common.OptionMap, StatusImageConfigOptionKey)
	loadOptionsFromDatabase()
	require.Equal(t, string(raw), common.OptionMap[StatusImageConfigOptionKey])
	require.Equal(t, "unchanged", common.OptionMap[ChannelStatusConfigOptionKey])
	require.Error(t, UpdateOption(StatusImageConfigOptionKey, `{}`))
	require.Equal(t, string(raw), requireOptionValue(t, db, StatusImageConfigOptionKey))

	require.NoError(t, db.Migrator().DropTable(&Option{}))
	require.Error(t, UpdateOption(StatusImageConfigOptionKey, DefaultStatusImageConfigJSONString()))
	require.Equal(t, string(raw), common.OptionMap[StatusImageConfigOptionKey])
}
