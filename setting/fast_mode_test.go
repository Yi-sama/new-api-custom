package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFastModeAllowedGroups(t *testing.T) {
	original := FastModeAllowedGroups2JSONString()
	t.Cleanup(func() { require.NoError(t, UpdateFastModeAllowedGroupsByJSONString(original)) })
	require.NoError(t, UpdateFastModeAllowedGroupsByJSONString(`[]`))
	assert.False(t, IsFastModeAllowedForGroup("default"), "empty selection disables Fast for every group")
	require.NoError(t, UpdateFastModeAllowedGroupsByJSONString(`["gpt-paid","gpt-pro"]`))
	assert.True(t, IsFastModeAllowedForGroup("gpt-paid"))
	assert.True(t, IsFastModeAllowedForGroup("gpt-pro"))
	for _, group := range []string{"", "auto", "default", "gpt", "gpt-paid-other", "GPT-paid"} {
		assert.False(t, IsFastModeAllowedForGroup(group), group)
	}
	for _, invalid := range []string{`null`, `{}`, `"gpt-paid"`, `[1]`, `[""]`, `[" "]`, `["auto"]`} {
		assert.Error(t, UpdateFastModeAllowedGroupsByJSONString(invalid), invalid)
		assert.JSONEq(t, `["gpt-paid","gpt-pro"]`, FastModeAllowedGroups2JSONString(), "invalid updates preserve the live policy")
	}
	require.NoError(t, UpdateFastModeAllowedGroupsByJSONString(`[]`))
	assert.False(t, IsFastModeAllowedForGroup("gpt-paid"), "revocation takes effect without restarting")
}
