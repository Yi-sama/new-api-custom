package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFastModeOptionPersistsReloadsAndRejectsInvalidChanges(t *testing.T) {
	db := useFrontendOptionMigrationDB(t)
	previousOptions := common.OptionMap
	previousGroups := setting.FastModeAllowedGroups2JSONString()
	common.OptionMap = map[string]string{"unrelated": "unchanged"}
	t.Cleanup(func() {
		common.OptionMap = previousOptions
		require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(previousGroups))
	})
	const key = setting.FastModeAllowedGroupsOptionKey
	require.NoError(t, UpdateOption(key, `["gpt-paid"]`))
	assert.Equal(t, `["gpt-paid"]`, requireOptionValue(t, db, key))
	assert.True(t, setting.IsFastModeAllowedForGroup("gpt-paid"))
	require.NoError(t, setting.UpdateFastModeAllowedGroupsByJSONString(`[]`))
	loadOptionsFromDatabase()
	assert.True(t, setting.IsFastModeAllowedForGroup("gpt-paid"))
	assert.Equal(t, "unchanged", common.OptionMap["unrelated"])
	for _, invalid := range []string{`null`, `{}`, `[1]`, `["auto"]`} {
		require.Error(t, UpdateOption(key, invalid))
		assert.Equal(t, `["gpt-paid"]`, requireOptionValue(t, db, key))
		assert.True(t, setting.IsFastModeAllowedForGroup("gpt-paid"))
	}
	require.NoError(t, db.Migrator().DropTable(&Option{}))
	require.Error(t, UpdateOption(key, `[]`))
	assert.True(t, setting.IsFastModeAllowedForGroup("gpt-paid"), "failed persistence must not change the policy")
	assert.Equal(t, `["gpt-paid"]`, common.OptionMap[key])
}
