package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseChannelStatusConfigKeepsModelOrderAndNormalizesWhitespace(t *testing.T) {
	raw, err := common.Marshal(ChannelStatusConfig{
		Enabled: true,
		Groups: []ChannelStatusGroupConfig{{
			Group:   " OpenAI ",
			Enabled: true,
			Models:  []string{" gpt-5.6-terra ", "gpt-5.6-sol", "gpt-5.6-terra", ""},
		}},
	})
	require.NoError(t, err)

	config, err := ParseChannelStatusConfig(string(raw))
	require.NoError(t, err)
	assert.Equal(t, "OpenAI", config.Groups[0].Group)
	assert.Equal(t, []string{"gpt-5.6-terra", "gpt-5.6-sol"}, config.Groups[0].Models)
}

func TestParseChannelStatusConfigRejectsEnabledGroupWithoutModel(t *testing.T) {
	raw, err := common.Marshal(ChannelStatusConfig{
		Enabled: true,
		Groups:  []ChannelStatusGroupConfig{{Group: "OpenAI", Enabled: true}},
	})
	require.NoError(t, err)

	_, err = ParseChannelStatusConfig(string(raw))
	require.Error(t, err)
}

func TestSortedChannelStatusModelsReturnsModelsForGroup(t *testing.T) {
	channels := []*Channel{
		{Group: "vip,default", Models: "gpt-5.6-sol,gpt-5.6-terra"},
		{Group: "vip", Models: "gpt-5.6-terra,claude-3-7"},
		{Group: "default", Models: "default-only"},
	}

	require.Equal(t, []string{"claude-3-7", "gpt-5.6-sol", "gpt-5.6-terra"}, SortedChannelStatusModels(channels, "vip"))
	require.Equal(t, []string{"default-only", "gpt-5.6-sol", "gpt-5.6-terra"}, SortedChannelStatusModels(channels, "default"))
}
