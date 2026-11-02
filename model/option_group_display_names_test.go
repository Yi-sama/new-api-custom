package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupDisplayNamesOptionPersistsReloadsAndRejectsInvalidChanges(t *testing.T) {
	db := useFrontendOptionMigrationDB(t)
	previousOptions := common.OptionMap
	previousNames := setting.GroupDisplayNames2JSONString()
	common.OptionMap = map[string]string{"unrelated": "unchanged"}
	t.Cleanup(func() {
		common.OptionMap = previousOptions
		require.NoError(t, setting.UpdateGroupDisplayNamesByJSONString(previousNames))
	})
	const key = setting.GroupDisplayNamesOptionKey
	const value = `[{"group":"default","name":"GPT"},{"group":"auto","name":"All models"}]`
	require.NoError(t, UpdateOption(key, `{"default":" GPT ","auto":"All models"}`))
	assert.JSONEq(t, value, requireOptionValue(t, db, key))
	require.NoError(t, setting.UpdateGroupDisplayNamesByJSONString(`{}`))
	loadOptionsFromDatabase()
	assert.Equal(t, []setting.GroupDisplayName{{Group: "default", Name: "GPT"}, {Group: "auto", Name: "All models"}}, setting.GetGroupDisplayNamesCopy())
	assert.Equal(t, "unchanged", common.OptionMap["unrelated"])

	for _, invalid := range []string{`null`, `[{}]`, `{"default":1}`, `{"default":null}`, `{" ":"GPT"}`, `{"default":`, `[{"group":"default","name":" "}]`, `[{"group":"default","name":"GPT"},{"group":"default","name":"Claude"}]`, `[{"group":"default","name":"GPT"},{"group":"auto","name":" GPT "}]`} {
		require.Error(t, UpdateOption(key, invalid))
		assert.JSONEq(t, value, requireOptionValue(t, db, key))
		assert.Equal(t, "GPT", setting.GetGroupDisplayNamesCopy()[0].Name)
	}

	require.NoError(t, UpdateOption(key, `{}`))
	assert.Empty(t, setting.GetGroupDisplayNamesCopy())
	require.NoError(t, UpdateOption(key, value))
	require.NoError(t, db.Migrator().DropTable(&Option{}))
	require.Error(t, UpdateOption(key, `{}`))
	assert.Equal(t, "GPT", setting.GetGroupDisplayNamesCopy()[0].Name)
	assert.JSONEq(t, value, common.OptionMap[key])
}
